// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

//go:build !windows

package proxyfault

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustCIDR(t *testing.T, s string) net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("ParseCIDR(%q): %v", s, err)
	}
	return *n
}

func sampleOpts(t *testing.T) Opts {
	return Opts{
		ExecutionId:  "exec-abc",
		ProxyPort:    3128,
		MetricsPort:  9090,
		Mark:         0x5c,
		MaxDuration:  30 * time.Second,
		IncludeCIDRs: []net.IPNet{mustCIDR(t, "10.0.0.0/8")},
		ExcludeCIDRs: []net.IPNet{mustCIDR(t, "10.1.2.3/32")},
		Ports:        []uint16{80, 443},
		Fault: Fault{
			Latency:     250 * time.Millisecond,
			Reset:       true,
			HTTPStatus:  503,
			Probability: fptr(0.25),
			Hosts:       []string{"api.example.com", "db.internal"},
		},
	}
}

func TestStartArgs(t *testing.T) {
	got := strings.Join(sampleOpts(t).startArgs(), " ")
	for _, want := range []string{
		"--listen 0.0.0.0:3128",
		"--exec-id exec-abc",
		"--intercept-cidrs 10.0.0.0/8",
		"--intercept-ports 80,443",
		"--exclude-cidrs 10.1.2.3/32",
		"--mark 92",
		"--max-duration 30s",
		"--metrics-addr 0.0.0.0:9090",
		"--fault-latency 250ms",
		"--fault-reset",
		"--fault-http-status 503",
		"--fault-probability 0.25",
		"--fault-hosts api.example.com,db.internal",
	} {
		assert.Contains(t, got, want, "startArgs missing %q", want)
	}
	assert.NotContains(t, got, "--revert")
}

func fptr(f float64) *float64 { return &f }

// An explicit probability — including 0 (never) — must be forwarded; a nil
// probability must omit the flag so the proxy default (always) applies.
func TestStartArgs_probability(t *testing.T) {
	o := sampleOpts(t)

	o.Fault.Probability = fptr(0)
	assert.Contains(t, strings.Join(o.startArgs(), " "), "--fault-probability 0",
		"explicit 0 must be forwarded as never")

	o.Fault.Probability = fptr(1)
	assert.Contains(t, strings.Join(o.startArgs(), " "), "--fault-probability 1")

	o.Fault.Probability = nil
	assert.NotContains(t, strings.Join(o.startArgs(), " "), "--fault-probability",
		"nil must omit the flag so the proxy default applies")
}

func TestStartArgs_tlsInterceptCA(t *testing.T) {
	o := sampleOpts(t)

	// Unset: TLS is never decrypted, so the flags must be absent entirely.
	got := strings.Join(o.startArgs(), " ")
	assert.NotContains(t, got, "--tls-ca")

	assert.Nil(t, o.stdinPayload())

	o.TLSInterceptCA = &TLSInterceptCA{CertPEM: []byte("CERT-PEM"), KeyPEM: []byte("KEY-PEM")}
	got = strings.Join(o.startArgs(), " ")
	assert.Contains(t, got, "--tls-ca-stdin")
	// The key must never reach the command line.
	assert.NotContains(t, got, "CERT-PEM")
	assert.NotContains(t, got, "KEY-PEM")

	// Both halves are handed over on stdin as one PEM stream.
	payload := string(o.stdinPayload())
	assert.Contains(t, payload, "CERT-PEM")
	assert.Contains(t, payload, "KEY-PEM")

	// Revert only reconstructs interception rules, which do not depend on the CA.
	assert.NotContains(t, strings.Join(o.revertArgs(), " "), "--tls-ca")
}

func TestSnapshot_tlsInterceptRejected(t *testing.T) {
	// The rejected counter must survive the stdout round-trip, since it is the
	// signal that the CA is missing from the target's truststore.
	var c metricsCollector
	c.collectFromReader(strings.NewReader(
		`{"connections_matched":2,"connections_faulted":0,"tls_intercept_rejected":2}`+"\n"), zerolog.Nop())

	snap, ok := c.snapshot()
	require.True(t, ok)
	assert.Equal(t, int64(2), snap.TLSInterceptRejected)
	assert.Equal(t, int64(0), snap.ConnectionsFaulted)
}

func TestRevertArgs(t *testing.T) {
	got := strings.Join(sampleOpts(t).revertArgs(), " ")
	// Revert must reproduce the same chain identity (exec-id) and filter so the
	// generated delete script matches what was installed.
	for _, want := range []string{
		"--revert",
		"--exec-id exec-abc",
		"--listen 0.0.0.0:3128",
		"--intercept-cidrs 10.0.0.0/8",
		"--intercept-ports 80,443",
		"--exclude-cidrs 10.1.2.3/32",
		"--mark 92",
	} {
		assert.Contains(t, got, want, "revertArgs missing %q", want)
	}
	// Revert should not carry fault or runtime flags.
	assert.NotContains(t, got, "--fault-")
	assert.NotContains(t, got, "--max-duration")
	assert.NotContains(t, got, "--metrics-addr")
}

func TestArgs_OmitOptionalWhenUnset(t *testing.T) {
	o := Opts{ExecutionId: "e", ProxyPort: 3128, IncludeCIDRs: []net.IPNet{mustCIDR(t, "0.0.0.0/0")}, Ports: []uint16{443}}
	got := strings.Join(o.startArgs(), " ")
	for _, absent := range []string{"--exclude-cidrs", "--mark", "--max-duration", "--metrics-addr", "--fault-"} {
		assert.NotContains(t, got, absent)
	}
}

func TestMetricsCollector_ParsesLatestFromStdout(t *testing.T) {
	var c metricsCollector
	if _, ok := c.snapshot(); ok {
		t.Fatal("expected no snapshot before any line")
	}
	stream := strings.Join([]string{
		`{"connections_matched":1,"connections_aborted":0}`,
		`not-json noise line`,
		`{"connections_matched":5,"connections_aborted":2,"per_host":{"api.example.com":{"matched":5,"faulted":2}}}`,
		``,
	}, "\n")
	c.collectFromReader(strings.NewReader(stream), zerolog.Nop())

	s, ok := c.snapshot()
	if !ok {
		t.Fatal("expected a snapshot after parsing")
	}
	if s.ConnectionsMatched != 5 || s.ConnectionsAborted != 2 {
		t.Fatalf("latest snapshot = %+v", s)
	}
	if h := s.PerHost["api.example.com"]; h.Matched != 5 || h.Faulted != 2 {
		t.Fatalf("per-host = %+v", h)
	}
	if hosts := s.SortedHosts(); len(hosts) != 1 || hosts[0] != "api.example.com" {
		t.Fatalf("sorted hosts = %v", hosts)
	}
}

func TestStartArgs_MetricsStdoutAndNoFlush(t *testing.T) {
	o := sampleOpts(t)
	o.MetricsStdoutInterval = 2 * time.Second
	o.NoFlush = true
	args := strings.Join(o.startArgs(), " ")
	if !strings.Contains(args, "--metrics-stdout-interval 2s") {
		t.Errorf("missing metrics-stdout-interval flag: %s", args)
	}
	if !strings.Contains(args, "--no-flush") {
		t.Errorf("missing no-flush flag: %s", args)
	}
}
