// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package stress

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/moby/sys/capability"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/runc"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"strconv"
	"syscall"
	"time"
)

const mountPointInContainer = "/stress-temp"

type Stress struct {
	bundle runc.ContainerBundle
	runc   runc.Runc

	state *runc.BackgroundState
	args  []string
}

type Opts struct {
	CpuWorkers   *int
	CpuLoad      int
	HddWorkers   *int
	HddBytes     string
	IoWorkers    *int
	IomixWorkers *int
	IomixBytes   string
	TempPath     string
	Timeout      time.Duration
	VmWorkers    *int
	VmHang       time.Duration
	VmBytes      string
}

type SidecarOpts struct {
	TargetProcess runc.LinuxProcessInfo
	IdSuffix      string
	ImagePath     string
	ExecutionId   uuid.UUID
}

func (o *Opts) Args() []string {
	args := []string{"--timeout", strconv.Itoa(int(o.Timeout.Seconds()))}
	if o.CpuWorkers != nil {
		args = append(args, "--cpu", strconv.Itoa(*o.CpuWorkers), "--cpu-load", strconv.Itoa(o.CpuLoad))
	}
	if o.HddWorkers != nil {
		args = append(args, "--hdd", strconv.Itoa(*o.HddWorkers))
	}
	if o.HddBytes != "" {
		args = append(args, "--hdd-bytes", o.HddBytes)
	}
	if o.IoWorkers != nil {
		args = append(args, "--io", strconv.Itoa(*o.IoWorkers))
	}
	if o.IomixWorkers != nil {
		args = append(args, "--iomix", strconv.Itoa(*o.IomixWorkers))
	}
	if o.IomixBytes != "" {
		args = append(args, "--iomix-bytes", strconv.Itoa(*o.IomixWorkers))
	}
	if o.TempPath != "" {
		args = append(args, "--temp-path", o.TempPath)
	}
	if o.VmWorkers != nil {
		args = append(args, "--vm", strconv.Itoa(*o.VmWorkers), "--vm-bytes", o.VmBytes, "--vm-hang", "0")
	}
	if log.Trace().Enabled() {
		args = append(args, "-v")
	}
	return args
}

func New(ctx context.Context, r runc.Runc, sidecar SidecarOpts, opts Opts) (*Stress, error) {
	containerId := getNextContainerId(sidecar.ExecutionId, sidecar.IdSuffix)

	bundle, err := r.Create(ctx, "/", containerId)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare bundle: %w", err)
	}

	success := false
	defer func() {
		if success {
			return
		}
		if err := bundle.Remove(); err != nil {
			log.Warn().Str("id", containerId).Err(err).Msg("failed to remove bundle")
		}
	}()

	if opts.TempPath != "" {
		if err := bundle.MountFromProcess(ctx, sidecar.TargetProcess.Pid, opts.TempPath, mountPointInContainer); err == nil {
			opts.TempPath = mountPointInContainer
		} else {
			log.Warn().Err(err).Msgf("failed to mount %s", opts.TempPath)
		}
	}

	runc.RefreshNamespaces(ctx, sidecar.TargetProcess.Namespaces, specs.PIDNamespace, specs.CgroupNamespace)

	stressngPath := utils.LocateExecutable("stress-ng", "STEADYBIT_EXTENSION_STRESSNG_PATH")
	processArgs := append([]string{stressngPath}, opts.Args()...)

	caps := []string{"CAP_DAC_OVERRIDE"}
	if ok, _ := capability.GetBound(capability.CAP_SYS_RESOURCE); ok {
		caps = append(caps, "CAP_SYS_RESOURCE")
	} else {
		log.Warn().Msg("CAP_SYS_RESOURCE not available. oom_score_adj will fail.")
	}

	editors := []runc.SpecEditor{
		runc.WithHostname(containerId),
		runc.WithAnnotations(map[string]string{
			"com.steadybit.sidecar": "true",
		}),
		runc.WithCopyEnviron(),
		runc.WithProcessArgs(processArgs...),
		runc.WithProcessCwd("/tmp"),
		runc.WithCgroupPath(sidecar.TargetProcess.CGroupPath, containerId),
		runc.WithCapabilities(caps...),
		runc.WithDisableOOMKiller(),
		runc.WithNamespaces(runc.FilterNamespaces(sidecar.TargetProcess.Namespaces, specs.PIDNamespace, specs.CgroupNamespace)),
		runc.WithMountIfNotPresent(specs.Mount{
			Destination: "/tmp",
			Type:        "tmpfs",
			Options:     []string{"noexec", "nosuid", "nodev", "rprivate"},
		}),
	}

	if err := bundle.EditSpec(editors...); err != nil {
		return nil, err
	}

	success = true
	return &Stress{
		bundle: bundle,
		runc:   r,
		args:   processArgs,
	}, nil
}

func getNextContainerId(executionId uuid.UUID, suffix string) string {
	return fmt.Sprintf("sb-stress-%d-%s-%s", time.Now().UnixMilli(), utils.ShortenUUID(executionId), suffix)
}

func (s *Stress) Exited() (bool, error) {
	return s.state.Exited()
}

func (s *Stress) Start() error {
	log.Info().
		Str("containerId", s.bundle.ContainerId()).
		Strs("args", s.args).
		Msg("Starting stress-ng")

	if state, err := runc.RunBundleInBackground(context.Background(), s.runc, s.bundle); err != nil {
		return fmt.Errorf("failed to start stress-ng: %w", err)
	} else {
		s.state = state
	}
	return nil
}

func (s *Stress) Stop() {
	log.Info().
		Str("containerId", s.bundle.ContainerId()).
		Msg("Stopping stress-ng")

	ctx := context.Background()
	if err := s.runc.Kill(ctx, s.bundle.ContainerId(), syscall.SIGINT); err != nil {
		log.Warn().Str("id", s.bundle.ContainerId()).Err(err).Msg("failed to send SIGINT to container")
	}

	timer := time.AfterFunc(10*time.Second, func() {
		if err := s.runc.Kill(ctx, s.bundle.ContainerId(), syscall.SIGTERM); err != nil {
			log.Warn().Str("id", s.bundle.ContainerId()).Err(err).Msg("failed to send SIGTERM to container")
		}
	})

	s.state.Wait()
	timer.Stop()

	if err := s.runc.Delete(ctx, s.bundle.ContainerId(), false); err != nil {
		level := zerolog.WarnLevel
		if errors.Is(err, runc.ErrContainerNotFound) {
			level = zerolog.DebugLevel
		}
		log.WithLevel(level).Str("id", s.bundle.ContainerId()).Err(err).Msg("failed to delete container")
	}

	if err := s.bundle.Remove(); err != nil {
		log.Warn().Str("id", s.bundle.ContainerId()).Err(err).Msg("failed to remove bundle")
	}
}
