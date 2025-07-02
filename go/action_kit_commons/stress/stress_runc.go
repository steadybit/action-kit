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
	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"syscall"
	"time"
)

const mountPointInContainer = "/stress-temp"

type stressRunc struct {
	bundle ociruntime.ContainerBundle
	runc   ociruntime.OciRuntime

	state *utils.BackgroundState
	args  []string
}

type SidecarOpts struct {
	TargetProcess ociruntime.LinuxProcessInfo
	IdSuffix      string
	ImagePath     string
	ExecutionId   uuid.UUID
}

func NewStressRunc(ctx context.Context, r ociruntime.OciRuntime, sidecar SidecarOpts, opts Opts) (Stress, error) {
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

	ociruntime.RefreshNamespaces(ctx, sidecar.TargetProcess.Namespaces, specs.PIDNamespace, specs.CgroupNamespace)

	stressngPath := utils.LocateExecutable("stress-ng", "STEADYBIT_EXTENSION_STRESSNG_PATH")
	processArgs := append([]string{stressngPath}, opts.Args()...)

	editors := []ociruntime.SpecEditor{
		ociruntime.WithHostname(containerId),
		ociruntime.WithAnnotations(map[string]string{
			"com.steadybit.sidecar": "true",
		}),
		ociruntime.WithCopyEnviron(),
		ociruntime.WithProcessArgs(processArgs...),
		ociruntime.WithProcessCwd("/tmp"),
		ociruntime.WithCgroupPath(sidecar.TargetProcess.CGroupPath, containerId),
		ociruntime.WithNamespaces(ociruntime.FilterNamespaces(sidecar.TargetProcess.Namespaces, specs.PIDNamespace, specs.CgroupNamespace)),
		ociruntime.WithMountIfNotPresent(specs.Mount{
			Destination: "/tmp",
			Type:        "tmpfs",
			Options:     []string{"noexec", "nosuid", "nodev", "rprivate"},
		}),
	}
	caps := []string{"CAP_DAC_OVERRIDE"}
	if ok, _ := capability.GetBound(capability.CAP_SYS_RESOURCE); ok {
		caps = append(caps, "CAP_SYS_RESOURCE")
		editors = append(editors, ociruntime.WithOOMScoreAdj(-1000))
	} else {
		log.Warn().Msg("CAP_SYS_RESOURCE not available. Cannot prevent OOM kill.")
	}
	editors = append(editors, ociruntime.WithCapabilities(caps...))

	if err := bundle.EditSpec(editors...); err != nil {
		return nil, err
	}

	success = true
	return &stressRunc{
		bundle: bundle,
		runc:   r,
		args:   processArgs,
	}, nil
}

func getNextContainerId(executionId uuid.UUID, suffix string) string {
	return fmt.Sprintf("sb-stress-%d-%s-%s", time.Now().UnixMilli(), utils.ShortenUUID(executionId), suffix)
}

func (s *stressRunc) Exited() (bool, error) {
	return s.state.Exited()
}

func (s *stressRunc) Start() error {
	log.Info().
		Str("containerId", s.bundle.ContainerId()).
		Strs("args", s.args).
		Msg("Starting stress-ng")

	if state, err := ociruntime.RunBundleInBackground(context.Background(), s.runc, s.bundle); err != nil {
		return fmt.Errorf("failed to start stress-ng: %w", err)
	} else {
		s.state = state
	}
	return nil
}

func (s *stressRunc) Stop() {
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
		if errors.Is(err, ociruntime.ErrContainerNotFound) {
			level = zerolog.DebugLevel
		}
		log.WithLevel(level).Str("id", s.bundle.ContainerId()).Err(err).Msg("failed to delete container")
	}

	if err := s.bundle.Remove(); err != nil {
		log.Warn().Str("id", s.bundle.ContainerId()).Err(err).Msg("failed to remove bundle")
	}
}
