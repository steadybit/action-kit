// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

//go:build !windows

// Package proxyfault launches and manages the steadybit transparent-proxy
// binary inside a target's network namespace, for injecting network faults
// against a target's dependencies (latency, connection reset, HTTP status
// injection). It mirrors the dnsinject package: the long-running proxy is run in
// a runc sidecar joined to the target netns (or via `ip netns exec` for named
// namespaces); the proxy self-manages its iptables interception and tears it
// down on stop. Revert provides an out-of-band teardown for the case where the
// proxy process was killed before its own cleanup ran.
//
// The proxy is run as an external binary (located via
// STEADYBIT_EXTENSION_TRANSPARENT_PROXY_PATH), so this package has no build
// dependency on the transparent-proxy module.
package proxyfault

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/network/nsrunner"
	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
)

var proxyPath = utils.LocateExecutable("transparent-proxy", "STEADYBIT_EXTENSION_TRANSPARENT_PROXY_PATH")

// Fault is the single fault the proxy injects on matching connections.
type Fault struct {
	Latency          time.Duration
	AbortProbability float64
	HTTPStatus       int
	Hosts            []string
}

// Opts configures interception and the injected fault.
type Opts struct {
	ExecutionId  string
	ProxyPort    uint16
	MetricsPort  uint16 // 0 disables the metrics endpoint
	Mark         uint32 // 0 uses the proxy's default loop-protection mark
	MaxDuration  time.Duration
	IncludeCIDRs []net.IPNet
	ExcludeCIDRs []net.IPNet
	Ports        []uint16
	Fault        Fault
}

// Proxy is a running transparent-proxy instance.
type Proxy interface {
	Start() error
	Stop() error
	Exited() (bool, error)
}

// NewProcess builds (but does not start) a proxy for the target's netns.
func NewProcess(ctx context.Context, r ociruntime.OciRuntime, targetProcess ociruntime.LinuxProcessInfo, id string, opts Opts) (Proxy, error) {
	ociruntime.RefreshNamespaces(ctx, targetProcess.Namespaces, specs.NetworkNamespace)
	if ociruntime.HasNamedNetworkNamespace(targetProcess.Namespaces...) {
		return newNetnsProcess(targetProcess, opts)
	}
	return newRuncProcess(ctx, r, targetProcess, id, opts)
}

// Revert removes the interception out of band by running the proxy's --revert.
// The runner is supplied by the caller (runc or process), matching how the
// proxy was launched. It is idempotent.
func Revert(ctx context.Context, runner nsrunner.Runner, opts Opts) error {
	_, err := runner.Run(ctx, append([]string{proxyPath}, opts.revertArgs()...), nil)
	return err
}

// startArgs are the flags to launch the self-managed proxy.
func (o Opts) startArgs() []string {
	args := o.interceptArgs()
	if o.MaxDuration > 0 {
		args = append(args, "--max-duration", o.MaxDuration.String())
	}
	if o.MetricsPort != 0 {
		args = append(args, "--metrics-addr", fmt.Sprintf("0.0.0.0:%d", o.MetricsPort))
	}
	if o.Fault.Latency > 0 {
		args = append(args, "--fault-latency", o.Fault.Latency.String())
	}
	if o.Fault.AbortProbability > 0 {
		args = append(args, "--fault-abort-probability", strconv.FormatFloat(o.Fault.AbortProbability, 'f', -1, 64))
	}
	if o.Fault.HTTPStatus != 0 {
		args = append(args, "--fault-http-status", strconv.Itoa(o.Fault.HTTPStatus))
	}
	if len(o.Fault.Hosts) > 0 {
		args = append(args, "--fault-hosts", strings.Join(o.Fault.Hosts, ","))
	}
	return args
}

// revertArgs are the flags to reconstruct and remove the same interception.
func (o Opts) revertArgs() []string {
	return append([]string{"--revert"}, o.interceptArgs()...)
}

// interceptArgs are the flags shared by start and revert (they must produce the
// same chain names and filter).
func (o Opts) interceptArgs() []string {
	args := []string{
		"--listen", fmt.Sprintf("0.0.0.0:%d", o.ProxyPort),
		"--exec-id", o.ExecutionId,
		"--intercept-cidrs", joinCIDRs(o.IncludeCIDRs),
		"--intercept-ports", joinPorts(o.Ports),
	}
	if len(o.ExcludeCIDRs) > 0 {
		args = append(args, "--exclude-cidrs", joinCIDRs(o.ExcludeCIDRs))
	}
	if o.Mark != 0 {
		args = append(args, "--mark", strconv.FormatUint(uint64(o.Mark), 10))
	}
	return args
}

func joinCIDRs(nets []net.IPNet) string {
	parts := make([]string, len(nets))
	for i, n := range nets {
		parts[i] = n.String()
	}
	return strings.Join(parts, ",")
}

func joinPorts(ports []uint16) string {
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = strconv.Itoa(int(p))
	}
	return strings.Join(parts, ",")
}

func (o Opts) String() string {
	return "transparent-proxy " + strings.Join(o.startArgs(), " ")
}

// processBase holds the exit channel and start/monitor helper shared by both
// backends.
type processBase struct {
	exited chan error
}

func (b *processBase) Exited() (bool, error) {
	select {
	case err := <-b.exited:
		b.exited <- err // put back for subsequent reads
		return true, err
	default:
		return false, nil
	}
}

func (b *processBase) startAndMonitor(cmd *exec.Cmd, logId string) error {
	logger := log.With().Str("id", logId).Logger()
	cmd.Stdout = &logWriter{logger: logger}
	cmd.Stderr = &logWriter{logger: logger}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start transparent-proxy: %w", err)
	}
	go func() { b.exited <- cmd.Wait() }()
	return nil
}

type logWriter struct {
	logger zerolog.Logger
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.logger.Debug().Msg(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

// --- process (named netns) backend ---

type netnsProxy struct {
	processBase
	cmd  *exec.Cmd
	opts Opts
}

func newNetnsProcess(targetProcess ociruntime.LinuxProcessInfo, opts Opts) (Proxy, error) {
	netns := ""
	for _, ns := range targetProcess.Namespaces {
		if ns.Type == specs.NetworkNamespace {
			netns = ociruntime.TrimNameNetworkNamespacePrefix(ns.Path)
			break
		}
	}
	if netns == "" {
		return nil, errors.New("no network namespace found")
	}
	ipPath := utils.LocateExecutable("ip", "STEADYBIT_EXTENSION_IP_PATH")
	cmdArgs := append([]string{"netns", "exec", netns, proxyPath}, opts.startArgs()...)
	cmd := utils.RootCommandContext(context.Background(), ipPath, cmdArgs...)
	return &netnsProxy{processBase: processBase{exited: make(chan error, 1)}, cmd: cmd, opts: opts}, nil
}

func (p *netnsProxy) Start() error {
	log.Trace().Str("cmd", p.opts.String()).Msg("starting transparent-proxy via ip netns exec")
	return p.startAndMonitor(p.cmd, "transparent-proxy")
}

func (p *netnsProxy) Stop() error {
	if p.cmd.Process == nil {
		return nil
	}
	pid := p.cmd.Process.Pid
	ctx := context.Background()
	// SIGTERM lets the in-proxy Guard tear down its interception cleanly.
	if err := utils.RootCommandContext(ctx, "kill", "-s", "SIGTERM", strconv.Itoa(pid)).Run(); err != nil {
		log.Warn().Err(err).Msg("failed to SIGTERM transparent-proxy")
	}
	killTimer := time.AfterFunc(15*time.Second, func() {
		if err := p.cmd.Process.Signal(syscall.SIGKILL); err != nil {
			log.Warn().Err(err).Msg("failed to SIGKILL transparent-proxy")
		}
	})
	<-p.exited
	killTimer.Stop()
	return nil
}

// --- runc sidecar backend ---

type runcProxy struct {
	processBase
	bundle ociruntime.ContainerBundle
	runc   ociruntime.OciRuntime
	opts   Opts
}

func newRuncProcess(ctx context.Context, r ociruntime.OciRuntime, targetProcess ociruntime.LinuxProcessInfo, id string, opts Opts) (Proxy, error) {
	containerId := fmt.Sprintf("sb-transparent-proxy-%d-%s", time.Now().UnixMilli(), id)
	bundle, err := r.Create(ctx, "/", containerId)
	if err != nil {
		return nil, fmt.Errorf("failed to create bundle: %w", err)
	}

	processArgs := append([]string{proxyPath}, opts.startArgs()...)
	if err := bundle.EditSpec(
		ociruntime.WithHostname(containerId),
		ociruntime.WithAnnotations(map[string]string{"com.steadybit.sidecar": "true"}),
		ociruntime.WithNamespaces(ociruntime.FilterNamespaces(targetProcess.Namespaces, specs.NetworkNamespace)),
		ociruntime.WithCapabilities("CAP_NET_ADMIN", "CAP_NET_RAW"),
		ociruntime.WithCopyEnviron(),
		// iptables in the sidecar needs a writable lock location.
		ociruntime.WithEnv("XTABLES_LOCKFILE=/tmp/xtables.lock"),
		ociruntime.WithMountIfNotPresent(specs.Mount{Destination: "/tmp", Type: "tmpfs", Source: "tmpfs", Options: []string{"nosuid", "nodev"}}),
		ociruntime.WithProcessArgs(processArgs...),
	); err != nil {
		_ = bundle.Remove()
		return nil, fmt.Errorf("failed to configure bundle: %w", err)
	}

	return &runcProxy{processBase: processBase{exited: make(chan error, 1)}, bundle: bundle, runc: r, opts: opts}, nil
}

func (d *runcProxy) Start() error {
	log.Trace().Str("containerId", d.bundle.ContainerId()).Str("cmd", d.opts.String()).Msg("starting transparent-proxy via runc sidecar")
	cmd, err := d.runc.RunCommand(context.Background(), d.bundle)
	if err != nil {
		return fmt.Errorf("failed to create run command: %w", err)
	}
	return d.startAndMonitor(cmd, d.bundle.ContainerId())
}

func (d *runcProxy) Stop() error {
	ctx := context.Background()
	// SIGTERM lets the in-proxy Guard tear down interception cleanly.
	if err := d.runc.Kill(ctx, d.bundle.ContainerId(), syscall.SIGTERM); err != nil {
		log.Warn().Str("id", d.bundle.ContainerId()).Err(err).Msg("failed to SIGTERM transparent-proxy")
	}
	killTimer := time.AfterFunc(15*time.Second, func() {
		if err := d.runc.Kill(ctx, d.bundle.ContainerId(), syscall.SIGKILL); err != nil {
			log.Warn().Str("id", d.bundle.ContainerId()).Err(err).Msg("failed to SIGKILL transparent-proxy")
		}
	})
	<-d.exited
	killTimer.Stop()

	if err := d.runc.Delete(ctx, d.bundle.ContainerId(), false); err != nil {
		level := zerolog.WarnLevel
		if errors.Is(err, ociruntime.ErrContainerNotFound) {
			level = zerolog.DebugLevel
		}
		log.WithLevel(level).Str("id", d.bundle.ContainerId()).Err(err).Msg("failed to delete container")
	}
	if err := d.bundle.Remove(); err != nil {
		log.Warn().Str("id", d.bundle.ContainerId()).Err(err).Msg("failed to remove bundle")
	}
	return nil
}
