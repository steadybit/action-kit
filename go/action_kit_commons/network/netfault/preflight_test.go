// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package netfault

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRootQdiscKinds(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want map[string]string
	}{
		{
			name: "empty output",
			out:  "",
			want: map[string]string{},
		},
		{
			name: "GKE COS multi-queue eth0",
			out: `qdisc mq 8002: dev eth0 root
qdisc fq_codel 0: dev eth0 parent 8002:4 limit 10240p flows 1024
qdisc fq_codel 0: dev eth0 parent 8002:3 limit 10240p flows 1024
qdisc fq_codel 0: dev eth0 parent 8002:2 limit 10240p flows 1024
qdisc fq_codel 0: dev eth0 parent 8002:1 limit 10240p flows 1024`,
			want: map[string]string{"eth0": "mq"},
		},
		{
			name: "modern single-queue with fq_codel default",
			out:  `qdisc fq_codel 0: dev eth0 root refcnt 2 limit 10240p flows 1024`,
			want: map[string]string{"eth0": "fq_codel"},
		},
		{
			name: "old single-queue with pfifo_fast",
			out:  `qdisc pfifo_fast 0: dev eth0 root refcnt 2 bands 3 priomap  1 2 2 2 1 2 0 0 1 1 1 1 1 1 1 1`,
			want: map[string]string{"eth0": "pfifo_fast"},
		},
		{
			name: "veth with noqueue",
			out:  `qdisc noqueue 0: dev veth0 root refcnt 2`,
			want: map[string]string{"veth0": "noqueue"},
		},
		{
			name: "user-installed htb",
			out:  `qdisc htb 1: dev eth0 root refcnt 2 r2q 10 default 0x30 direct_packets_stat 0 direct_qlen 1000`,
			want: map[string]string{"eth0": "htb"},
		},
		{
			name: "user-installed cake",
			out:  `qdisc cake 8001: dev eth0 root refcnt 2 bandwidth 1Gbit diffserv3 triple-isolate nonat nowash no-ack-filter split-gso rtt 100ms raw overhead 0`,
			want: map[string]string{"eth0": "cake"},
		},
		{
			name: "fq default (BBR)",
			out:  `qdisc fq 8001: dev eth0 root refcnt 2 limit 10000p flow_limit 100p buckets 1024`,
			want: map[string]string{"eth0": "fq"},
		},
		{
			name: "ingress only — not root",
			out:  `qdisc clsact ffff: dev eth0 parent ffff:fff1`,
			want: map[string]string{},
		},
		{
			name: "root + ingress companion",
			out: `qdisc fq_codel 0: dev eth0 root refcnt 2 limit 10240p flows 1024
qdisc clsact ffff: dev eth0 parent ffff:fff1`,
			want: map[string]string{"eth0": "fq_codel"},
		},
		{
			name: "multiple interfaces, mixed kinds",
			out: `qdisc mq 8002: dev eth0 root
qdisc htb 1: dev eth1 root refcnt 2 r2q 10 default 0x30
qdisc noqueue 0: dev lo root refcnt 2`,
			want: map[string]string{"eth0": "mq", "eth1": "htb", "lo": "noqueue"},
		},
		{
			name: "leading whitespace tolerated",
			out:  "   qdisc mq 8002: dev eth0 root",
			want: map[string]string{"eth0": "mq"},
		},
		{
			name: "garbage line ignored",
			out: `Cannot find device "eth42"
qdisc mq 0: dev eth0 root`,
			want: map[string]string{"eth0": "mq"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseRootQdiscKinds(tt.out))
		})
	}
}

// fakeRunner is a CommandRunner that returns canned stdout for any command,
// optionally erroring. Used by the netfault, preflight and Apply tests.
type fakeRunner struct {
	netNsId string
	stdout  string
	err     error
	calls   []recordedCall
}

type recordedCall struct {
	args []string
	cmds []string
}

func (f *fakeRunner) run(_ context.Context, args []string, cmds []string) (string, error) {
	f.calls = append(f.calls, recordedCall{args: args, cmds: cmds})
	return f.stdout, f.err
}

func (f *fakeRunner) id() string {
	if f.netNsId == "" {
		return "testns"
	}
	return f.netNsId
}

func TestPreflightWarnings(t *testing.T) {
	tests := []struct {
		name       string
		interfaces []string
		tcOutput   string
		runErr     error
		wantCount  int
		wantSubstr string
	}{
		{
			name:       "no interfaces",
			interfaces: nil,
			wantCount:  0,
		},
		{
			name:       "mq (GKE COS) — no warning",
			interfaces: []string{"eth0"},
			tcOutput:   `qdisc mq 8002: dev eth0 root`,
			wantCount:  0,
		},
		{
			name:       "fq_codel — no warning",
			interfaces: []string{"eth0"},
			tcOutput:   `qdisc fq_codel 0: dev eth0 root refcnt 2 limit 10240p flows 1024`,
			wantCount:  0,
		},
		{
			name:       "pfifo_fast — no warning",
			interfaces: []string{"eth0"},
			tcOutput:   `qdisc pfifo_fast 0: dev eth0 root refcnt 2 bands 3`,
			wantCount:  0,
		},
		{
			name:       "noqueue — no warning",
			interfaces: []string{"veth0"},
			tcOutput:   `qdisc noqueue 0: dev veth0 root refcnt 2`,
			wantCount:  0,
		},
		{
			name:       "fq — no warning",
			interfaces: []string{"eth0"},
			tcOutput:   `qdisc fq 8001: dev eth0 root refcnt 2 limit 10000p`,
			wantCount:  0,
		},
		{
			name:       "htb — warning",
			interfaces: []string{"eth0"},
			tcOutput:   `qdisc htb 1: dev eth0 root refcnt 2 r2q 10 default 0x30`,
			wantCount:  1,
			wantSubstr: `"htb"`,
		},
		{
			name:       "cake — warning",
			interfaces: []string{"eth0"},
			tcOutput:   `qdisc cake 8001: dev eth0 root refcnt 2 bandwidth 1Gbit`,
			wantCount:  1,
			wantSubstr: `"cake"`,
		},
		{
			name:       "multi-interface: only the user-installed one warns",
			interfaces: []string{"eth0", "eth1"},
			tcOutput: `qdisc mq 8002: dev eth0 root
qdisc htb 1: dev eth1 root`,
			wantCount:  1,
			wantSubstr: `"eth1"`,
		},
		{
			name:       "tc failure — no warning, error swallowed",
			interfaces: []string{"eth0"},
			runErr:     errors.New("tc not found"),
			wantCount:  0,
		},
		{
			name:       "interface has no root qdisc — no warning",
			interfaces: []string{"eth0"},
			tcOutput:   "",
			wantCount:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{stdout: tt.tcOutput, err: tt.runErr}
			got := preflightWarnings(context.Background(), r, tt.interfaces)
			assert.Len(t, got, tt.wantCount)
			if tt.wantCount > 0 && tt.wantSubstr != "" {
				assert.Contains(t, got[0], tt.wantSubstr)
			}
		})
	}
}
