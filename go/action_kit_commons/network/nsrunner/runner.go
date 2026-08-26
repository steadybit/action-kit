// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

// Package nsrunner runs a one-shot command inside a target's network namespace.
//
// It is a general-purpose, exported counterpart to the unexported runner in the
// netfault package: extensions that need to execute a tool (iptables, a custom
// binary, ...) inside a discovered container/host network namespace can use it
// directly, feeding the command's stdin as a batch of directives and reading
// back its combined output. Two backends are provided: a runc sidecar joined to
// the target's network namespace, and a process runner that executes in the
// extension's own namespace.
package nsrunner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
)

var ipPath = utils.LocateExecutable("ip", "STEADYBIT_EXTENSION_IP_PATH")

// Runner executes a command inside a target's network namespace.
type Runner interface {
	// Run executes argv inside the target network namespace, writes the stdin
	// lines (joined by newlines) to the process's standard input, and returns
	// its combined stdout. Each stdin line must be a single directive; a line
	// containing a newline is rejected to prevent batch injection.
	Run(ctx context.Context, argv []string, stdin []string) (string, error)
	// NetNsPath is the target network namespace path (/proc/<pid>/ns/net or
	// /var/run/netns/<name>).
	NetNsPath() string
	// ID identifies the namespace (inode or path), for locking and logging.
	ID() string
}

// SidecarOpts configures the runc-backed runner.
type SidecarOpts struct {
	TargetProcess ociruntime.LinuxProcessInfo
	Id            string
	// Capabilities granted to the sidecar process. Defaults to
	// CAP_NET_ADMIN and CAP_NET_RAW when empty.
	Capabilities []string
}

func (o SidecarOpts) capabilities() []string {
	if len(o.Capabilities) == 0 {
		return []string{"CAP_NET_ADMIN", "CAP_NET_RAW"}
	}
	return o.Capabilities
}

// validateStdin rejects any directive containing a newline before it is written
// to a batch stdin, so an unsanitized value forwarded from user input cannot be
// split into extra privileged directives.
func validateStdin(lines []string) error {
	for _, s := range lines {
		if strings.ContainsAny(s, "\n\r") {
			return fmt.Errorf("refusing to build batch: line contains a newline (possible injection): %q", s)
		}
	}
	return nil
}

func toReader(lines []string) io.Reader {
	return strings.NewReader(strings.Join(lines, "\n") + "\n")
}

func nextContainerId(tool, id string) string {
	return fmt.Sprintf("sb-%s-%s", path.Base(tool), id)
}

// --- process runner ---

// NewProcessRunner runs commands directly in the calling process's network
// namespace (the extension's own netns).
func NewProcessRunner() Runner { return &processRunner{} }

type processRunner struct{}

func (processRunner) Run(ctx context.Context, argv []string, stdin []string) (string, error) {
	if len(argv) == 0 {
		return "", errors.New("empty argv")
	}
	if err := validateStdin(stdin); err != nil {
		return "", err
	}
	log.Debug().Strs("argv", argv).Msg("running command in process netns")

	var outb, errb bytes.Buffer
	cmd := utils.RootCommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	if len(stdin) > 0 {
		cmd.Stdin = toReader(stdin)
	}
	if err := cmd.Run(); err != nil {
		return outb.String(), fmt.Errorf("%s failed: %w, stderr: %s", argv[0], err, strings.TrimSpace(errb.String()))
	}
	return outb.String(), nil
}

func (processRunner) NetNsPath() string { return "/proc/self/ns/net" }
func (processRunner) ID() string        { return "host" }

// --- runc runner ---

// NewRuncRunner runs commands inside the target's network namespace using a runc
// sidecar (or `ip netns exec` when the target uses a named network namespace).
func NewRuncRunner(r ociruntime.OciRuntime, sidecar SidecarOpts) Runner {
	return &runcRunner{runc: r, sidecar: sidecar}
}

type runcRunner struct {
	runc    ociruntime.OciRuntime
	sidecar SidecarOpts
}

func (r *runcRunner) Run(ctx context.Context, argv []string, stdin []string) (string, error) {
	if len(argv) == 0 {
		return "", errors.New("empty argv")
	}
	if err := validateStdin(stdin); err != nil {
		return "", err
	}
	ociruntime.RefreshNamespaces(ctx, r.sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)

	if ociruntime.HasNamedNetworkNamespace(r.sidecar.TargetProcess.Namespaces...) {
		return r.runInNamedNetns(ctx, argv, stdin)
	}
	return r.runInRuncSidecar(ctx, argv, stdin)
}

func (r *runcRunner) runInNamedNetns(ctx context.Context, argv []string, stdin []string) (string, error) {
	netns := ""
	for _, n := range r.sidecar.TargetProcess.Namespaces {
		if n.Type == specs.NetworkNamespace {
			netns = ociruntime.TrimNameNetworkNamespacePrefix(n.Path)
			break
		}
	}
	log.Debug().Str("netns", netns).Strs("argv", argv).Msg("running command via ip netns exec")

	ipArgs := append([]string{"netns", "exec", netns}, argv...)
	var outb, errb bytes.Buffer
	cmd := utils.RootCommandContext(ctx, ipPath, ipArgs...)
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	if len(stdin) > 0 {
		cmd.Stdin = toReader(stdin)
	}
	if err := cmd.Run(); err != nil {
		return outb.String(), fmt.Errorf("ip netns exec failed: %w, stderr: %s", err, strings.TrimSpace(errb.String()))
	}
	return outb.String(), nil
}

func (r *runcRunner) runInRuncSidecar(ctx context.Context, argv []string, stdin []string) (string, error) {
	id := nextContainerId(argv[0], r.sidecar.Id)
	bundle, err := r.runc.Create(ctx, "/", id)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := bundle.Remove(); err != nil {
			log.Warn().Str("id", id).Err(err).Msg("failed to remove bundle")
		}
	}()

	if err = bundle.EditSpec(
		ociruntime.WithHostname(id),
		ociruntime.WithAnnotations(map[string]string{"com.steadybit.sidecar": "true"}),
		ociruntime.WithNamespaces(ociruntime.FilterNamespaces(r.sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)),
		ociruntime.WithCapabilities(r.sidecar.capabilities()...),
		ociruntime.WithCopyEnviron(),
		ociruntime.WithEnv("XTABLES_LOCKFILE=/tmp/xtables.lock"),
		ociruntime.WithMountIfNotPresent(specs.Mount{Destination: "/tmp", Type: "tmpfs", Source: "tmpfs", Options: []string{"nosuid", "nodev", "noexec"}}),
		ociruntime.WithProcessArgs(argv...),
	); err != nil {
		return "", err
	}

	var outb, errb bytes.Buffer
	ioOpts := ociruntime.IoOpts{Stdout: &outb, Stderr: &errb}
	if len(stdin) > 0 {
		ioOpts.Stdin = toReader(stdin)
	}
	runErr := r.runc.Run(ctx, bundle, ioOpts)
	defer func() {
		if err := r.runc.Delete(context.Background(), id, true); err != nil {
			level := zerolog.WarnLevel
			if errors.Is(err, ociruntime.ErrContainerNotFound) {
				level = zerolog.DebugLevel
			}
			log.WithLevel(level).Str("id", id).Err(err).Msg("failed to delete container")
		}
	}()

	if runErr != nil {
		return outb.String(), fmt.Errorf("%s failed: %w, stderr: %s", id, runErr, strings.TrimSpace(errb.String()))
	}
	return outb.String(), nil
}

func (r *runcRunner) NetNsPath() string {
	for _, ns := range r.sidecar.TargetProcess.Namespaces {
		if ns.Type == specs.NetworkNamespace {
			return ns.Path
		}
	}
	return ""
}

func (r *runcRunner) ID() string {
	for _, ns := range r.sidecar.TargetProcess.Namespaces {
		if ns.Type == specs.NetworkNamespace {
			if ns.Inode != 0 {
				return strconv.FormatUint(ns.Inode, 10)
			}
			return ns.Path
		}
	}
	return ""
}
