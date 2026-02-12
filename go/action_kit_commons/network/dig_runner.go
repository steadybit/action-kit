// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package network

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"io"
	"os/exec"
)

type DigRunner interface {
	Run(ctx context.Context, arg []string, stdin io.Reader) ([]byte, error)
}

type CommandDigRunner struct {
}

func (c *CommandDigRunner) Run(ctx context.Context, arg []string, stdin io.Reader) ([]byte, error) {
	var outb, errb bytes.Buffer

	cmd := exec.CommandContext(ctx, "dig", arg...)
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	cmd.Stdin = stdin

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("could not resolve hostnames: %w: %s", err, errb.String())
	}

	return outb.Bytes(), nil
}

type RuncDigRunner struct {
	Runc    ociruntime.OciRuntime
	Sidecar SidecarOpts
}

var _ DigRunner = (*RuncDigRunner)(nil)

func (r *RuncDigRunner) Run(ctx context.Context, arg []string, stdin io.Reader) ([]byte, error) {
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

	ociruntime.RefreshNamespaces(ctx, r.Sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)

	digPath := utils.LocateExecutable("dig", "STEADYBIT_EXTENSION_DIG_PATH")
	if err = bundle.EditSpec(
		ociruntime.WithHostname(fmt.Sprintf("dig-%s", id)),
		ociruntime.WithAnnotations(map[string]string{
			"com.steadybit.sidecar": "true",
		}),
		ociruntime.WithNamespaces(ociruntime.FilterNamespaces(r.Sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)),
		ociruntime.WithCapabilities("CAP_NET_ADMIN"),
		ociruntime.WithCopyEnviron(),
		ociruntime.WithProcessArgs(append([]string{digPath}, arg...)...),
	); err != nil {
		return nil, err
	}

	var outb, errb bytes.Buffer
	err = r.Runc.Run(ctx, bundle, ociruntime.IoOpts{Stdin: stdin, Stdout: &outb, Stderr: &errb})
	defer func() {
		if err := r.Runc.Delete(context.Background(), id, true); err != nil {
			level := zerolog.WarnLevel
			if errors.Is(err, ociruntime.ErrContainerNotFound) {
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
