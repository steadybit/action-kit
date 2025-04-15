// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH
//go:build !windows

package runc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/trace"
	"strconv"
	"syscall"
	"time"
)

type Runc interface {
	State(ctx context.Context, id string) (*ContainerState, error)
	Create(ctx context.Context, image, id string) (ContainerBundle, error)
	Run(ctx context.Context, container ContainerBundle, ioOpts IoOpts) error
	RunCommand(ctx context.Context, container ContainerBundle) (*exec.Cmd, error)
	Delete(ctx context.Context, id string, force bool) error
	Kill(background context.Context, id string, signal syscall.Signal) error
}

type ContainerBundle interface {
	EditSpec(editors ...SpecEditor) error
	MountFromProcess(ctx context.Context, fromPid int, fromPath, mountpoint string) error
	CopyFileFromProcess(ctx context.Context, pid int, fromPath, toPath string) error
	Path() string
	ContainerId() string
	Remove() error
}

type Config struct {
	Root          string `json:"root" split_words:"true" required:"false"`
	Debug         bool   `json:"debug" split_words:"true" required:"false"`
	SystemdCgroup bool   `json:"systemdCgroup" split_words:"true" required:"false"`
	Rootless      string `json:"rootless" split_words:"true" required:"false"`
	NsmountPath   string `json:"nsmountPath" split_words:"true" default:"nsmount"`
}

type defaultRunc struct {
	cfg Config
}

type ContainerState struct {
	ID          string            `json:"id"`
	Pid         int               `json:"pid"`
	Status      string            `json:"status"`
	Bundle      string            `json:"bundle"`
	Rootfs      string            `json:"rootfs"`
	Created     time.Time         `json:"created"`
	Annotations map[string]string `json:"annotations"`
}

func ConfigFromEnvironment() Config {
	cfg := Config{}
	err := envconfig.Process("steadybit_extension_runc", &cfg)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to parse health HTTP server configuration from environment.")
	}
	return cfg
}

var (
	ErrContainerNotFound = errors.New("container not found")
)

func NewRunc(cfg Config) Runc {
	return &defaultRunc{cfg: cfg}
}

func (r *defaultRunc) State(ctx context.Context, id string) (*ContainerState, error) {
	defer trace.StartRegion(ctx, "runc.State").End()
	cmd := r.command(ctx, "state", id)
	var outputBuffer, errorBuffer bytes.Buffer
	cmd.Stdout = &outputBuffer
	cmd.Stderr = &errorBuffer
	err := cmd.Run()
	output := outputBuffer.Bytes()
	stderr := errorBuffer.Bytes()
	if err != nil {
		return nil, r.toError(err, stderr)
	}

	log.Trace().Str("output", string(output)).Str("stderr", string(stderr)).Msg("get container state")

	var state ContainerState
	if err := unmarshalGuarded(output, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal container state: %w", err)
	}
	return &state, nil
}

func (r *defaultRunc) Create(ctx context.Context, image string, id string) (ContainerBundle, error) {
	defer trace.StartRegion(ctx, "runc.Create").End()

	bundle := containerBundle{
		id:   id,
		path: filepath.Join("/tmp/steadybit/containers", id),
		runc: r,
	}

	if _, err := os.Stat(bundle.path); err == nil {
		return nil, errors.New("container bundle already exists")
	}

	success := false
	defer func() {
		if !success {
			err := bundle.Remove()
			if err != nil {
				log.Warn().Err(err).Msg("failed to run bundle finalizers")
			}
		}
	}()

	log.Trace().Str("bundle", bundle.path).Msg("creating container bundle")
	if err := os.MkdirAll(bundle.path, 0775); err != nil {
		return nil, fmt.Errorf("failed to create directory '%s': %w", bundle.path, err)
	}
	bundle.addFinalizer(func() error {
		log.Trace().Str("bundle", bundle.path).Msg("removing container bundle")
		return os.RemoveAll(bundle.path)
	})

	var imagePath string
	if imageStat, err := os.Stat(image); err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	} else if imageStat.IsDir() {
		if abs, err := filepath.Abs(image); err == nil {
			imagePath = abs
		} else {
			log.Debug().Err(err).Str("image", image).Msg("failed to get absolute path for image")
			imagePath = image
		}
	} else {
		extractPath := filepath.Join("/tmp/", image, strconv.FormatInt(imageStat.ModTime().Unix(), 10))
		if err := extractImage(image, extractPath); err != nil {
			return nil, fmt.Errorf("failed to extract image: %w", err)
		}
		imagePath = extractPath
	}

	if err := bundle.mountRootfsOverlay(ctx, imagePath); err != nil {
		return nil, fmt.Errorf("failed to mount image: %w", err)
	}

	if err := r.createSpec(ctx, bundle.path); err != nil {
		return nil, fmt.Errorf("failed to create container spec: %w", err)
	}

	log.Trace().Str("bundle", bundle.path).Str("id", id).Msg("prepared container bundle")
	success = true
	return &bundle, nil
}

func (r *defaultRunc) Delete(ctx context.Context, id string, force bool) error {
	defer trace.StartRegion(ctx, "runc.Delete").End()
	log.Trace().Str("id", id).Msg("deleting container")
	if output, err := r.command(ctx, "delete", fmt.Sprintf("--force=%t", force), id).CombinedOutput(); err != nil {
		return r.toError(err, output)
	}
	return nil
}

func (r *defaultRunc) toError(err error, output []byte) error {
	if bytes.Contains(output, []byte("msg=\"container does not exist\"")) {
		return ErrContainerNotFound
	}
	return fmt.Errorf("%s: %s", err, output)
}

type IoOpts struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func (o IoOpts) WithStdin(reader io.Reader) IoOpts {
	return IoOpts{
		Stdin:  reader,
		Stdout: o.Stdout,
		Stderr: o.Stderr,
	}
}

func (r *defaultRunc) Run(ctx context.Context, container ContainerBundle, ioOpts IoOpts) error {
	defer trace.StartRegion(ctx, "runc.Run").End()
	cmd, err := r.RunCommand(ctx, container)
	if err != nil {
		return err
	}

	cmd.Stdin = ioOpts.Stdin
	cmd.Stdout = ioOpts.Stdout
	cmd.Stderr = ioOpts.Stderr

	log.Trace().Str("id", container.ContainerId()).Msg("running container")
	err = cmd.Run()

	log.Trace().Str("id", container.ContainerId()).Int("exitCode", cmd.ProcessState.ExitCode()).Msg("container exited")
	return err
}

func (r *defaultRunc) RunCommand(ctx context.Context, container ContainerBundle) (*exec.Cmd, error) {
	defer trace.StartRegion(ctx, "runc.RunCommand").End()
	bundle, ok := container.(*containerBundle)
	if !ok {
		return nil, fmt.Errorf("invalid bundle type: %T", container)
	}

	return r.command(ctx, "run", "--bundle", bundle.path, bundle.id), nil
}

func (r *defaultRunc) Kill(ctx context.Context, id string, signal syscall.Signal) error {
	defer trace.StartRegion(ctx, "runc.Kill").End()
	log.Trace().Str("id", id).Int("signal", int(signal)).Msg("sending signal to container")
	if output, err := r.command(ctx, "kill", id, strconv.Itoa(int(signal))).CombinedOutput(); err != nil {
		return r.toError(err, output)
	}
	return nil
}

type SpecEditor func(spec *specs.Spec)

func (b *containerBundle) EditSpec(editors ...SpecEditor) error {
	spec, err := b.readSpec()
	if err != nil {
		return err
	}

	withDefaults(spec)

	for _, fn := range editors {
		fn(spec)
	}

	err = b.writeSpec(spec)
	log.Trace().Str("bundle", b.path).Interface("createSpec", spec).Msg("written runc createSpec")
	return err
}

func (b *containerBundle) readSpec() (*specs.Spec, error) {
	content, err := os.ReadFile(filepath.Join(b.path, "config.json"))
	if err != nil {
		return nil, err
	}

	var spec specs.Spec

	if err := json.Unmarshal(content, &spec); err != nil {
		return nil, err
	}

	return &spec, nil
}
func (b *containerBundle) writeSpec(spec *specs.Spec) error {
	content, err := json.MarshalIndent(spec, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(b.path, "config.json"), content, 0644)
}

func (r *defaultRunc) createSpec(ctx context.Context, bundle string) error {
	defer trace.StartRegion(ctx, "runc.Spec").End()
	log.Trace().Str("bundle", bundle).Msg("creating container createSpec")
	output, err := r.command(ctx, "spec", "--bundle", bundle).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, output)
	}
	return nil
}

func (r *defaultRunc) command(ctx context.Context, args ...string) *exec.Cmd {
	nsenterArgs := []string{"-t", "1", "-C", "--", "runc"}
	nsenterArgs = append(nsenterArgs, r.defaultArgs()...)
	nsenterArgs = append(nsenterArgs, args...)
	return RootCommandContext(ctx, "nsenter", nsenterArgs...)
}

func (r *defaultRunc) defaultArgs() []string {
	var out []string
	if r.cfg.Root != "" {
		out = append(out, "--root", r.cfg.Root)
	}
	if r.cfg.Debug {
		out = append(out, "--debug")
	}
	if r.cfg.SystemdCgroup {
		out = append(out, "--systemd-cgroup")
	}
	if r.cfg.Rootless != "" {
		out = append(out, "--rootless", r.cfg.Rootless)
	}
	return out
}

func extractImage(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		//image was already extracted.
		return nil
	}

	err := os.MkdirAll(dst, 0775)
	if err != nil {
		return fmt.Errorf("failed to create directory '%s': %w", dst, err)
	}

	log.Trace().Str("src", src).Str("dst", dst).Msg("extracting sidecar image")
	out, err := exec.Command("tar", "-C", dst, "-xf", src).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}

func unmarshalGuarded(output []byte, v any) error {
	err := json.Unmarshal(output, v)
	if err == nil {
		return nil
	}

	if output[0] != '{' && bytes.Contains(output, []byte("{")) && bytes.Contains(output, []byte("}")) {
		if err := json.Unmarshal(output[bytes.IndexByte(output, '{'):], v); err == nil {
			return nil
		}
	}

	return err
}

func withDefaults(spec *specs.Spec) {
	spec.Root.Path = "rootfs"
	spec.Root.Readonly = true
	spec.Process.Terminal = false
	WithNamespace(LinuxNamespace{Type: specs.MountNamespace})(spec)
}

func WithMountIfNotPresent(mount specs.Mount) SpecEditor {
	return func(spec *specs.Spec) {
		for _, m := range spec.Mounts {
			if m.Destination == mount.Destination {
				return
			}
		}
		spec.Mounts = append(spec.Mounts, mount)
	}
}

func WithDisableOOMKiller() SpecEditor {
	return func(spec *specs.Spec) {
		t := true
		if spec.Linux.Resources == nil {
			spec.Linux.Resources = &specs.LinuxResources{}
		}

		if spec.Linux.Resources.Memory == nil {
			spec.Linux.Resources.Memory = &specs.LinuxMemory{}

		}

		spec.Linux.Resources.Memory.DisableOOMKiller = &t
	}
}

func WithHostname(hostname string) SpecEditor {
	return func(spec *specs.Spec) {
		WithNamespace(LinuxNamespace{Type: specs.UTSNamespace})(spec)
		spec.Hostname = hostname
	}
}

func WithAnnotations(annotations map[string]string) SpecEditor {
	return func(spec *specs.Spec) {
		spec.Annotations = annotations
	}
}

func WithProcessArgs(args ...string) SpecEditor {
	return func(spec *specs.Spec) {
		spec.Process.Args = args
	}
}
func WithProcessCwd(cwd string) SpecEditor {
	return func(spec *specs.Spec) {
		spec.Process.Cwd = cwd
	}
}

func WithCapabilities(caps ...string) SpecEditor {
	return func(spec *specs.Spec) {
		for _, c := range caps {
			spec.Process.Capabilities.Bounding = appendIfMissing(spec.Process.Capabilities.Bounding, c)
			spec.Process.Capabilities.Effective = appendIfMissing(spec.Process.Capabilities.Effective, c)
			spec.Process.Capabilities.Inheritable = appendIfMissing(spec.Process.Capabilities.Inheritable, c)
			spec.Process.Capabilities.Permitted = appendIfMissing(spec.Process.Capabilities.Effective, c)
			spec.Process.Capabilities.Ambient = appendIfMissing(spec.Process.Capabilities.Ambient, c)
		}
	}
}

func appendIfMissing(list []string, str string) []string {
	for _, item := range list {
		if item == str {
			return list
		}
	}
	return append(list, str)
}

func WithCgroupPath(cgroupPath, child string) SpecEditor {
	return func(spec *specs.Spec) {
		spec.Linux.CgroupsPath = filepath.Join(cgroupPath, child)
	}
}

func WithNamespaces(ns []LinuxNamespace) SpecEditor {
	return func(spec *specs.Spec) {
		for _, namespace := range ns {
			WithNamespace(namespace)(spec)
		}
	}
}

func WithNamespace(ns LinuxNamespace) SpecEditor {
	return func(spec *specs.Spec) {
		ns := specs.LinuxNamespace{Type: ns.Type, Path: ns.Path}

		for i, namespace := range spec.Linux.Namespaces {
			if namespace.Type == ns.Type {
				spec.Linux.Namespaces[i] = ns
				return
			}
		}
		spec.Linux.Namespaces = append(spec.Linux.Namespaces, ns)
	}
}
