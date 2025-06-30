// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package network

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/runc"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"path"
	"strconv"
	"time"
)

type runcRunner struct {
	runc    runc.Runc
	sidecar SidecarOpts
}

type SidecarOpts struct {
	TargetProcess runc.LinuxProcessInfo
	IdSuffix      string
	ExecutionId   uuid.UUID
}

func NewRuncRunner(r runc.Runc, sidecar SidecarOpts) CommandRunner {
	return &runcRunner{
		runc:    r,
		sidecar: sidecar,
	}
}

func (r *runcRunner) run(ctx context.Context, processArgs []string, cmds []string) (string, error) {
	runc.RefreshNamespaces(ctx, r.sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)

	if runc.HasNamedNetworkNamespace(r.sidecar.TargetProcess.Namespaces...) {
		return r.executeInNamedNetworkUsingIpNetNs(ctx, processArgs, cmds)
	} else {
		return r.executeInNetworkNamespaceUsingRunc(ctx, processArgs, cmds)
	}
}

func (r *runcRunner) id() string {
	for _, ns := range r.sidecar.TargetProcess.Namespaces {
		if ns.Type == specs.NetworkNamespace {
			if ns.Inode != 0 {
				return strconv.FormatUint(ns.Inode, 10)
			} else {
				return ns.Path
			}
		}
	}
	return ""
}

func (r *runcRunner) executeInNamedNetworkUsingIpNetNs(ctx context.Context, processArgs []string, cmds []string) (string, error) {
	netns := ""
	for _, n := range r.sidecar.TargetProcess.Namespaces {
		if n.Type == specs.NetworkNamespace {
			netns = n.Path
			break
		}
	}

	log.Info().Strs("cmds", cmds).Strs("processArgs", processArgs).Msg("running commands in network namespace using ip netns")

	ipArgs := append([]string{"netns", "exec", netns}, processArgs...)
	var outb, errb bytes.Buffer
	cmd := utils.RootCommandContext(ctx, ipPath, ipArgs...)
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	cmd.Stdin = ToReader(cmds)
	err := cmd.Run()

	if err != nil {
		if parsed := ParseBatchError(processArgs, bytes.NewReader(errb.Bytes())); parsed != nil {
			return "", parsed
		}
		return "", fmt.Errorf("netns exec failed: %w, output: %s, error: %s", err, outb.String(), errb.String())
	}
	return outb.String(), err
}

func (r *runcRunner) executeInNetworkNamespaceUsingRunc(ctx context.Context, processArgs []string, cmds []string) (string, error) {
	log.Trace().Strs("cmds", cmds).Strs("processArgs", processArgs).Msg("running commands in network namespace using runc")

	id := getNextContainerId(r.sidecar.ExecutionId, path.Base(processArgs[0]), r.sidecar.IdSuffix)
	bundle, err := r.runc.Create(ctx, "/", id)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := bundle.Remove(); err != nil {
			log.Warn().Str("id", id).Err(err).Msg("failed to remove bundle")
		}
	}()

	if err = bundle.EditSpec(
		runc.WithHostname(id),
		runc.WithAnnotations(map[string]string{"com.steadybit.sidecar": "true"}),
		runc.WithNamespaces(runc.FilterNamespaces(r.sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)),
		runc.WithCapabilities("CAP_NET_ADMIN"),
		runc.WithCopyEnviron(),
		runc.WithProcessArgs(processArgs...),
	); err != nil {
		return "", err
	}

	var outb, errb bytes.Buffer
	err = r.runc.Run(ctx, bundle, runc.IoOpts{
		Stdin:  ToReader(cmds),
		Stdout: &outb,
		Stderr: &errb,
	})
	defer func() {
		if err := r.runc.Delete(context.Background(), id, true); err != nil {
			level := zerolog.WarnLevel
			if errors.Is(err, runc.ErrContainerNotFound) {
				level = zerolog.DebugLevel
			}
			log.WithLevel(level).Str("id", id).Err(err).Msg("failed to delete container")
		}
	}()

	if err != nil {
		if parsed := ParseBatchError(processArgs, bytes.NewReader(errb.Bytes())); parsed != nil {
			return "", parsed
		}
		return "", fmt.Errorf("%s failed: %w, output: %s, error: %s", id, err, outb.String(), errb.String())
	}
	return outb.String(), err
}

func getNextContainerId(executionId uuid.UUID, tool, suffix string) string {
	return fmt.Sprintf("sb-%s-%d-%s-%s", tool, time.Now().UnixMilli(), utils.ShortenUUID(executionId), suffix)
}
