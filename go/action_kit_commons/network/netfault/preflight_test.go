// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package netfault

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func (f *fakeRunner) netNsPath() string {
	return ""
}

func TestPreflightCheck(t *testing.T) {
	tests := []struct {
		name       string
		interfaces []string
		tcOutput   string
		runErr     error
		wantErr    bool
		wantIfc    string
		wantKind   string
	}{
		{
			name:       "no interfaces — ok",
			interfaces: nil,
		},
		{
			name:       "mq (GKE COS) — ok",
			interfaces: []string{"eth0"},
			tcOutput:   `qdisc mq 8002: dev eth0 root`,
		},
		{
			name:       "noqueue — ok",
			interfaces: []string{"veth0"},
			tcOutput:   `qdisc noqueue 0: dev veth0 root refcnt 2`,
		},
		{
			name:       "fq_codel — ok",
			interfaces: []string{"eth0"},
			tcOutput:   `qdisc fq_codel 0: dev eth0 root refcnt 2 limit 10240p`,
		},
		{
			name:       "interface has no root qdisc — ok",
			interfaces: []string{"eth0"},
			tcOutput:   "",
		},
		{
			name:       "tc failure — skipped, no error",
			interfaces: []string{"eth0"},
			runErr:     errors.New("tc not found"),
		},
		{
			name:       "user-installed htb — refused",
			interfaces: []string{"eth0"},
			tcOutput:   `qdisc htb 1: dev eth0 root refcnt 2 default 0x30`,
			wantErr:    true,
			wantIfc:    "eth0",
			wantKind:   "htb",
		},
		{
			name:       "user-installed cake — refused",
			interfaces: []string{"eth0"},
			tcOutput:   `qdisc cake 8001: dev eth0 root bandwidth 1Gbit`,
			wantErr:    true,
			wantIfc:    "eth0",
			wantKind:   "cake",
		},
		{
			name:       "multi-interface: only the user-installed one is refused",
			interfaces: []string{"eth0", "eth1"},
			tcOutput:   "qdisc mq 8002: dev eth0 root\nqdisc htb 1: dev eth1 root",
			wantErr:    true,
			wantIfc:    "eth1",
			wantKind:   "htb",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Unique netNsId per subtest so activeNetfault state from other
			// tests does not leak in and short-circuit the check.
			r := &fakeRunner{netNsId: tt.name, stdout: tt.tcOutput, err: tt.runErr}
			err := PreflightCheck(context.Background(), r, &DelayOpts{Interfaces: tt.interfaces})
			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}
			var e *ErrUserRootQdisc
			require.ErrorAs(t, err, &e)
			assert.Equal(t, tt.wantIfc, e.Interface)
			assert.Equal(t, tt.wantKind, e.Kind)
		})
	}
}

// A root qdisc belonging to an already-running steadybit attack on the same
// netns must not trip the preflight — the apply-time conflict detection owns
// that decision. We simulate an active attack by seeding activeNetfault.
func TestPreflightCheck_SkippedWhenAttackActive(t *testing.T) {
	const ns = "ns-active"
	activeNetfaultLock.Lock()
	activeNetfault[ns] = []Opts{&DelayOpts{Interfaces: []string{"docker0"}}}
	activeNetfaultLock.Unlock()
	defer func() {
		activeNetfaultLock.Lock()
		delete(activeNetfault, ns)
		activeNetfaultLock.Unlock()
	}()

	r := &fakeRunner{netNsId: ns, stdout: `qdisc htb 1: dev eth0 root`}
	err := PreflightCheck(context.Background(), r, &DelayOpts{Interfaces: []string{"eth0"}})
	assert.NoError(t, err, "preflight must defer to conflict detection when an attack is already active")
	assert.Empty(t, r.calls, "preflight should not even inspect when an attack is already active")
}

// With SetStrictRootQdisc(true) the only safe root is `noqueue`; every other
// pre-existing root — including the kernel default `mq` — is refused.
func TestPreflightCheck_StrictMode(t *testing.T) {
	SetStrictRootQdisc(true)
	defer SetStrictRootQdisc(false)

	cases := []struct {
		name     string
		tcOutput string
		wantErr  bool
		wantKind string
	}{
		{name: "noqueue allowed", tcOutput: `qdisc noqueue 0: dev eth0 root refcnt 2`, wantErr: false},
		{name: "mq refused", tcOutput: `qdisc mq 8002: dev eth0 root`, wantErr: true, wantKind: "mq"},
		{name: "fq_codel refused", tcOutput: `qdisc fq_codel 0: dev eth0 root`, wantErr: true, wantKind: "fq_codel"},
		{name: "htb refused", tcOutput: `qdisc htb 1: dev eth0 root`, wantErr: true, wantKind: "htb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRunner{netNsId: "strict-" + tc.name, stdout: tc.tcOutput}
			err := PreflightCheck(context.Background(), r, &DelayOpts{Interfaces: []string{"eth0"}})
			if !tc.wantErr {
				assert.NoError(t, err)
				return
			}
			var e *ErrUserRootQdisc
			require.ErrorAs(t, err, &e)
			assert.Equal(t, tc.wantKind, e.Kind)
		})
	}
}
