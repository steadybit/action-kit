// Copyright 2025 steadybit GmbH. All rights reserved.
//go:build !windows

package network

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeRunner struct {
	calls []struct {
		args []string
		cmds []string
	}
}

func (f *fakeRunner) run(_ context.Context, args []string, cmds []string) (string, error) {
	f.calls = append(f.calls, struct {
		args []string
		cmds []string
	}{args: args, cmds: cmds})
	return "", nil
}

func (f *fakeRunner) id() string { return "testns" }

func TestApply_Order_IptablesBeforeTcWhenTcpPshOnly(t *testing.T) {
	// Disable ipv6 for the test to avoid ip6tables invocation
	ipv6Supported = func() bool { return false }
	defer func() { ipv6Supported = defaultIpv6Supported }()

	opts := &DelayOpts{
		Filter:     Filter{Include: []NetWithPortRange{mustParseNetWithPortRange("0.0.0.0/0", "*")}},
		Delay:      100 * time.Millisecond,
		Jitter:     10 * time.Millisecond,
		Interfaces: []string{"eth0"},
		TcpPshOnly: true,
	}

	r := &fakeRunner{}
	err := Apply(context.Background(), r, opts)
	assert.NoError(t, err)

	iptablesIdx := -1
	tcBatchIdx := -1
	for i, c := range r.calls {
		if len(c.args) > 0 && c.args[0] == "iptables-restore" {
			iptablesIdx = i
		}
		if len(c.args) > 0 && c.args[0] == "tc" && len(c.cmds) > 0 && strings.HasPrefix(c.cmds[0], "qdisc add") {
			tcBatchIdx = i
		}
	}

	if !(iptablesIdx >= 0 && tcBatchIdx >= 0) {
		t.Fatalf("expected both iptables-restore and tc batch calls, got: %+v", r.calls)
	}
	if !(iptablesIdx < tcBatchIdx) {
		t.Fatalf("expected iptables-restore to run before tc batch: iptablesIdx=%d, tcBatchIdx=%d", iptablesIdx, tcBatchIdx)
	}
}

func TestDelayOpts_IptablesScripts_FilterByFamily(t *testing.T) {
	opts := &DelayOpts{
		Filter: Filter{
			Include: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.0/24", "80-81"),
				mustParseNetWithPortRange("ff02::114/128", "8000-8001"),
			},
			Exclude: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				// Overlapping IPv6 exclude to ensure it is retained after optimizeFilter
				mustParseNetWithPortRange("ff02::114/128", "8000"),
			},
		},
		TcpPshOnly: true,
	}

	v4, v6, err := opts.IptablesScripts(ModeAdd)
	assert.NoError(t, err)
	// v4 script should not contain IPv6 addresses
	assert.NotContains(t, v4, "ff02::")
	assert.Contains(t, v4, "*mangle\n")
	assert.Contains(t, v4, "-A OUTPUT -j STEADYBIT_DELAY\n")
	assert.Contains(t, v4, "-A POSTROUTING -j STEADYBIT_DELAY\n")
	assert.Contains(t, v4, "--tcp-flags PSH PSH")
	assert.Contains(t, v4, "-d 192.168.2.1/32 --dport 80 -j RETURN")
	assert.Contains(t, v4, "-s 192.168.2.0/24 --sport 80:81 -j MARK --set-mark 0x1")

	// v6 script should not contain IPv4 addresses
	assert.NotContains(t, v6, "192.168.")
	assert.Contains(t, v6, "--tcp-flags PSH PSH")
	assert.Contains(t, v6, "-d ff02::114/128 --dport 8000 -j RETURN")
	assert.Contains(t, v6, "-s ff02::114/128 --sport 8000:8001 -j MARK --set-mark 0x1")
}

func TestCondenseNetWithPortRange(t *testing.T) {
	tests := []struct {
		name  string
		nwps  []NetWithPortRange
		limit int
		want  []NetWithPortRange
	}{
		{
			name: "must not condense when limit is higher than the number of elements",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("192.168.2.2/32", "80"),
			},
			limit: 3,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("192.168.2.2/32", "80"),
			},
		},
		{
			name: "must not condense ipv6 with ipv4",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("fe80::784c:f9ff:fe48:a552/128", "80"),
			},
			limit: 1,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("fe80::784c:f9ff:fe48:a552/128", "80"),
			},
		},
		{
			name: "must not condense different port ranges",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("192.168.2.1/32", "90"),
			},
			limit: 1,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("192.168.2.1/32", "90"),
			},
		},
		{
			name: "should condense greatest common prefix",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.4/32", "80-81"), //should be condensed with next
				mustParseNetWithPortRange("192.168.2.5/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"), //should not be condensed, different ports
				mustParseNetWithPortRange("192.168.2.10/32", "80-81"),
			},
			limit: 4,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.4/31", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"),
				mustParseNetWithPortRange("192.168.2.10/32", "80-81"),
			},
		},
		{
			name: "should condense greatest common prefix further",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.4/32", "80-81"), //should be condensed with next
				mustParseNetWithPortRange("192.168.2.5/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"), //should not be condensed, different ports
				mustParseNetWithPortRange("192.168.2.10/32", "80-81"),
			},
			limit: 3,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.0/29", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"),
				mustParseNetWithPortRange("192.168.2.10/32", "80-81"),
			},
		},
		{
			name: "should condense greatest common prefix further",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.4/32", "80-81"), //should be condensed with next
				mustParseNetWithPortRange("192.168.2.5/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"), //should not be condensed, different ports
				mustParseNetWithPortRange("192.168.2.10/32", "80-81"),
			},
			limit: 2,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.0/28", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"),
			},
		},
		{
			name: "should condense",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.0.0/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.4/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.8/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.12/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.16/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.20/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.24/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.28/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.32/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.36/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.40/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.44/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.48/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.52/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.56/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.60/30", "8086-8088"),
			},
			limit: 5,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.0.0/28", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.16/28", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.32/28", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.48/29", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.56/29", "8086-8088"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CondenseNetWithPortRange(tt.nwps, tt.limit)
			assert.Equalf(t, toString(tt.want), toString(result), "CondenseNetWithPortRange(%v, %v)", tt.nwps, tt.limit)
		})
	}
}

func toString(s []NetWithPortRange) string {
	slices.SortFunc(s, NetWithPortRange.Compare)
	var sb strings.Builder
	for _, portRange := range s {
		sb.WriteString(portRange.String())
		sb.WriteString("\n")
	}
	return sb.String()
}
