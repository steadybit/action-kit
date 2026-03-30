// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

//go:build !windows

package dnsinject

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
)

// dnsInjectRunc runs dns-inject in an OCI sidecar container for unnamed network namespaces
type dnsInjectRunc struct {
	processBase
	bundle ociruntime.ContainerBundle
	runc   ociruntime.OciRuntime
	opts   Opts
}

func newRuncProcess(ctx context.Context, r ociruntime.OciRuntime, targetProcess ociruntime.LinuxProcessInfo, id string, opts Opts) (DNSInject, error) {
	containerId := fmt.Sprintf("sb-dns-inject-%d-%s", time.Now().UnixMilli(), id)

	bundle, err := r.Create(ctx, "/", containerId)
	if err != nil {
		return nil, fmt.Errorf("failed to create bundle: %w", err)
	}

	processArgs := append([]string{dnsInjectPath}, opts.toArgs()...)

	if err := bundle.EditSpec(
		ociruntime.WithHostname(containerId),
		ociruntime.WithAnnotations(map[string]string{"com.steadybit.sidecar": "true"}),
		ociruntime.WithNamespaces(ociruntime.FilterNamespaces(targetProcess.Namespaces, specs.NetworkNamespace)),
		ociruntime.WithCapabilities("CAP_NET_ADMIN", "CAP_BPF", "CAP_SYS_ADMIN"),
		ociruntime.WithCopyEnviron(),
		ociruntime.WithProcessArgs(processArgs...),
	); err != nil {
		_ = bundle.Remove()
		return nil, fmt.Errorf("failed to configure bundle: %w", err)
	}

	return &dnsInjectRunc{
		processBase: processBase{exited: make(chan error, 1)},
		bundle:      bundle,
		runc:        r,
		opts:        opts,
	}, nil
}

func (d *dnsInjectRunc) Start() error {
	log.Trace().
		Str("containerId", d.bundle.ContainerId()).
		Str("cmd", d.opts.String()).
		Msg("starting dns-inject via runc sidecar")

	cmd, err := d.runc.RunCommand(context.Background(), d.bundle)
	if err != nil {
		return fmt.Errorf("failed to create run command: %w", err)
	}

	return d.startAndMonitor(cmd, d.bundle.ContainerId())
}

func (d *dnsInjectRunc) Stop() error {
	log.Info().
		Str("containerId", d.bundle.ContainerId()).
		Msg("stopping dns-inject")

	ctx := context.Background()
	if err := d.runc.Kill(ctx, d.bundle.ContainerId(), syscall.SIGINT); err != nil {
		log.Warn().Str("id", d.bundle.ContainerId()).Err(err).Msg("failed to send SIGINT")
	}

	timer := time.AfterFunc(10*time.Second, func() {
		if err := d.runc.Kill(ctx, d.bundle.ContainerId(), syscall.SIGTERM); err != nil {
			log.Warn().Str("id", d.bundle.ContainerId()).Err(err).Msg("failed to send SIGTERM")
		}
	})

	<-d.exited
	timer.Stop()

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
