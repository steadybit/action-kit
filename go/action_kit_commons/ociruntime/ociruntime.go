// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH
//go:build !windows

package ociruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/moby/sys/capability"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const steadybitBundlesPath = "/tmp/steadybit/containers"
const steadybitRoot = "/tmp/steadybit/oci-root"

var (
	ErrContainerNotFound = errors.New("container not found")
	nsmountPath          = initNsMountPath()
)

func initNsMountPath() string {
	path := "nsmount"

	if fromEnv := os.Getenv("STEADYBIT_EXTENSION_NSMOUNT_PATH"); fromEnv != "" {
		path = fromEnv
	} else if fromEnv = os.Getenv("STEADYBIT_EXTENSION_RUNC_NSMOUNT_PATH"); fromEnv != "" {
		log.Warn().Msg("The STEADYBIT_EXTENSION_RUNC_NSMOUNT_PATH environment variable is deprecated, please use STEADYBIT_EXTENSION_NSMOUNT_PATH instead.")
		path = fromEnv
	}

	if lookupPath, err := exec.LookPath(path); err == nil {
		return lookupPath
	} else {
		return path
	}
}

type OciRuntime interface {
	State(ctx context.Context, id string) (*ContainerState, error)
	Create(ctx context.Context, image, id string) (ContainerBundle, error)
	Run(ctx context.Context, container ContainerBundle, ioOpts IoOpts) error
	RunCommand(ctx context.Context, container ContainerBundle) (*exec.Cmd, error)
	Delete(ctx context.Context, id string, force bool) error
	Kill(ctx context.Context, id string, signal syscall.Signal) error
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
	Path          string `json:"path" split_words:"true" default:"runc"`
	Root          string `json:"root" split_words:"true" required:"false"`
	Debug         bool   `json:"debug" split_words:"true" required:"false"`
	SystemdCgroup bool   `json:"systemdCgroup" split_words:"true" required:"false"`
	Rootless      string `json:"rootless" split_words:"true" required:"false"`
}

type defaultRuntime struct {
	cfg        Config
	cachedSpec struct {
		value []byte
		err   error
	}
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

	//we changed the prefix, we need to check if the old prefix is configured
	prefix := "steadybit_extension_ociruntime"
	if slices.ContainsFunc(os.Environ(), func(env string) bool {
		return strings.HasPrefix(strings.ToLower(env), "steadybit_extension_runc")
	}) {
		log.Warn().Msg("The STEADYBIT_EXTENSION_RUNC_* environment variables are deprecated, please use STEADYBIT_EXTENSION_OCIRUNTIME_* instead.")
		prefix = "steadybit_extension_runc"
	}

	if err := envconfig.Process(prefix, &cfg); err != nil {
		log.Fatal().Err(err).Msgf("Failed to parse OCI runtime configuration from environment.")
	}
	log.Info().Any("config", cfg).Msg("OCI runtime configuration loaded")
	return cfg
}

func NewOciRuntimeWithCrunForSidecars(cfg Config) OciRuntime {
	runtime := NewOciRuntime(cfg)

	if crunPath, err := exec.LookPath("crun"); err == nil {
		if ok, _ := capability.GetBound(capability.CAP_MKNOD); !ok {
			return runtime
		}
		if ok, _ := capability.GetBound(capability.CAP_SETPCAP); !ok {
			return runtime
		}
		if _, err := os.Open("/sys/fs/cgroup/cpu,cpuacct"); err == nil {
			return runtime // crun is not compatible with cgroup v1
		}

		if err = os.MkdirAll(steadybitRoot, 0755); err != nil {
			return runtime
		}

		//the sb runtime (using crun) is used for all `sb-` containers, when available.
		sbCfg := cfg
		sbCfg.Path = crunPath
		sbCfg.Root = steadybitRoot
		sbRuntime := NewOciRuntime(sbCfg)
		log.Info().Any("config", sbCfg).Msg("Using crun as OCI runtime for steadybit containers.")
		return NewDelegatingOciRuntime(runtime, map[string]OciRuntime{"sb-": sbRuntime})
	}

	return runtime
}

func NewOciRuntime(cfg Config) OciRuntime {
	if lookupPath, err := exec.LookPath(cfg.Path); err == nil {
		cfg.Path = lookupPath
	}

	r := &defaultRuntime{cfg: cfg}

	r.generateCachedSpec()
	return r
}

func (r *defaultRuntime) State(ctx context.Context, id string) (*ContainerState, error) {
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

func (r *defaultRuntime) Create(ctx context.Context, image string, id string) (ContainerBundle, error) {
	bundle := containerBundle{
		id:      id,
		path:    filepath.Join(steadybitBundlesPath, id),
		runtime: r,
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

func (r *defaultRuntime) Delete(ctx context.Context, id string, force bool) error {
	log.Trace().Str("id", id).Msg("deleting container")

	var args = []string{"delete"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, id)

	if output, err := r.command(ctx, args...).CombinedOutput(); err != nil {
		return r.toError(err, output)
	}
	return nil
}

func (r *defaultRuntime) toError(err error, output []byte) error {
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

func (r *defaultRuntime) Run(ctx context.Context, container ContainerBundle, ioOpts IoOpts) error {
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

func (r *defaultRuntime) RunCommand(ctx context.Context, container ContainerBundle) (*exec.Cmd, error) {
	bundle, ok := container.(*containerBundle)
	if !ok {
		return nil, fmt.Errorf("invalid bundle type: %T", container)
	}

	return r.command(ctx, "run", "--bundle", bundle.path, bundle.id), nil
}

func (r *defaultRuntime) Kill(ctx context.Context, id string, signal syscall.Signal) error {
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
	log.Trace().Str("bundle", b.path).Interface("createSpec", spec).Msg("written runtime createSpec")
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

func (r *defaultRuntime) generateCachedSpec() {
	bundle := filepath.Join(steadybitBundlesPath, "temp")
	if err := os.MkdirAll(bundle, 0775); err != nil {
		r.cachedSpec.err = fmt.Errorf("failed to create temporary bundle directory '%s': %w", bundle, err)
		return
	}
	defer func() { _ = os.RemoveAll(bundle) }()

	if output, err := r.command(context.Background(), "spec", "--bundle", bundle).CombinedOutput(); err != nil {
		r.cachedSpec.err = fmt.Errorf("failed to generate runtime spec: %w: %s", err, output)
		return
	}

	var err error
	r.cachedSpec.value, err = os.ReadFile(filepath.Join(bundle, "config.json"))
	if err != nil {
		r.cachedSpec.err = fmt.Errorf("failed to read runtime spec: %w", err)
	}
}

func (r *defaultRuntime) createSpec(_ context.Context, bundle string) error {
	log.Trace().Str("bundle", bundle).Msg("creating container createSpec")
	if r.cachedSpec.err != nil {
		return fmt.Errorf("%w: %s", r.cachedSpec.err, r.cachedSpec.value)
	}
	return os.WriteFile(filepath.Join(bundle, "config.json"), r.cachedSpec.value, 0644)
}

func (r *defaultRuntime) command(ctx context.Context, args ...string) *exec.Cmd {
	runtimeArgs := append(r.defaultArgs(), args...)
	nsenterArgs := append([]string{"-t", "1", "-C", "--", r.cfg.Path}, runtimeArgs...)
	log.Trace().Str("path", r.cfg.Path).Strs("args", runtimeArgs).Msg("exec oci-runtime")
	return utils.RootCommandContext(ctx, nsenterPath, nsenterArgs...)
}

func (r *defaultRuntime) defaultArgs() []string {
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

func WithOOMScoreAdj(adj int) SpecEditor {
	return func(spec *specs.Spec) {
		if spec.Process == nil {
			spec.Process = &specs.Process{}
		}

		spec.Process.OOMScoreAdj = &adj
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

func WithCopyEnviron() SpecEditor {
	env := os.Environ()
	return func(spec *specs.Spec) {
		spec.Process.Env = append(spec.Process.Env, env...)
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
