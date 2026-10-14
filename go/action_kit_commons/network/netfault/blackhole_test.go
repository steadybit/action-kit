// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package netfault

import (
	"context"
	"net"
	"slices"
	"testing"
	"testing/iotest"
	"time"

	"github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/stretchr/testify/assert"
)

func TestBlackholeOpts_IpCommands(t *testing.T) {
	tests := []struct {
		name         string
		opts         BlackholeOpts
		ipv6Disabled bool
		wantAddV4    []byte
		wantDelV4    []byte
		wantAddV6    []byte
		wantDelV6    []byte
		wantErr      bool
	}{
		{
			name: "blackhole",
			opts: BlackholeOpts{
				Filter: Filter{
					Include: []network.NetWithPortRange{
						mustParseNetWithPortRange("0.0.0.0/0", "*"),
						mustParseNetWithPortRange("0.0.0.0/0", "*"), //should filter that duplicate
						mustParseNetWithPortRange("::0/0", "*"),
					},
					Exclude: []network.NetWithPortRange{
						mustParseNetWithPortRange("192.168.2.1/32", "80"),
						mustParseNetWithPortRange("192.168.2.1/32", "80"), //should filter that duplicate
						mustParseNetWithPortRange("ff02::114/128", "8000-8999"),
					},
				},
			},
			wantAddV4: []byte(`rule add blackhole to 0.0.0.0/0
rule add blackhole from 0.0.0.0/0
rule add to 192.168.2.1/32 dport 80 table main
rule add from 192.168.2.1/32 sport 80 table main
`),
			wantDelV4: []byte(`rule del from 192.168.2.1/32 sport 80 table main
rule del to 192.168.2.1/32 dport 80 table main
rule del blackhole from 0.0.0.0/0
rule del blackhole to 0.0.0.0/0
`),
			wantAddV6: []byte(`rule add blackhole to ::/0
rule add blackhole from ::/0
rule add to ff02::114/128 dport 8000-8999 table main
rule add from ff02::114/128 sport 8000-8999 table main
`),
			wantDelV6: []byte(`rule del from ff02::114/128 sport 8000-8999 table main
rule del to ff02::114/128 dport 8000-8999 table main
rule del blackhole from ::/0
rule del blackhole to ::/0
`),
			wantErr: false,
		},
		{
			name: "blackhole udp port 123 only",
			opts: BlackholeOpts{
				IpProto: IpProtoUdp,
				Filter: Filter{
					Include: network.NewNetWithPortRanges(network.NetAny, network.PortRange{From: 123, To: 123}),
					Exclude: []network.NetWithPortRange{
						{Net: net.IPNet{IP: net.ParseIP("192.168.2.1"), Mask: net.CIDRMask(32, 32)}, PortRange: network.PortRange{From: 80, To: 80}},
					},
				},
			},
			wantAddV4: []byte(`rule add blackhole to 0.0.0.0/0 ipproto udp dport 123
rule add blackhole from 0.0.0.0/0 ipproto udp sport 123
`),
			wantDelV4: []byte(`rule del blackhole from 0.0.0.0/0 ipproto udp sport 123
rule del blackhole to 0.0.0.0/0 ipproto udp dport 123
`),
			wantAddV6: []byte(`rule add blackhole to ::/0 ipproto udp dport 123
rule add blackhole from ::/0 ipproto udp sport 123
`),
			wantDelV6: []byte(`rule del blackhole from ::/0 ipproto udp sport 123
rule del blackhole to ::/0 ipproto udp dport 123
`),
			wantErr: false,
		},
		{
			// No port specified => PortRangeAny => rule must omit dport/sport so
			// that portless protocols (e.g. ICMP) are blackholed as well.
			name: "blackhole a single host on all protocols when no port specified",
			opts: BlackholeOpts{
				Filter: Filter{
					Include: []network.NetWithPortRange{
						mustParseNetWithPortRange("10.0.0.5/32", "*"),
						mustParseNetWithPortRange("2001:db8::5/128", "*"),
					},
				},
			},
			wantAddV4: []byte(`rule add blackhole to 10.0.0.5/32
rule add blackhole from 10.0.0.5/32
`),
			wantDelV4: []byte(`rule del blackhole from 10.0.0.5/32
rule del blackhole to 10.0.0.5/32
`),
			wantAddV6: []byte(`rule add blackhole to 2001:db8::5/128
rule add blackhole from 2001:db8::5/128
`),
			wantDelV6: []byte(`rule del blackhole from 2001:db8::5/128
rule del blackhole to 2001:db8::5/128
`),
			wantErr: false,
		},
		{
			name: "blackhole including localhost ipv4 only leaves ipv6 rules alone",
			opts: BlackholeOpts{
				BlockLocalhost: true,
				Filter: Filter{
					Include: []network.NetWithPortRange{mustParseNetWithPortRange("127.0.0.1/32", "9999")},
				},
			},
			wantAddV4: []byte(`rule add to 127.0.0.0/8 lookup local
rule add blackhole to 127.0.0.1/32 dport 9999
rule add blackhole from 127.0.0.1/32 sport 9999
rule add not to 127.0.0.0/8 lookup local
rule del pref 0 lookup local
`),
			wantDelV4: []byte(`rule add pref 0 lookup local
rule del not to 127.0.0.0/8 lookup local
rule del blackhole from 127.0.0.1/32 sport 9999
rule del blackhole to 127.0.0.1/32 dport 9999
rule del to 127.0.0.0/8 lookup local
`),
			wantAddV6: []byte("\n"),
			wantDelV6: []byte("\n"),
			wantErr:   false,
		},
		{
			name: "blackhole including localhost",
			opts: BlackholeOpts{
				BlockLocalhost: true,
				Filter: Filter{
					Include: []network.NetWithPortRange{
						mustParseNetWithPortRange("127.0.0.0/8", "*"),
						mustParseNetWithPortRange("::1/128", "9999"),
					},
					Exclude: []network.NetWithPortRange{
						mustParseNetWithPortRange("127.0.0.1/32", "8080"),
					},
				},
			},
			wantAddV4: []byte(`rule add to 127.0.0.0/8 lookup local
rule add blackhole to 127.0.0.0/8
rule add blackhole from 127.0.0.0/8
rule add to 127.0.0.1/32 dport 8080 table main
rule add from 127.0.0.1/32 sport 8080 table main
rule add to 127.0.0.1/32 dport 8080 table local
rule add from 127.0.0.1/32 sport 8080 table local
rule add not to 127.0.0.0/8 lookup local
rule del pref 0 lookup local
`),
			wantDelV4: []byte(`rule add pref 0 lookup local
rule del not to 127.0.0.0/8 lookup local
rule del from 127.0.0.1/32 sport 8080 table local
rule del to 127.0.0.1/32 dport 8080 table local
rule del from 127.0.0.1/32 sport 8080 table main
rule del to 127.0.0.1/32 dport 8080 table main
rule del blackhole from 127.0.0.0/8
rule del blackhole to 127.0.0.0/8
rule del to 127.0.0.0/8 lookup local
`),
			wantAddV6: []byte(`rule add to ::1/128 lookup local
rule add blackhole to ::1/128 dport 9999
rule add blackhole from ::1/128 sport 9999
rule add not to ::1/128 lookup local
rule del pref 0 lookup local
`),
			wantDelV6: []byte(`rule add pref 0 lookup local
rule del not to ::1/128 lookup local
rule del blackhole from ::1/128 sport 9999
rule del blackhole to ::1/128 dport 9999
rule del to ::1/128 lookup local
`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ipv6Supported = func() bool {
				return !tt.ipv6Disabled
			}
			defer func() {
				ipv6Supported = defaultIpv6Supported
			}()

			gotAddV4, err := tt.opts.ipCommands(familyV4, modeAdd)
			if (err != nil) != tt.wantErr {
				t.Errorf("TcCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NoError(t, iotest.TestReader(toReader(gotAddV4), tt.wantAddV4))

			gotDelV4, err := tt.opts.ipCommands(familyV4, modeDelete)
			if (err != nil) != tt.wantErr {
				t.Errorf("TcCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NoError(t, iotest.TestReader(toReader(gotDelV4), tt.wantDelV4))

			gotAddV6, err := tt.opts.ipCommands(familyV6, modeAdd)
			if (err != nil) != tt.wantErr {
				t.Errorf("TcCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NoError(t, iotest.TestReader(toReader(gotAddV6), tt.wantAddV6))

			gotDelV6, err := tt.opts.ipCommands(familyV6, modeDelete)
			if (err != nil) != tt.wantErr {
				t.Errorf("TcCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NoError(t, iotest.TestReader(toReader(gotDelV6), tt.wantDelV6))
		})
	}
}

func TestBlackholeOpts_doesConflictWith(t *testing.T) {
	filter := Filter{Include: network.NewNetWithPortRanges(network.NetAny, network.PortRangeAny)}

	assert.False(t, (&BlackholeOpts{Filter: filter}).doesConflictWith(&BlackholeOpts{Filter: filter}), "identical opts should not conflict")
	assert.True(t, (&BlackholeOpts{Filter: filter, BlockLocalhost: true}).doesConflictWith(&BlackholeOpts{Filter: filter}), "localhost and non-localhost should conflict")
	assert.True(t, (&BlackholeOpts{Filter: filter}).doesConflictWith(&BlackholeOpts{Filter: filter, BlockLocalhost: true}), "non-localhost and localhost should conflict")
	assert.True(t, (&BlackholeOpts{Filter: filter, BlockLocalhost: true}).doesConflictWith(&BlackholeOpts{Filter: filter, BlockLocalhost: true}), "two localhost attacks should conflict")
}

func TestBlockLocalhost_RevertOfRejectedAttackKeepsRunningOne(t *testing.T) {
	ipv6Supported = func() bool { return false }
	defer func() { ipv6Supported = defaultIpv6Supported }()

	filter := Filter{Include: []network.NetWithPortRange{mustParseNetWithPortRange("127.0.0.0/8", "9999")}}
	running := &BlackholeOpts{Filter: filter, BlockLocalhost: true, ExecutionContext: ExecutionContext{TargetExecutionId: "running"}}
	rejected := &BlackholeOpts{Filter: filter, BlockLocalhost: true, ExecutionContext: ExecutionContext{TargetExecutionId: "rejected"}}
	ns := "localhost-exclusive"

	_, err := Apply(context.Background(), &fakeRunner{netNsId: ns}, running)
	assert.NoError(t, err)

	_, err = Apply(context.Background(), &fakeRunner{netNsId: ns}, rejected)
	assert.Error(t, err, "second localhost attack must be rejected")

	// the rejected attack never applied anything, its revert must not touch the running one's rules
	r := &fakeRunner{netNsId: ns}
	assert.NoError(t, Revert(context.Background(), r, rejected, QdiscSnapshot{}))
	assert.Empty(t, r.calls, "revert of a rejected attack must not run any command")

	r = &fakeRunner{netNsId: ns}
	assert.NoError(t, Revert(context.Background(), r, running, QdiscSnapshot{}))
	assert.True(t, revertRestoredDefaultLocalRule(r), "last attack must restore pref 0")
	assert.False(t, hasActiveNetfault(ns))
}

func revertRestoredDefaultLocalRule(r *fakeRunner) bool {
	for _, c := range r.calls {
		if slices.Contains(c.cmds, restoreDefaultLocalRuleCmd) {
			return true
		}
	}
	return false
}

func TestRevert_OfAttackRejectedByOtherTypeRunsNothing(t *testing.T) {
	ipv6Supported = func() bool { return false }
	defer func() { ipv6Supported = defaultIpv6Supported }()

	ns := "rejected-other-type"
	running := &BlackholeOpts{Filter: Filter{Include: []network.NetWithPortRange{mustParseNetWithPortRange("10.0.0.0/8", "*")}}}
	rejected := &DelayOpts{
		Filter:     Filter{Include: []network.NetWithPortRange{mustParseNetWithPortRange("0.0.0.0/0", "*")}},
		Delay:      100 * time.Millisecond,
		Interfaces: []string{"eth0"},
	}

	_, err := Apply(context.Background(), &fakeRunner{netNsId: ns}, running)
	assert.NoError(t, err)
	_, err = Apply(context.Background(), &fakeRunner{netNsId: ns}, rejected)
	assert.Error(t, err, "delay must be rejected while a blackhole is running")

	r := &fakeRunner{netNsId: ns}
	assert.NoError(t, Revert(context.Background(), r, rejected, QdiscSnapshot{}))
	assert.Empty(t, r.calls, "must not delete the running attack's rules or qdiscs")

	assert.NoError(t, Revert(context.Background(), &fakeRunner{netNsId: ns}, running, QdiscSnapshot{}))
	assert.False(t, hasActiveNetfault(ns))
}

func TestRevert_WithoutActiveAttacksStillRuns(t *testing.T) {
	ipv6Supported = func() bool { return false }
	defer func() { ipv6Supported = defaultIpv6Supported }()

	// e.g. after an extension restart nothing is tracked as active
	opts := &BlackholeOpts{Filter: Filter{Include: []network.NetWithPortRange{mustParseNetWithPortRange("10.0.0.0/8", "*")}}}
	r := &fakeRunner{netNsId: "restarted"}
	assert.NoError(t, Revert(context.Background(), r, opts, QdiscSnapshot{}))
	assert.NotEmpty(t, r.calls, "a regular revert must run its delete commands")
}
