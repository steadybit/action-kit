// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

//go:build !windows

package dnsinject

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
)

type ErrorType string

const (
	ErrorTypeNXDOMAIN ErrorType = "NXDOMAIN"
	ErrorTypeSERVFAIL ErrorType = "SERVFAIL"
	ErrorTypeTimeout  ErrorType = "TIMEOUT"
)

type Opts struct {
	ErrorTypes []ErrorType
	CIDRs      []net.IPNet
	PortRange  network.PortRange
	Interfaces []string
}

type Metrics struct {
	Seen             uint64 `json:"seen"`
	Ipv4             uint64 `json:"ipv4"`
	Ipv6             uint64 `json:"ipv6"`
	DnsMatched       uint64 `json:"dns_matched"`
	Injected         uint64 `json:"injected"`
	InjectedNxdomain uint64 `json:"injected_nxdomain"`
	InjectedServfail uint64 `json:"injected_servfail"`
	InjectedTimeout  uint64 `json:"injected_timeout"`
}

type DNSInject interface {
	Start() error
	Stop() error
	Exited() (bool, error)
	Metrics() (*Metrics, error)
}

type SidecarOpts struct {
	TargetProcess ociruntime.LinuxProcessInfo
	Id            string
}

var dnsInjectPath = utils.LocateExecutable("dns-inject", "STEADYBIT_EXTENSION_DNS_INJECT_PATH")

func NewProcess(ctx context.Context, r ociruntime.OciRuntime, sidecar SidecarOpts, opts Opts) (DNSInject, error) {
	ociruntime.RefreshNamespaces(ctx, sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)

	if ociruntime.HasNamedNetworkNamespace(sidecar.TargetProcess.Namespaces...) {
		return newNetnsProcess(sidecar, opts)
	}
	return newRuncProcess(ctx, r, sidecar, opts)
}

func (o *Opts) toArgs() []string {
	var args []string

	for _, t := range o.ErrorTypes {
		args = append(args, "--error-type", string(t))
	}

	for _, cidr := range o.CIDRs {
		args = append(args, "--cidr", cidr.String())
	}

	args = append(args, "--port", o.PortRange.String())

	for _, iface := range o.Interfaces {
		args = append(args, "--interface", iface)
	}

	return args
}

func (o *Opts) String() string {
	return fmt.Sprintf("dns-inject %s", strings.Join(o.toArgs(), " "))
}

// processBase contains the shared state and methods for both implementations
type processBase struct {
	exited  chan error
	metrics metricsCollector
}

func (b *processBase) Exited() (bool, error) {
	select {
	case err := <-b.exited:
		b.exited <- err // put back for subsequent reaads
		return true, err
	default:
		return false, nil
	}
}

func (b *processBase) Metrics() (*Metrics, error) {
	return b.metrics.latest()
}

// startAndMonitor sets up stdout/stderr on the command, starts it, and launches
// background goroutines for metrics collection and exit monitoring.
func (b *processBase) startAndMonitor(cmd *exec.Cmd, logId string) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	cmd.Stderr = &logWriter{logger: log.With().Str("id", logId).Logger()}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dns-inject: %w", err)
	}

	go b.metrics.collectFromReader(stdout)
	go func() {
		b.exited <- cmd.Wait()
	}()

	return nil
}

// metricsCollector parses JSON metrics lines from dns-inject stdout
type metricsCollector struct {
	mu   sync.Mutex
	last *Metrics
}

func (mc *metricsCollector) collectFromReader(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var m Metrics
		if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
			log.Trace().Err(err).Str("line", scanner.Text()).Msg("failed to parse metrics line")
			continue
		}
		mc.mu.Lock()
		mc.last = &m
		mc.mu.Unlock()
	}
	if err := scanner.Err(); err != nil {
		log.Warn().Err(err).Msg("metrics reader error")
	}
}

func (mc *metricsCollector) latest() (*Metrics, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if mc.last == nil {
		return nil, fmt.Errorf("no metrics available yet")
	}
	m := *mc.last
	return &m, nil
}

type logWriter struct {
	logger zerolog.Logger
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.logger.Debug().Msg(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}
