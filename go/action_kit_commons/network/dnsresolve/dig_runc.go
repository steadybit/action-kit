// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

//go:build !windows

package dnsresolve

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
)

type digRunc struct {
	runc          ociruntime.OciRuntime
	targetProcess ociruntime.LinuxProcessInfo
}

func NewDigRunc(r ociruntime.OciRuntime, targetProcess ociruntime.LinuxProcessInfo) Resolver {
	return &digRunc{runc: r, targetProcess: targetProcess}
}

func (d *digRunc) Resolve(ctx context.Context, hostnames ...string) ([]net.IP, error) {
	return resolve(ctx, d, hostnames...)
}

func (d *digRunc) run(ctx context.Context, arg []string, stdin io.Reader) ([]byte, error) {
	id := fmt.Sprintf("sb-dig-%d-%s", time.Now().UnixMilli(), uuid.New().String())

	bundle, err := d.runc.Create(ctx, "/", id)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := bundle.Remove(); err != nil {
			log.Warn().Str("id", id).Err(err).Msg("failed to remove bundle")
		}
	}()

	if err := bundle.CopyFileFromProcess(ctx, d.targetProcess.Pid, "/etc/resolv.conf", "/etc/resolv.conf"); err != nil {
		log.Warn().Err(err).Msg("failed to copy /etc/resolv.conf")
	}

	if err := bundle.CopyFileFromProcess(ctx, d.targetProcess.Pid, "/etc/hosts", "/etc/hosts"); err != nil {
		log.Warn().Err(err).Msg("failed to copy /etc/hosts")
	}

	ociruntime.RefreshNamespaces(ctx, d.targetProcess.Namespaces, specs.NetworkNamespace)

	digPath := utils.LocateExecutable("dig", "STEADYBIT_EXTENSION_DIG_PATH")
	if err = bundle.EditSpec(
		ociruntime.WithHostname(fmt.Sprintf("dig-%s", id)),
		ociruntime.WithAnnotations(map[string]string{
			"com.steadybit.sidecar": "true",
		}),
		ociruntime.WithNamespaces(ociruntime.FilterNamespaces(d.targetProcess.Namespaces, specs.NetworkNamespace)),
		ociruntime.WithCapabilities("CAP_NET_ADMIN"),
		ociruntime.WithCopyEnviron(),
		ociruntime.WithProcessArgs(append([]string{digPath}, arg...)...),
	); err != nil {
		return nil, err
	}

	var outb, errb bytes.Buffer
	err = d.runc.Run(ctx, bundle, ociruntime.IoOpts{Stdin: stdin, Stdout: &outb, Stderr: &errb})
	defer func() {
		if err := d.runc.Delete(context.Background(), id, true); err != nil {
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
