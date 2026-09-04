// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

//go:build !windows

// Package proxyfault launches and manages the steadybit transparent-proxy
// binary inside a target's network namespace, for injecting network faults
// against a target's dependencies (latency, connection reset, HTTP status
// injection). It mirrors the dnsinject package: the long-running proxy is run in
// a runc sidecar joined to the target netns (or via `ip netns exec` for named
// namespaces); the proxy self-manages its iptables interception and tears it
// down on stop. Revert provides an out-of-band teardown for the case where the
// proxy process was killed before its own cleanup ran.
//
// The proxy is run as an external binary (located via
// STEADYBIT_EXTENSION_TRANSPARENT_PROXY_PATH), so this package has no build
// dependency on the transparent-proxy module.
package proxyfault

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/network/nsrunner"
	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
)

var proxyPath = utils.LocateExecutable("transparent-proxy", "STEADYBIT_EXTENSION_TRANSPARENT_PROXY_PATH")
var ipPath = utils.LocateExecutable("ip", "STEADYBIT_EXTENSION_IP_PATH")

// Fault is the single fault the proxy injects on matching connections.
type Fault struct {
	Latency    time.Duration
	Reset      bool
	HTTPStatus int
	// HTTPBody / HTTPHeaders customize the synthesized HTTP response that
	// accompanies HTTPStatus (cleartext HTTP only). Empty keeps the proxy's
	// defaults.
	HTTPBody    string
	HTTPHeaders map[string]string
	// Probability in [0,1] gates the fault per connection. nil (unset) lets the
	// proxy default apply (always); an explicit value — including 0 (never) — is
	// forwarded as-is, so callers can express "never" as well as "always".
	Probability *float64
	Hosts       []string
}

// TLSInterceptCA carries the certificate authority the proxy uses to mint
// per-SNI certificates, which is what lets an HTTPStatus fault reach an HTTPS
// dependency instead of only a cleartext one.
//
// The CA belongs to the customer: they generate it, choose how long it lives,
// and install it in the truststores of the workloads they want to fault. This
// package only relays it.
//
// It is carried as PEM rather than as file paths on purpose. The runc backend
// runs the proxy in a bundle whose rootfs is an overlay of the extension's "/",
// and an overlay does not carry the extension's submounts — so a CA mounted
// from a Kubernetes Secret is simply not visible by path inside the sidecar
// (verified: the mount point appears as an empty directory). Passing the PEM
// over the process's stdin works identically for both backends, keeps the key
// off the command line, and never writes it to a disk the target could reach.
type TLSInterceptCA struct {
	CertPEM []byte
	KeyPEM  []byte
	// LeafValidity, when >0, overrides how long the per-SNI certificates the
	// proxy mints stay valid. Always clamped to the CA's own expiry by the
	// proxy. Zero keeps the proxy's built-in default.
	LeafValidity time.Duration
}

// pemStream is what the proxy reads from stdin: one PEM stream carrying both
// halves. Order is irrelevant to the parser on the other side.
func (c *TLSInterceptCA) pemStream() []byte {
	out := make([]byte, 0, len(c.CertPEM)+len(c.KeyPEM)+1)
	out = append(out, c.CertPEM...)
	if len(c.CertPEM) > 0 && c.CertPEM[len(c.CertPEM)-1] != '\n' {
		out = append(out, '\n')
	}
	return append(out, c.KeyPEM...)
}

// Opts configures interception and the injected fault.
type Opts struct {
	ExecutionId string
	ProxyPort   uint16
	MetricsPort uint16 // 0 disables the HTTP metrics endpoint
	// MetricsStdoutInterval, when >0, makes the proxy print JSON metrics
	// snapshots to stdout at this cadence. This is how the extension reads
	// intercept statistics: the proxy runs in the target's netns (a container's,
	// for container attacks), so its HTTP endpoint is unreachable, but its stdout
	// is always captured here.
	MetricsStdoutInterval time.Duration
	Mark                  uint32 // 0 uses the proxy's default loop-protection mark
	MaxDuration           time.Duration
	// NoFlush leaves already-established connections alone (only new connections
	// are intercepted). Default (false) resets warm connection pools so the fault
	// takes effect immediately.
	NoFlush      bool
	IncludeCIDRs []net.IPNet
	ExcludeCIDRs []net.IPNet
	Ports        []uint16
	Fault        Fault
	// TLSInterceptCA, when set, lets an HTTPStatus fault also apply to HTTPS
	// connections. Nil (the default) means TLS is never decrypted and HTTPS is
	// spliced through untouched.
	TLSInterceptCA *TLSInterceptCA
}

// Proxy is a running transparent-proxy instance.
type Proxy interface {
	Start() error
	Stop() error
	Exited() (bool, error)
	// Metrics returns the latest interception statistics scraped from the proxy's
	// stdout, and whether any snapshot has been received yet. Requires the proxy
	// to have been started with a MetricsStdoutInterval.
	Metrics() (Snapshot, bool)
}

// HostStat is the per-dependency-hostname breakdown in a Snapshot.
type HostStat struct {
	Matched int64 `json:"matched"`
	Faulted int64 `json:"faulted"`
}

// Snapshot mirrors the transparent-proxy metrics JSON emitted on stdout.
type Snapshot struct {
	ConnectionsMatched    int64 `json:"connections_matched"`
	ConnectionsActive     int64 `json:"connections_active"`
	ConnectionsProxied    int64 `json:"connections_proxied"`
	ConnectionsAborted    int64 `json:"connections_aborted"`
	ConnectionsDropped    int64 `json:"connections_dropped"`
	ConnectionsFaulted    int64 `json:"connections_faulted"`
	LatencyApplied        int64 `json:"latency_applied"`
	HTTPResponsesInjected int64 `json:"http_responses_injected"`
	// TLSInterceptRejected counts HTTPS connections on which the client refused
	// the minted certificate. A non-zero value is the canonical "the CA is not in
	// the target's truststore (or the client pins certificates)" signal — the
	// fault never applied, so these are deliberately not counted as faulted.
	TLSInterceptRejected int64               `json:"tls_intercept_rejected"`
	UpstreamErrors       int64               `json:"upstream_errors"`
	BytesToUpstream      int64               `json:"bytes_to_upstream"`
	BytesToClient        int64               `json:"bytes_to_client"`
	PerHost              map[string]HostStat `json:"per_host,omitempty"`
}

// SortedHosts returns the per-host keys in a stable order.
func (s Snapshot) SortedHosts() []string {
	hosts := make([]string, 0, len(s.PerHost))
	for h := range s.PerHost {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	return hosts
}

// metricsCollector keeps the most recent metrics snapshot scraped from stdout.
type metricsCollector struct {
	mu     sync.Mutex
	latest Snapshot
	got    bool
}

func (c *metricsCollector) store(s Snapshot) {
	c.mu.Lock()
	c.latest, c.got = s, true
	c.mu.Unlock()
}

func (c *metricsCollector) snapshot() (Snapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.latest, c.got
}

// collectFromReader scans the proxy's stdout line by line. Each line is a JSON
// metrics snapshot (the proxy sends logs to stderr, keeping stdout clean); a
// line that does not decode is forwarded to the debug log, matching the old
// behaviour where stdout was logged.
func (c *metricsCollector) collectFromReader(r io.Reader, logger zerolog.Logger) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var snap Snapshot
		if err := json.Unmarshal(line, &snap); err == nil {
			c.store(snap)
			continue
		}
		logger.Debug().Msg(strings.TrimRight(string(line), "\n"))
	}
	// The scanner stops on the first error (e.g. a line over the buffer cap). If
	// the process is still running it would then block writing to an unread pipe,
	// and cmd.Wait would never return — so keep draining until EOF. Metrics past
	// the offending line are lost, but the process still exits cleanly.
	if err := sc.Err(); err != nil {
		logger.Debug().Err(err).Msg("transparent-proxy stdout scan stopped; draining remainder")
		_, _ = io.Copy(io.Discard, r)
	}
}

// NewProcess builds (but does not start) a proxy for the target's netns.
func NewProcess(ctx context.Context, r ociruntime.OciRuntime, targetProcess ociruntime.LinuxProcessInfo, id string, opts Opts) (Proxy, error) {
	ociruntime.RefreshNamespaces(ctx, targetProcess.Namespaces, specs.NetworkNamespace)
	if ociruntime.HasNamedNetworkNamespace(targetProcess.Namespaces...) {
		return newNetnsProcess(targetProcess, opts)
	}
	return newRuncProcess(ctx, r, targetProcess, id, opts)
}

// Revert removes the interception out of band by running the proxy's --revert.
// The runner is supplied by the caller (runc or process), matching how the
// proxy was launched. It is idempotent.
func Revert(ctx context.Context, runner nsrunner.Runner, opts Opts) error {
	_, err := runner.Run(ctx, append([]string{proxyPath}, opts.revertArgs()...), nil)
	return err
}

// startArgs are the flags to launch the self-managed proxy.
func (o Opts) startArgs() []string {
	args := o.interceptArgs()
	if o.MaxDuration > 0 {
		args = append(args, "--max-duration", o.MaxDuration.String())
	}
	if o.MetricsPort != 0 {
		args = append(args, "--metrics-addr", fmt.Sprintf("0.0.0.0:%d", o.MetricsPort))
	}
	if o.MetricsStdoutInterval > 0 {
		args = append(args, "--metrics-stdout-interval", o.MetricsStdoutInterval.String())
	}
	if o.Fault.Latency > 0 {
		args = append(args, "--fault-latency", o.Fault.Latency.String())
	}
	if o.Fault.Reset {
		args = append(args, "--fault-reset")
	}
	if o.Fault.HTTPStatus != 0 {
		args = append(args, "--fault-http-status", strconv.Itoa(o.Fault.HTTPStatus))
	}
	if o.Fault.HTTPBody != "" {
		args = append(args, "--fault-http-body", o.Fault.HTTPBody)
	}
	for _, k := range sortedKeys(o.Fault.HTTPHeaders) {
		args = append(args, "--fault-http-header", fmt.Sprintf("%s: %s", k, o.Fault.HTTPHeaders[k]))
	}
	if o.Fault.Probability != nil {
		args = append(args, "--fault-probability", strconv.FormatFloat(*o.Fault.Probability, 'f', -1, 64))
	}
	if len(o.Fault.Hosts) > 0 {
		args = append(args, "--fault-hosts", strings.Join(o.Fault.Hosts, ","))
	}
	// Start-only: --revert reconstructs the interception rules, which do not
	// depend on the CA. The PEM itself goes over stdin, never argv.
	//
	// Gated on the same condition as the payload: telling the proxy to read its
	// CA from stdin while writing nothing there would leave it reading an empty
	// stream, which is a far more confusing failure than not enabling it.
	if _, ok := o.interceptCAPayload(); ok {
		args = append(args, "--tls-ca-stdin")
		if o.TLSInterceptCA.LeafValidity > 0 {
			args = append(args, "--tls-leaf-validity", o.TLSInterceptCA.LeafValidity.String())
		}
	}
	return args
}

// interceptCAPayload returns the PEM to hand the proxy on stdin, and whether a
// usable CA was configured at all. A half-populated CA counts as unusable: the
// proxy needs both halves, and silently sending one produces a startup failure
// that reads like a certificate problem.
func (o Opts) interceptCAPayload() ([]byte, bool) {
	ca := o.TLSInterceptCA
	if ca == nil || len(ca.CertPEM) == 0 || len(ca.KeyPEM) == 0 {
		return nil, false
	}
	return ca.pemStream(), true
}

// stdinPayload is what gets written to the proxy's stdin at start: the
// interception CA, or nothing when HTTPS is not being decrypted.
func (o Opts) stdinPayload() []byte {
	payload, _ := o.interceptCAPayload()
	return payload
}

// sortedKeys returns a map's keys in a deterministic order so the built argv is
// stable (which keeps revert-arg matching and tests predictable).
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// revertArgs are the flags to reconstruct and remove the same interception.
func (o Opts) revertArgs() []string {
	return append([]string{"--revert"}, o.interceptArgs()...)
}

// interceptArgs are the flags shared by start and revert (they must produce the
// same chain names and filter).
func (o Opts) interceptArgs() []string {
	args := []string{
		"--listen", fmt.Sprintf("0.0.0.0:%d", o.ProxyPort),
		"--exec-id", o.ExecutionId,
		"--intercept-cidrs", joinCIDRs(o.IncludeCIDRs),
		"--intercept-ports", joinPorts(o.Ports),
	}
	if len(o.ExcludeCIDRs) > 0 {
		args = append(args, "--exclude-cidrs", joinCIDRs(o.ExcludeCIDRs))
	}
	if o.Mark != 0 {
		args = append(args, "--mark", strconv.FormatUint(uint64(o.Mark), 10))
	}
	// NoFlush is part of the shared interception args (not just start), so the
	// out-of-band --revert reconstructs the same flush decision and does not try
	// to delete a filter chain that was never installed.
	if o.NoFlush {
		args = append(args, "--no-flush")
	}
	return args
}

func joinCIDRs(nets []net.IPNet) string {
	parts := make([]string, len(nets))
	for i, n := range nets {
		parts[i] = n.String()
	}
	return strings.Join(parts, ",")
}

func joinPorts(ports []uint16) string {
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = strconv.Itoa(int(p))
	}
	return strings.Join(parts, ",")
}

func (o Opts) String() string {
	return "transparent-proxy " + strings.Join(o.startArgs(), " ")
}

// processBase holds the exit channel and start/monitor helper shared by both
// backends.
type processBase struct {
	started atomic.Bool
	done    chan struct{} // closed exactly once when the process exits
	exitErr error         // set before done is closed; read only after
	metrics metricsCollector
}

func newProcessBase() processBase {
	return processBase{done: make(chan struct{})}
}

// Metrics returns the latest snapshot scraped from the proxy's stdout.
func (b *processBase) Metrics() (Snapshot, bool) { return b.metrics.snapshot() }

// Exited reports whether the process has exited, without consuming any state —
// it is safe to call repeatedly and after Stop.
func (b *processBase) Exited() (bool, error) {
	select {
	case <-b.done:
		return true, b.exitErr
	default:
		return false, nil
	}
}

// waitExited blocks until the process has exited. It is safe to call more than
// once (a closed channel keeps returning) and safe when the process was never
// started (returns immediately rather than blocking forever).
func (b *processBase) waitExited() {
	if !b.started.Load() {
		return
	}
	<-b.done
}

// startAndMonitor starts cmd and watches it. stdin, when non-empty, is written
// to the process and the pipe then closed — this is how the interception CA is
// handed over without touching argv or the filesystem.
func (b *processBase) startAndMonitor(cmd *exec.Cmd, logId string, stdin []byte) error {
	logger := log.With().Str("id", logId).Logger()
	var stdinPipe io.WriteCloser
	if len(stdin) > 0 {
		w, err := cmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("failed to pipe transparent-proxy stdin: %w", err)
		}
		stdinPipe = w
	}
	// stdout carries the JSON metrics stream (scraped for statistics); stderr
	// carries the proxy's structured logs.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		// Unreachable in practice (StdoutPipe only fails when cmd.Stdout is
		// already set, and cmd is always freshly built), but returning here
		// without closing the stdin pipe would strand its write end.
		if stdinPipe != nil {
			_ = stdinPipe.Close()
		}
		return fmt.Errorf("failed to pipe transparent-proxy stdout: %w", err)
	}
	cmd.Stderr = &logWriter{logger: logger}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start transparent-proxy: %w", err)
	}
	b.started.Store(true)
	// Only after a successful Start. On the error paths above os/exec never
	// closes the child end of the pipe, so writing there would leak a descriptor
	// and leave a copy of the CA key resident for a process that never ran.
	if stdinPipe != nil {
		go func() {
			defer func() { _ = stdinPipe.Close() }()
			if _, werr := stdinPipe.Write(stdin); werr != nil {
				logger.Warn().Err(werr).Msg("failed to write CA to transparent-proxy stdin")
			}
		}()
	}
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		b.metrics.collectFromReader(stdout, logger)
	}()
	go func() {
		<-scanDone // drain stdout before reaping, so the final snapshot is not lost
		b.exitErr = cmd.Wait()
		close(b.done)
	}()
	return nil
}

type logWriter struct {
	logger zerolog.Logger
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.logger.Debug().Msg(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

// --- process (named netns) backend ---

type netnsProxy struct {
	processBase
	cmd  *exec.Cmd
	opts Opts
}

func newNetnsProcess(targetProcess ociruntime.LinuxProcessInfo, opts Opts) (Proxy, error) {
	netns := ""
	for _, ns := range targetProcess.Namespaces {
		if ns.Type == specs.NetworkNamespace {
			netns = ociruntime.TrimNameNetworkNamespacePrefix(ns.Path)
			break
		}
	}
	if netns == "" {
		return nil, errors.New("no network namespace found")
	}
	cmdArgs := append([]string{"netns", "exec", netns, proxyPath}, opts.startArgs()...)
	cmd := utils.RootCommandContext(context.Background(), ipPath, cmdArgs...)
	return &netnsProxy{processBase: newProcessBase(), cmd: cmd, opts: opts}, nil
}

func (p *netnsProxy) Start() error {
	log.Trace().Str("cmd", p.opts.String()).Msg("starting transparent-proxy via ip netns exec")
	return p.startAndMonitor(p.cmd, "transparent-proxy", p.opts.stdinPayload())
}

func (p *netnsProxy) Stop() error {
	if !p.started.Load() {
		return nil
	}
	// Already exited: nothing to signal, and the pid may have been reaped and
	// recycled — signalling it could hit an unrelated process.
	if exited, _ := p.Exited(); exited {
		return nil
	}
	pid := p.cmd.Process.Pid
	ctx := context.Background()
	// SIGTERM lets the in-proxy Guard tear down its interception cleanly.
	if err := utils.RootCommandContext(ctx, "kill", "-s", "SIGTERM", strconv.Itoa(pid)).Run(); err != nil {
		log.Warn().Err(err).Msg("failed to SIGTERM transparent-proxy")
	}
	killTimer := time.AfterFunc(15*time.Second, func() {
		// Escalate through the same root helper as SIGTERM: an in-process
		// Signal would return EPERM if the proxy runs privileged while this
		// extension does not, leaving a hung proxy alive.
		if err := utils.RootCommandContext(ctx, "kill", "-s", "SIGKILL", strconv.Itoa(pid)).Run(); err != nil {
			log.Warn().Err(err).Msg("failed to SIGKILL transparent-proxy")
		}
	})
	p.waitExited()
	killTimer.Stop()
	return nil
}

// --- runc sidecar backend ---

type runcProxy struct {
	processBase
	bundle ociruntime.ContainerBundle
	runc   ociruntime.OciRuntime
	opts   Opts
}

// proxyContainerSeq disambiguates sidecar container ids created within the same
// millisecond, mirroring nsrunner's counter — a timestamp alone is not unique.
var proxyContainerSeq atomic.Uint64

func newRuncProcess(ctx context.Context, r ociruntime.OciRuntime, targetProcess ociruntime.LinuxProcessInfo, id string, opts Opts) (Proxy, error) {
	containerId := fmt.Sprintf("sb-transparent-proxy-%d-%d-%s", time.Now().UnixMilli(), proxyContainerSeq.Add(1), id)
	bundle, err := r.Create(ctx, "/", containerId)
	if err != nil {
		return nil, fmt.Errorf("failed to create bundle: %w", err)
	}

	processArgs := append([]string{proxyPath}, opts.startArgs()...)
	if err := bundle.EditSpec(
		ociruntime.WithHostname(containerId),
		ociruntime.WithAnnotations(map[string]string{"com.steadybit.sidecar": "true"}),
		ociruntime.WithNamespaces(ociruntime.FilterNamespaces(targetProcess.Namespaces, specs.NetworkNamespace)),
		ociruntime.WithCapabilities("CAP_NET_ADMIN", "CAP_NET_RAW"),
		ociruntime.WithCopyEnviron(),
		// iptables in the sidecar needs a writable lock location.
		ociruntime.WithEnv("XTABLES_LOCKFILE=/tmp/xtables.lock"),
		ociruntime.WithMountIfNotPresent(specs.Mount{Destination: "/tmp", Type: "tmpfs", Source: "tmpfs", Options: []string{"nosuid", "nodev", "noexec"}}),
		ociruntime.WithProcessArgs(processArgs...),
	); err != nil {
		_ = bundle.Remove()
		return nil, fmt.Errorf("failed to configure bundle: %w", err)
	}

	return &runcProxy{processBase: newProcessBase(), bundle: bundle, runc: r, opts: opts}, nil
}

func (d *runcProxy) Start() error {
	log.Trace().Str("containerId", d.bundle.ContainerId()).Str("cmd", d.opts.String()).Msg("starting transparent-proxy via runc sidecar")
	cmd, err := d.runc.RunCommand(context.Background(), d.bundle)
	if err != nil {
		return fmt.Errorf("failed to create run command: %w", err)
	}
	return d.startAndMonitor(cmd, d.bundle.ContainerId(), d.opts.stdinPayload())
}

func (d *runcProxy) Stop() error {
	ctx := context.Background()
	// If Start never succeeded, no monitor goroutine will ever close done, so
	// waiting on it would block forever. Still remove the bundle we created.
	if !d.started.Load() {
		if err := d.bundle.Remove(); err != nil {
			log.Warn().Str("id", d.bundle.ContainerId()).Err(err).Msg("failed to remove bundle")
		}
		return nil
	}
	// SIGTERM lets the in-proxy Guard tear down interception cleanly.
	if err := d.runc.Kill(ctx, d.bundle.ContainerId(), syscall.SIGTERM); err != nil {
		log.Warn().Str("id", d.bundle.ContainerId()).Err(err).Msg("failed to SIGTERM transparent-proxy")
	}
	killTimer := time.AfterFunc(15*time.Second, func() {
		if err := d.runc.Kill(ctx, d.bundle.ContainerId(), syscall.SIGKILL); err != nil {
			log.Warn().Str("id", d.bundle.ContainerId()).Err(err).Msg("failed to SIGKILL transparent-proxy")
		}
	})
	d.waitExited()
	killTimer.Stop()

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
