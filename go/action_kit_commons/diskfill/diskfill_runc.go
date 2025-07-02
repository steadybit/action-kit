// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package diskfill

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/moby/sys/capability"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type diskfillRunc struct {
	bundle       ociruntime.ContainerBundle
	runc         ociruntime.OciRuntime
	state        *utils.BackgroundState
	args         []string
	pathToRemove string
}

const mountpointInContainer = "/disk-fill-temp"

func NewDiskfillRunc(ctx context.Context, r ociruntime.OciRuntime, sidecar SidecarOpts, opts Opts) (Diskfill, error) {
	processArgs, err := opts.Args(mountpointInContainer, func(path string) (*DiskUsage, error) {
		return readDiskUsageRunc(context.Background(), r, sidecar, path)
	})
	if err != nil {
		return nil, err
	}

	bundle, err := createBundle(ctx, r, sidecar, opts.TempPath, processArgs...)
	if err != nil {
		log.Error().Err(err).Msg("failed to create start bundle")
		return nil, err
	}

	return &diskfillRunc{
		bundle:       bundle,
		runc:         r,
		args:         processArgs,
		pathToRemove: filepath.Join(bundle.Path(), "rootfs", "/disk-fill-temp/disk-fill"),
	}, nil
}

func (df *diskfillRunc) Exited() (bool, error) {
	return df.state.Exited()
}

func (df *diskfillRunc) Start() error {
	log.Info().
		Str("containerId", df.bundle.ContainerId()).
		Strs("args", df.args).
		Msg("Starting diskfill")

	if state, err := ociruntime.RunBundleInBackground(context.Background(), df.runc, df.bundle); err != nil {
		return fmt.Errorf("failed to start diskfill: %w", err)
	} else {
		df.state = state
	}
	return nil
}

func (df *diskfillRunc) Stop() error {
	log.Info().
		Str("containerId", df.bundle.ContainerId()).
		Msg("stopping diskfill")
	ctx := context.Background()

	//stop writer
	if err := df.runc.Kill(ctx, df.bundle.ContainerId(), syscall.SIGINT); err != nil {
		log.WithLevel(levelForErr(err)).Str("id", df.bundle.ContainerId()).Err(err).Msg("failed to send SIGINT to container")
	}

	timerStart := time.AfterFunc(10*time.Second, func() {
		if err := df.runc.Kill(ctx, df.bundle.ContainerId(), syscall.SIGTERM); err != nil {
			log.WithLevel(levelForErr(err)).Str("id", df.bundle.ContainerId()).Err(err).Msg("failed to send SIGTERM to container")
		}
	})

	df.state.Wait()
	timerStart.Stop()

	// remove file
	var deleteFileErr error
	if !df.Noop() {

		if out, err := utils.RootCommandContext(ctx, "rm", df.pathToRemove).CombinedOutput(); err != nil && !strings.Contains(string(out), "No such file or directory") {
			log.Error().Err(err).Msgf("failed to remove file %s", out)
			deleteFileErr = fmt.Errorf("failed to remove file %s! You have to remove it manually now! %s", df.pathToRemove, out)
		} else {
			log.Debug().Msgf("removed file %s: %s", df.pathToRemove, out)
		}
	}

	if err := df.runc.Delete(ctx, df.bundle.ContainerId(), false); err != nil {
		log.WithLevel(levelForErr(err)).Str("id", df.bundle.ContainerId()).Err(err).Msg("failed to delete container")
	}

	if err := df.bundle.Remove(); err != nil {
		log.Warn().Str("id", df.bundle.ContainerId()).Err(err).Msg("failed to remove bundle")
	}
	return deleteFileErr
}

func (df *diskfillRunc) Args() []string {
	return df.args
}

type SidecarOpts struct {
	TargetProcess ociruntime.LinuxProcessInfo
	IdSuffix      string
	ImagePath     string
	ExecutionId   uuid.UUID
}

func createBundle(ctx context.Context, r ociruntime.OciRuntime, sidecar SidecarOpts, targetPath string, processArgs ...string) (ociruntime.ContainerBundle, error) {
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

	if targetPath != "" {
		if err := bundle.MountFromProcess(ctx, sidecar.TargetProcess.Pid, targetPath, mountpointInContainer); err != nil {
			log.Warn().Err(err).Msgf("failed to mount %s", targetPath)
		}
	}

	ociruntime.RefreshNamespaces(ctx, sidecar.TargetProcess.Namespaces, specs.PIDNamespace)

	caps := []string{"CAP_DAC_OVERRIDE"}
	if ok, _ := capability.GetBound(capability.CAP_SYS_RESOURCE); ok {
		caps = append(caps, "CAP_SYS_RESOURCE")
	} else {
		log.Warn().Msg("CAP_SYS_RESOURCE not available. oom_score_adj will fail.")
	}

	editors := []ociruntime.SpecEditor{
		ociruntime.WithHostname(containerId),
		ociruntime.WithAnnotations(map[string]string{
			"com.steadybit.sidecar": "true",
		}),
		ociruntime.WithCopyEnviron(),
		ociruntime.WithProcessArgs(processArgs...),
		ociruntime.WithProcessCwd("/tmp"),
		ociruntime.WithCgroupPath(sidecar.TargetProcess.CGroupPath, containerId),
		ociruntime.WithCapabilities(caps...),
		ociruntime.WithNamespaces(ociruntime.FilterNamespaces(sidecar.TargetProcess.Namespaces, specs.PIDNamespace)),
		ociruntime.WithMountIfNotPresent(specs.Mount{
			Destination: "/tmp",
			Type:        "tmpfs",
			Options:     []string{"noexec", "nosuid", "nodev", "rprivate"},
		}),
	}

	if err := bundle.EditSpec(editors...); err != nil {
		return nil, err
	}

	success = true
	return bundle, nil
}

func getNextContainerId(executionId uuid.UUID, suffix string) string {
	return fmt.Sprintf("sb-diskfill-%d-%s-%s", time.Now().UnixMilli(), utils.ShortenUUID(executionId), suffix)
}

func (df *diskfillRunc) Noop() bool {
	return df.args[0] == "echo" && df.Args()[1] == "noop"
}
