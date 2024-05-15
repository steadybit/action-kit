package diskfill

import (
	"context"
	"fmt"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/runc"
	"time"
)

type SidecarOpts struct {
	TargetProcess runc.LinuxProcessInfo
	IdSuffix      string
	ImagePath     string
}

func createBundle(ctx context.Context, r runc.Runc, sidecar SidecarOpts, opts Opts, processArgs ...string) (runc.ContainerBundle, error) {
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

	if opts.TempPath != "" {
		if err := bundle.MountFromProcess(ctx, sidecar.TargetProcess.Pid, opts.TempPath, mountpointInContainer); err != nil {
			log.Warn().Err(err).Msgf("failed to mount %s", opts.TempPath)
		}
	}

	runc.RefreshNamespaces(ctx, sidecar.TargetProcess.Namespaces, specs.PIDNamespace)

	if err := bundle.EditSpec(
		runc.WithHostname(containerId),
		runc.WithAnnotations(map[string]string{
			"com.steadybit.sidecar": "true",
		}),
		runc.WithProcessArgs(processArgs...),
		runc.WithProcessCwd("/tmp"),
		runc.WithCgroupPath(sidecar.TargetProcess.CGroupPath, containerId),
		runc.WithNamespaces(runc.FilterNamespaces(sidecar.TargetProcess.Namespaces, specs.PIDNamespace)),
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
	return fmt.Sprintf("sb-diskfill-%d-%s", time.Now().UnixMilli(), suffix)
}
