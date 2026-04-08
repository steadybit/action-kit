// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package netfault

import (
	"net"
	"testing"

	"github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testChain = "SB_TCP_RST_aabbccddeeff"
const testMangleChain = "SB_TCP_RST_M_aabbccddeeff"

var testExecCtx = ExecutionContext{TargetExecutionId: "00000000-0000-0000-0000-aabbccddeeff"}

var stdDeleteScript = []string{
	"*filter",
	"-D OUTPUT -j " + testChain,
	"-D INPUT -j " + testChain,
	"-D FORWARD -j " + testChain,
	"-F " + testChain,
	"-X " + testChain,
	"COMMIT",
}

func TestTcpResetOpts_iptablesScripts(t *testing.T) {
	tests := []struct {
		name      string
		opts      TcpResetOpts
		wantAddV4 []string
		wantDelV4 []string
		wantAddV6 []string
		wantDelV6 []string
	}{
		{
			name: "all traffic",
			opts: TcpResetOpts{
				Filter: Filter{
					Include: network.NewNetWithPortRanges(network.NetAny, network.PortRangeAny),
				},
				ExecutionContext: testExecCtx,
			},
			wantAddV4: []string{
				"*filter",
				":" + testChain + " - [0:0]",
				"-A OUTPUT -j " + testChain,
				"-A INPUT -j " + testChain,
				"-A FORWARD -j " + testChain,
				"-A " + testChain + " -p tcp -d 0.0.0.0/0 --dport 1:65534 -j REJECT --reject-with tcp-reset",
				"-A " + testChain + " -p tcp -s 0.0.0.0/0 --sport 1:65534 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV4: stdDeleteScript,
			wantAddV6: []string{
				"*filter",
				":" + testChain + " - [0:0]",
				"-A OUTPUT -j " + testChain,
				"-A INPUT -j " + testChain,
				"-A FORWARD -j " + testChain,
				"-A " + testChain + " -p tcp -d ::/0 --dport 1:65534 -j REJECT --reject-with tcp-reset",
				"-A " + testChain + " -p tcp -s ::/0 --sport 1:65534 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV6: stdDeleteScript,
		},
		{
			name: "single port",
			opts: TcpResetOpts{
				Filter: Filter{
					Include: network.NewNetWithPortRanges(network.NetAny, network.PortRange{From: 8080, To: 8080}),
				},
				ExecutionContext: testExecCtx,
			},
			wantAddV4: []string{
				"*filter",
				":" + testChain + " - [0:0]",
				"-A OUTPUT -j " + testChain,
				"-A INPUT -j " + testChain,
				"-A FORWARD -j " + testChain,
				"-A " + testChain + " -p tcp -d 0.0.0.0/0 --dport 8080 -j REJECT --reject-with tcp-reset",
				"-A " + testChain + " -p tcp -s 0.0.0.0/0 --sport 8080 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV4: stdDeleteScript,
			wantAddV6: []string{
				"*filter",
				":" + testChain + " - [0:0]",
				"-A OUTPUT -j " + testChain,
				"-A INPUT -j " + testChain,
				"-A FORWARD -j " + testChain,
				"-A " + testChain + " -p tcp -d ::/0 --dport 8080 -j REJECT --reject-with tcp-reset",
				"-A " + testChain + " -p tcp -s ::/0 --sport 8080 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV6: stdDeleteScript,
		},
		{
			name: "port range",
			opts: TcpResetOpts{
				Filter: Filter{
					Include: network.NewNetWithPortRanges(network.NetAny, network.PortRange{From: 8000, To: 8999}),
				},
				ExecutionContext: testExecCtx,
			},
			wantAddV4: []string{
				"*filter",
				":" + testChain + " - [0:0]",
				"-A OUTPUT -j " + testChain,
				"-A INPUT -j " + testChain,
				"-A FORWARD -j " + testChain,
				"-A " + testChain + " -p tcp -d 0.0.0.0/0 --dport 8000:8999 -j REJECT --reject-with tcp-reset",
				"-A " + testChain + " -p tcp -s 0.0.0.0/0 --sport 8000:8999 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV4: stdDeleteScript,
			wantAddV6: []string{
				"*filter",
				":" + testChain + " - [0:0]",
				"-A OUTPUT -j " + testChain,
				"-A INPUT -j " + testChain,
				"-A FORWARD -j " + testChain,
				"-A " + testChain + " -p tcp -d ::/0 --dport 8000:8999 -j REJECT --reject-with tcp-reset",
				"-A " + testChain + " -p tcp -s ::/0 --sport 8000:8999 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV6: stdDeleteScript,
		},
		{
			name: "with excludes",
			opts: TcpResetOpts{
				Filter: Filter{
					Include: []network.NetWithPortRange{
						mustParseNetWithPortRange("0.0.0.0/0", "*"),
						mustParseNetWithPortRange("0.0.0.0/0", "*"),
						mustParseNetWithPortRange("::0/0", "*"),
					},
					Exclude: []network.NetWithPortRange{
						mustParseNetWithPortRange("192.168.2.1/32", "80"),
						mustParseNetWithPortRange("192.168.2.1/32", "80"),
						mustParseNetWithPortRange("ff02::114/128", "8000-8999"),
					},
				},
				ExecutionContext: testExecCtx,
			},
			wantAddV4: []string{
				"*filter",
				":" + testChain + " - [0:0]",
				"-A OUTPUT -j " + testChain,
				"-A INPUT -j " + testChain,
				"-A FORWARD -j " + testChain,
				"-A " + testChain + " -p tcp -d 192.168.2.1/32 --dport 80 -j ACCEPT",
				"-A " + testChain + " -p tcp -s 192.168.2.1/32 --sport 80 -j ACCEPT",
				"-A " + testChain + " -p tcp -d 0.0.0.0/0 --dport 1:65534 -j REJECT --reject-with tcp-reset",
				"-A " + testChain + " -p tcp -s 0.0.0.0/0 --sport 1:65534 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV4: stdDeleteScript,
			wantAddV6: []string{
				"*filter",
				":" + testChain + " - [0:0]",
				"-A OUTPUT -j " + testChain,
				"-A INPUT -j " + testChain,
				"-A FORWARD -j " + testChain,
				"-A " + testChain + " -p tcp -d ff02::114/128 --dport 8000:8999 -j ACCEPT",
				"-A " + testChain + " -p tcp -s ff02::114/128 --sport 8000:8999 -j ACCEPT",
				"-A " + testChain + " -p tcp -d ::/0 --dport 1:65534 -j REJECT --reject-with tcp-reset",
				"-A " + testChain + " -p tcp -s ::/0 --sport 1:65534 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV6: stdDeleteScript,
		},
		{
			name: "with interfaces",
			opts: TcpResetOpts{
				Filter: Filter{
					Include: network.NewNetWithPortRanges(network.NetAny, network.PortRange{From: 8080, To: 8080}),
				},
				Interfaces:       []string{"eth0", "eth1"},
				ExecutionContext: testExecCtx,
			},
			wantAddV4: []string{
				"*filter",
				":" + testChain + " - [0:0]",
				"-A OUTPUT -o eth0 -j " + testChain,
				"-A INPUT -i eth0 -j " + testChain,
				"-A FORWARD -i eth0 -j " + testChain,
				"-A FORWARD -o eth0 -j " + testChain,
				"-A OUTPUT -o eth1 -j " + testChain,
				"-A INPUT -i eth1 -j " + testChain,
				"-A FORWARD -i eth1 -j " + testChain,
				"-A FORWARD -o eth1 -j " + testChain,
				"-A " + testChain + " -p tcp -d 0.0.0.0/0 --dport 8080 -j REJECT --reject-with tcp-reset",
				"-A " + testChain + " -p tcp -s 0.0.0.0/0 --sport 8080 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV4: []string{
				"*filter",
				"-D OUTPUT -o eth0 -j " + testChain,
				"-D INPUT -i eth0 -j " + testChain,
				"-D FORWARD -i eth0 -j " + testChain,
				"-D FORWARD -o eth0 -j " + testChain,
				"-D OUTPUT -o eth1 -j " + testChain,
				"-D INPUT -i eth1 -j " + testChain,
				"-D FORWARD -i eth1 -j " + testChain,
				"-D FORWARD -o eth1 -j " + testChain,
				"-F " + testChain,
				"-X " + testChain,
				"COMMIT",
			},
			wantAddV6: []string{
				"*filter",
				":" + testChain + " - [0:0]",
				"-A OUTPUT -o eth0 -j " + testChain,
				"-A INPUT -i eth0 -j " + testChain,
				"-A FORWARD -i eth0 -j " + testChain,
				"-A FORWARD -o eth0 -j " + testChain,
				"-A OUTPUT -o eth1 -j " + testChain,
				"-A INPUT -i eth1 -j " + testChain,
				"-A FORWARD -i eth1 -j " + testChain,
				"-A FORWARD -o eth1 -j " + testChain,
				"-A " + testChain + " -p tcp -d ::/0 --dport 8080 -j REJECT --reject-with tcp-reset",
				"-A " + testChain + " -p tcp -s ::/0 --sport 8080 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV6: []string{
				"*filter",
				"-D OUTPUT -o eth0 -j " + testChain,
				"-D INPUT -i eth0 -j " + testChain,
				"-D FORWARD -i eth0 -j " + testChain,
				"-D FORWARD -o eth0 -j " + testChain,
				"-D OUTPUT -o eth1 -j " + testChain,
				"-D INPUT -i eth1 -j " + testChain,
				"-D FORWARD -i eth1 -j " + testChain,
				"-D FORWARD -o eth1 -j " + testChain,
				"-F " + testChain,
				"-X " + testChain,
				"COMMIT",
			},
		},
		{
			name: "prepend",
			opts: TcpResetOpts{
				Filter: Filter{
					Include: network.NewNetWithPortRanges(network.NetAny, network.PortRangeAny),
				},
				ExecutionContext: testExecCtx,
				Prepend:          true,
			},
			wantAddV4: []string{
				"*filter",
				":" + testChain + " - [0:0]",
				"-I OUTPUT 1 -j " + testChain,
				"-I INPUT 1 -j " + testChain,
				"-I FORWARD 1 -j " + testChain,
				"-A " + testChain + " -p tcp -d 0.0.0.0/0 --dport 1:65534 -j REJECT --reject-with tcp-reset",
				"-A " + testChain + " -p tcp -s 0.0.0.0/0 --sport 1:65534 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV4: stdDeleteScript,
			wantAddV6: []string{
				"*filter",
				":" + testChain + " - [0:0]",
				"-I OUTPUT 1 -j " + testChain,
				"-I INPUT 1 -j " + testChain,
				"-I FORWARD 1 -j " + testChain,
				"-A " + testChain + " -p tcp -d ::/0 --dport 1:65534 -j REJECT --reject-with tcp-reset",
				"-A " + testChain + " -p tcp -s ::/0 --sport 1:65534 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV6: stdDeleteScript,
		},
		{
			name: "mangle mark mode",
			opts: TcpResetOpts{
				Filter: Filter{
					Include: network.NewNetWithPortRanges(network.NetAny, network.PortRange{From: 8080, To: 8080}),
				},
				ExecutionContext: testExecCtx,
				UseMangleChain:    true,
			},
			wantAddV4: []string{
				"*mangle",
				":" + testMangleChain + " - [0:0]",
				"-A OUTPUT -j " + testMangleChain,
				"-A PREROUTING -j " + testMangleChain,
				"-A FORWARD -j " + testMangleChain,
				"-A " + testMangleChain + " -p tcp -d 0.0.0.0/0 --dport 8080 -j MARK --set-mark 0x5B",
				"-A " + testMangleChain + " -p tcp -s 0.0.0.0/0 --sport 8080 -j MARK --set-mark 0x5B",
				"COMMIT",
				"*filter",
				":" + testChain + " - [0:0]",
				"-A OUTPUT -j " + testChain,
				"-A INPUT -j " + testChain,
				"-A FORWARD -j " + testChain,
				"-A " + testChain + " -p tcp -m mark --mark 0x5B -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV4: []string{
				"*mangle",
				"-D OUTPUT -j " + testMangleChain,
				"-D PREROUTING -j " + testMangleChain,
				"-D FORWARD -j " + testMangleChain,
				"-F " + testMangleChain,
				"-X " + testMangleChain,
				"COMMIT",
				"*filter",
				"-D OUTPUT -j " + testChain,
				"-D INPUT -j " + testChain,
				"-D FORWARD -j " + testChain,
				"-F " + testChain,
				"-X " + testChain,
				"COMMIT",
			},
			wantAddV6: []string{
				"*mangle",
				":" + testMangleChain + " - [0:0]",
				"-A OUTPUT -j " + testMangleChain,
				"-A PREROUTING -j " + testMangleChain,
				"-A FORWARD -j " + testMangleChain,
				"-A " + testMangleChain + " -p tcp -d ::/0 --dport 8080 -j MARK --set-mark 0x5B",
				"-A " + testMangleChain + " -p tcp -s ::/0 --sport 8080 -j MARK --set-mark 0x5B",
				"COMMIT",
				"*filter",
				":" + testChain + " - [0:0]",
				"-A OUTPUT -j " + testChain,
				"-A INPUT -j " + testChain,
				"-A FORWARD -j " + testChain,
				"-A " + testChain + " -p tcp -m mark --mark 0x5B -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV6: []string{
				"*mangle",
				"-D OUTPUT -j " + testMangleChain,
				"-D PREROUTING -j " + testMangleChain,
				"-D FORWARD -j " + testMangleChain,
				"-F " + testMangleChain,
				"-X " + testMangleChain,
				"COMMIT",
				"*filter",
				"-D OUTPUT -j " + testChain,
				"-D INPUT -j " + testChain,
				"-D FORWARD -j " + testChain,
				"-F " + testChain,
				"-X " + testChain,
				"COMMIT",
			},
		},
		{
			name: "mangle mark with excludes and interface",
			opts: TcpResetOpts{
				Filter: Filter{
					Include: network.NewNetWithPortRanges([]net.IPNet{network.NetAnyIpv4}, network.PortRange{From: 9080, To: 9080}),
					Exclude: []network.NetWithPortRange{
						mustParseNetWithPortRange("10.0.0.1/32", "9080"),
					},
				},
				Interfaces:       []string{"eth0"},
				ExecutionContext: testExecCtx,
				UseMangleChain:    true,
			},
			wantAddV4: []string{
				"*mangle",
				":" + testMangleChain + " - [0:0]",
				"-A OUTPUT -o eth0 -j " + testMangleChain,
				"-A PREROUTING -i eth0 -j " + testMangleChain,
				"-A FORWARD -i eth0 -j " + testMangleChain,
				"-A FORWARD -o eth0 -j " + testMangleChain,
				"-A " + testMangleChain + " -p tcp -d 10.0.0.1/32 --dport 9080 -j RETURN",
				"-A " + testMangleChain + " -p tcp -s 10.0.0.1/32 --sport 9080 -j RETURN",
				"-A " + testMangleChain + " -p tcp -d 0.0.0.0/0 --dport 9080 -j MARK --set-mark 0x5B",
				"-A " + testMangleChain + " -p tcp -s 0.0.0.0/0 --sport 9080 -j MARK --set-mark 0x5B",
				"COMMIT",
				"*filter",
				":" + testChain + " - [0:0]",
				"-A OUTPUT -o eth0 -j " + testChain,
				"-A INPUT -i eth0 -j " + testChain,
				"-A FORWARD -i eth0 -j " + testChain,
				"-A FORWARD -o eth0 -j " + testChain,
				"-A " + testChain + " -p tcp -m mark --mark 0x5B -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV4: []string{
				"*mangle",
				"-D OUTPUT -o eth0 -j " + testMangleChain,
				"-D PREROUTING -i eth0 -j " + testMangleChain,
				"-D FORWARD -i eth0 -j " + testMangleChain,
				"-D FORWARD -o eth0 -j " + testMangleChain,
				"-F " + testMangleChain,
				"-X " + testMangleChain,
				"COMMIT",
				"*filter",
				"-D OUTPUT -o eth0 -j " + testChain,
				"-D INPUT -i eth0 -j " + testChain,
				"-D FORWARD -i eth0 -j " + testChain,
				"-D FORWARD -o eth0 -j " + testChain,
				"-F " + testChain,
				"-X " + testChain,
				"COMMIT",
			},
			wantAddV6: nil,
			wantDelV6: nil,
		},
		{
			name: "ipv4-only include produces nil v6 script",
			opts: TcpResetOpts{
				Filter: Filter{
					Include: network.NewNetWithPortRanges([]net.IPNet{network.NetAnyIpv4}, network.PortRangeAny),
				},
				ExecutionContext: testExecCtx,
			},
			wantAddV4: []string{
				"*filter",
				":" + testChain + " - [0:0]",
				"-A OUTPUT -j " + testChain,
				"-A INPUT -j " + testChain,
				"-A FORWARD -j " + testChain,
				"-A " + testChain + " -p tcp -d 0.0.0.0/0 --dport 1:65534 -j REJECT --reject-with tcp-reset",
				"-A " + testChain + " -p tcp -s 0.0.0.0/0 --sport 1:65534 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV4: stdDeleteScript,
			wantAddV6: nil,
			wantDelV6: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addV4, addV6, err := tt.opts.iptablesScripts(modeAdd)
			require.NoError(t, err)
			assert.Equal(t, tt.wantAddV4, addV4)
			assert.Equal(t, tt.wantAddV6, addV6)

			delV4, delV6, err := tt.opts.iptablesScripts(modeDelete)
			require.NoError(t, err)
			assert.Equal(t, tt.wantDelV4, delV4)
			assert.Equal(t, tt.wantDelV6, delV6)
		})
	}
}

func TestTcpResetOpts_chainName(t *testing.T) {
	assert.Equal(t, "SB_TCP_RST_446655440000",
		(&TcpResetOpts{ExecutionContext: ExecutionContext{TargetExecutionId: "550e8400-e29b-41d4-a716-446655440000"}}).rstFilterChainName())

	assert.Equal(t, "SB_TCP_RST_M_446655440000",
		(&TcpResetOpts{ExecutionContext: ExecutionContext{TargetExecutionId: "550e8400-e29b-41d4-a716-446655440000"}}).rstMangleChainName())

	assert.Equal(t, "SB_TCP_RST_short",
		(&TcpResetOpts{ExecutionContext: ExecutionContext{TargetExecutionId: "short"}}).rstFilterChainName())

	assert.Equal(t, "SB_TCP_RST_default",
		(&TcpResetOpts{}).rstFilterChainName())
}

func TestTcpResetOpts_ipAndTcCommandsReturnNil(t *testing.T) {
	opts := &TcpResetOpts{
		Filter: Filter{
			Include: network.NewNetWithPortRanges(network.NetAny, network.PortRangeAny),
		},
	}

	ipCmds, err := opts.ipCommands(familyV4, modeAdd)
	require.NoError(t, err)
	assert.Nil(t, ipCmds)

	ipCmds, err = opts.ipCommands(familyV6, modeAdd)
	require.NoError(t, err)
	assert.Nil(t, ipCmds)

	tcCmds, err := opts.tcCommands(modeAdd)
	require.NoError(t, err)
	assert.Nil(t, tcCmds)
}

func TestTcpResetOpts_doesConflictWith(t *testing.T) {
	filter := Filter{Include: network.NewNetWithPortRanges(network.NetAny, network.PortRangeAny)}

	opts := &TcpResetOpts{Filter: filter}

	assert.False(t, opts.doesConflictWith(&TcpResetOpts{Filter: filter}), "identical opts should not conflict")
	assert.False(t, opts.doesConflictWith(&TcpResetOpts{Filter: filter, Interfaces: nil}), "nil and empty interfaces should not conflict")
	assert.True(t, opts.doesConflictWith(&TcpResetOpts{Filter: filter, Interfaces: []string{"eth0"}}), "different interfaces should conflict")
	assert.True(t, opts.doesConflictWith(&TcpResetOpts{Filter: Filter{Include: network.NewNetWithPortRanges(network.NetAny, network.PortRange{From: 80, To: 80})}}), "different filter should conflict")
	assert.True(t, opts.doesConflictWith(&BlackholeOpts{Filter: filter}), "different opts type should conflict")
}

func TestTcpResetOpts_String(t *testing.T) {
	assert.Equal(t,
		"resetting tcp connections (filter)\nto/from:\n 0.0.0.0/0\n ::/0\n",
		(&TcpResetOpts{
			Filter: Filter{Include: network.NewNetWithPortRanges(network.NetAny, network.PortRangeAny)},
		}).String(),
	)

	assert.Equal(t,
		"resetting tcp connections (filter, interfaces: eth0, eth1)\nto/from:\n 0.0.0.0/0 8000-8999\n ::/0 8000-8999\n",
		(&TcpResetOpts{
			Filter:     Filter{Include: network.NewNetWithPortRanges(network.NetAny, network.PortRange{From: 8000, To: 8999})},
			Interfaces: []string{"eth0", "eth1"},
		}).String(),
	)

	assert.Equal(t,
		"resetting tcp connections (mangle+filter mark)\nto/from:\n 0.0.0.0/0\n ::/0\n",
		(&TcpResetOpts{
			UseMangleChain: true,
			Filter:        Filter{Include: network.NewNetWithPortRanges(network.NetAny, network.PortRangeAny)},
		}).String(),
	)
}
