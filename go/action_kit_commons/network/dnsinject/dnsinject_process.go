// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

//go:build !windows

package dnsinject

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
)

// dnsInjectProcess runs dns-inject via "ip netns exec" for named network namespaces
type dnsInjectProcess struct {
	processBase
	cmd  *exec.Cmd
	opts Opts
}

func newNetnsProcess(targetProcess ociruntime.LinuxProcessInfo, opts Opts) (DNSInject, error) {
	netns := ""
	for _, ns := range targetProcess.Namespaces {
		if ns.Type == specs.NetworkNamespace {
			netns = ociruntime.TrimNameNetworkNamespacePrefix(ns.Path)
			break
		}
	}
	if netns == "" {
		return nil, fmt.Errorf("no network namespace found")
	}

	ipPath := utils.LocateExecutable("ip", "STEADYBIT_EXTENSION_IP_PATH")
	cmdArgs := append([]string{"netns", "exec", netns, dnsInjectPath}, opts.toArgs()...)
	cmd := utils.RootCommandContext(context.Background(), ipPath, cmdArgs...)

	return &dnsInjectProcess{
		processBase: processBase{exited: make(chan error, 1)},
		cmd:         cmd,
		opts:        opts,
	}, nil
}

func (p *dnsInjectProcess) Start() error {
	log.Trace().Str("cmd", p.opts.String()).Msg("starting dns-inject via ip netns exec")
	return p.startAndMonitor(p.cmd, "dns-inject")
}

func (p *dnsInjectProcess) Stop() error {
	log.Trace().Msg("stopping dns-inject")

	if p.cmd.Process == nil {
		return nil
	}

	pid := p.cmd.Process.Pid
	ctx := context.Background()
	if err := utils.RootCommandContext(ctx, "kill", "-s", "SIGINT", strconv.Itoa(pid)).Run(); err != nil {
		log.Warn().Err(err).Msg("failed to send SIGINT to dns-inject")
	}

	termTimer := time.AfterFunc(10*time.Second, func() {
		if err := utils.RootCommandContext(ctx, "kill", "-s", "SIGTERM", strconv.Itoa(pid)).Run(); err != nil {
			log.Warn().Err(err).Msg("failed to send SIGTERM to dns-inject")
		}
	})

	killTimer := time.AfterFunc(15*time.Second, func() {
		if err := p.cmd.Process.Signal(syscall.SIGKILL); err != nil {
			log.Warn().Err(err).Msg("failed to send SIGKILL to dns-inject")
		}
	})

	<-p.exited
	termTimer.Stop()
	killTimer.Stop()
	return nil
}
