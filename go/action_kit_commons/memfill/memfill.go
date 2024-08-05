// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package memfill

import (
	"context"
	"errors"
	"fmt"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/runc"
	"syscall"
	"time"
)

type MemFill struct {
	bundle runc.ContainerBundle
	runc   runc.Runc

	state *runc.BackgroundState
	args  []string
	Noop  bool
}

type Mode string
type Unit string

const (
	ModeUsage    Mode = "usage"
	ModeAbsolute Mode = "absolute"
	UnitPercent  Unit = "%"
	UnitMegabyte Unit = "MiB"
)

type Opts struct {
	BinaryPath string
	Size       int
	Mode       Mode
	Unit       Unit
	Duration   time.Duration
}

func (o Opts) processArgs() []string {
	args := []string{o.BinaryPath, fmt.Sprintf("%d%s", o.Size, o.Unit), string(o.Mode), fmt.Sprintf("%.0f", o.Duration.Seconds())}
	if len(args[0]) == 0 {
		args[0] = "memfill"
	}
	return args
}

func New(ctx context.Context, r runc.Runc, sidecar SidecarOpts, opts Opts) (*MemFill, error) {
	bundle, err := createBundle(ctx, r, sidecar, opts.processArgs()...)
	if err != nil {
		log.Error().Err(err).Msg("failed to create start bundle")
		return nil, err
	}

	return &MemFill{
		bundle: bundle,
		runc:   r,
	}, nil
}

func (mf *MemFill) Exited() (bool, error) {
	return mf.state.Exited()
}

func (mf *MemFill) Start() error {
	log.Info().
		Str("containerId", mf.bundle.ContainerId()).
		Strs("args", mf.args).
		Msg("Starting memfill")

	if state, err := runc.RunBundleInBackground(context.Background(), mf.runc, mf.bundle); err != nil {
		return fmt.Errorf("failed to start memfill: %w", err)
	} else {
		mf.state = state
	}
	return nil
}

func (mf *MemFill) Stop() error {
	log.Info().
		Str("containerId", mf.bundle.ContainerId()).
		Msg("stopping memfill")
	ctx := context.Background()

	if err := mf.runc.Kill(ctx, mf.bundle.ContainerId(), syscall.SIGINT); err != nil {
		log.Warn().Str("id", mf.bundle.ContainerId()).Err(err).Msg("failed to send SIGINT to container")
	}

	timerStart := time.AfterFunc(10*time.Second, func() {
		if err := mf.runc.Kill(ctx, mf.bundle.ContainerId(), syscall.SIGTERM); err != nil {
			log.Warn().Str("id", mf.bundle.ContainerId()).Err(err).Msg("failed to send SIGTERM to container")
		}
	})

	mf.state.Wait()
	timerStart.Stop()

	if err := mf.runc.Delete(ctx, mf.bundle.ContainerId(), false); err != nil {
		level := zerolog.WarnLevel
		if errors.Is(err, runc.ErrContainerNotFound) {
			level = zerolog.DebugLevel
		}
		log.WithLevel(level).Str("id", mf.bundle.ContainerId()).Err(err).Msg("failed to delete container")
	}

	if err := mf.bundle.Remove(); err != nil {
		log.Warn().Str("id", mf.bundle.ContainerId()).Err(err).Msg("failed to remove bundle")
	}
	return nil
}

func (mf *MemFill) Args() []string {
	return mf.args
}

type SidecarOpts struct {
	TargetProcess runc.LinuxProcessInfo
	IdSuffix      string
	ImagePath     string
}

func createBundle(ctx context.Context, r runc.Runc, sidecar SidecarOpts, processArgs ...string) (runc.ContainerBundle, error) {
	containerId := getNextContainerId(sidecar.IdSuffix)
	bundle, err := r.Create(ctx, sidecar.ImagePath, containerId)
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

	runc.RefreshNamespaces(ctx, sidecar.TargetProcess.Namespaces, specs.PIDNamespace, specs.CgroupNamespace)

	if err := bundle.EditSpec(
		runc.WithHostname(containerId),
		runc.WithAnnotations(map[string]string{
			"com.steadybit.sidecar": "true",
		}),
		runc.WithProcessArgs(processArgs...),
		runc.WithProcessCwd("/tmp"),
		runc.WithCgroupPath(sidecar.TargetProcess.CGroupPath, containerId),
		runc.WithDisableOOMKiller(),
		runc.WithNamespaces(runc.FilterNamespaces(sidecar.TargetProcess.Namespaces, specs.PIDNamespace, specs.CgroupNamespace)),
		runc.WithCapabilities("CAP_SYS_RESOURCE"),
		runc.WithMountIfNotPresent(specs.Mount{
			Destination: "/tmp",
			Type:        "tmpfs",
			Options:     []string{"noexec", "nosuid", "nodev", "rprivate"},
		}),
	); err != nil {
		return nil, err
	}

	success = true

	return bundle, nil
}

func getNextContainerId(suffix string) string {
	return fmt.Sprintf("sb-memfill-%d-%s", time.Now().UnixMilli(), suffix)
}
