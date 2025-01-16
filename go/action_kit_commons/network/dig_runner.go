// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package network

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/runc"
	"io"
	"runtime/trace"
)

type RuncDigRunner struct {
	Runc    runc.Runc
	Sidecar SidecarOpts
}

var _ DigRunner = (*RuncDigRunner)(nil)

func (r *RuncDigRunner) Run(ctx context.Context, arg []string, stdin io.Reader) ([]byte, error) {
	defer trace.StartRegion(ctx, "RuncDigRunner.Run").End()
	id := getNextContainerId(r.Sidecar.ExecutionId, "dig", r.Sidecar.IdSuffix)

	bundle, err := r.Runc.Create(ctx, "/", id)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := bundle.Remove(); err != nil {
			log.Warn().Str("id", id).Err(err).Msg("failed to remove bundle")
		}
	}()

	if err := bundle.CopyFileFromProcess(ctx, r.Sidecar.TargetProcess.Pid, "/etc/resolv.conf", "/etc/resolv.conf"); err != nil {
		log.Warn().Err(err).Msg("failed to copy /etc/resolv.conf")
	}

	if err := bundle.CopyFileFromProcess(ctx, r.Sidecar.TargetProcess.Pid, "/etc/hosts", "/etc/hosts"); err != nil {
		log.Warn().Err(err).Msg("failed to copy /etc/hosts")
	}

	runc.RefreshNamespaces(ctx, r.Sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)

	if err = bundle.EditSpec(
		runc.WithHostname(fmt.Sprintf("dig-%s", id)),
		runc.WithAnnotations(map[string]string{
			"com.steadybit.sidecar": "true",
		}),
		runc.WithNamespaces(runc.FilterNamespaces(r.Sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)),
		runc.WithCapabilities("CAP_NET_ADMIN"),
		runc.WithProcessArgs(append([]string{"dig"}, arg...)...),
	); err != nil {
		return nil, err
	}

	var outb, errb bytes.Buffer
	err = r.Runc.Run(ctx, bundle, runc.IoOpts{Stdin: stdin, Stdout: &outb, Stderr: &errb})
	defer func() {
		if err := r.Runc.Delete(context.Background(), id, true); err != nil {
			level := zerolog.WarnLevel
			if errors.Is(err, runc.ErrContainerNotFound) {
				level = zerolog.DebugLevel
			}
			log.WithLevel(level).Str("id", id).Err(err).Msg("failed to delete container")
		}
	}()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, errb.String())
	}
	return outb.Bytes(), nil
}
