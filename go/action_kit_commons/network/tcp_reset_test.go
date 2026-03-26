// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package network

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deleteScript is the standard no-interface delete script, shared across test cases.
var stdDeleteScript = []string{
	"*filter",
	"-D OUTPUT -j STEADYBIT_TCP_RESET",
	"-D INPUT -j STEADYBIT_TCP_RESET",
	"-D FORWARD -j STEADYBIT_TCP_RESET",
	"-F STEADYBIT_TCP_RESET",
	"-X STEADYBIT_TCP_RESET",
	"COMMIT",
}

func TestTcpResetOpts_IptablesScripts(t *testing.T) {
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
					Include: NewNetWithPortRanges(NetAny, PortRangeAny),
				},
			},
			wantAddV4: []string{
				"*filter",
				":STEADYBIT_TCP_RESET - [0:0]",
				"-A OUTPUT -j STEADYBIT_TCP_RESET",
				"-A INPUT -j STEADYBIT_TCP_RESET",
				"-A FORWARD -j STEADYBIT_TCP_RESET",
				"-A STEADYBIT_TCP_RESET -p tcp -d 0.0.0.0/0 --dport 1:65534 -j REJECT --reject-with tcp-reset",
				"-A STEADYBIT_TCP_RESET -p tcp -s 0.0.0.0/0 --sport 1:65534 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV4: stdDeleteScript,
			wantAddV6: []string{
				"*filter",
				":STEADYBIT_TCP_RESET - [0:0]",
				"-A OUTPUT -j STEADYBIT_TCP_RESET",
				"-A INPUT -j STEADYBIT_TCP_RESET",
				"-A FORWARD -j STEADYBIT_TCP_RESET",
				"-A STEADYBIT_TCP_RESET -p tcp -d ::/0 --dport 1:65534 -j REJECT --reject-with tcp-reset",
				"-A STEADYBIT_TCP_RESET -p tcp -s ::/0 --sport 1:65534 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV6: stdDeleteScript,
		},
		{
			name: "single port",
			opts: TcpResetOpts{
				Filter: Filter{
					Include: NewNetWithPortRanges(NetAny, PortRange{From: 8080, To: 8080}),
				},
			},
			wantAddV4: []string{
				"*filter",
				":STEADYBIT_TCP_RESET - [0:0]",
				"-A OUTPUT -j STEADYBIT_TCP_RESET",
				"-A INPUT -j STEADYBIT_TCP_RESET",
				"-A FORWARD -j STEADYBIT_TCP_RESET",
				"-A STEADYBIT_TCP_RESET -p tcp -d 0.0.0.0/0 --dport 8080 -j REJECT --reject-with tcp-reset",
				"-A STEADYBIT_TCP_RESET -p tcp -s 0.0.0.0/0 --sport 8080 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV4: stdDeleteScript,
			wantAddV6: []string{
				"*filter",
				":STEADYBIT_TCP_RESET - [0:0]",
				"-A OUTPUT -j STEADYBIT_TCP_RESET",
				"-A INPUT -j STEADYBIT_TCP_RESET",
				"-A FORWARD -j STEADYBIT_TCP_RESET",
				"-A STEADYBIT_TCP_RESET -p tcp -d ::/0 --dport 8080 -j REJECT --reject-with tcp-reset",
				"-A STEADYBIT_TCP_RESET -p tcp -s ::/0 --sport 8080 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV6: stdDeleteScript,
		},
		{
			name: "port range",
			opts: TcpResetOpts{
				Filter: Filter{
					Include: NewNetWithPortRanges(NetAny, PortRange{From: 8000, To: 8999}),
				},
			},
			wantAddV4: []string{
				"*filter",
				":STEADYBIT_TCP_RESET - [0:0]",
				"-A OUTPUT -j STEADYBIT_TCP_RESET",
				"-A INPUT -j STEADYBIT_TCP_RESET",
				"-A FORWARD -j STEADYBIT_TCP_RESET",
				"-A STEADYBIT_TCP_RESET -p tcp -d 0.0.0.0/0 --dport 8000:8999 -j REJECT --reject-with tcp-reset",
				"-A STEADYBIT_TCP_RESET -p tcp -s 0.0.0.0/0 --sport 8000:8999 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV4: stdDeleteScript,
			wantAddV6: []string{
				"*filter",
				":STEADYBIT_TCP_RESET - [0:0]",
				"-A OUTPUT -j STEADYBIT_TCP_RESET",
				"-A INPUT -j STEADYBIT_TCP_RESET",
				"-A FORWARD -j STEADYBIT_TCP_RESET",
				"-A STEADYBIT_TCP_RESET -p tcp -d ::/0 --dport 8000:8999 -j REJECT --reject-with tcp-reset",
				"-A STEADYBIT_TCP_RESET -p tcp -s ::/0 --sport 8000:8999 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV6: stdDeleteScript,
		},
		{
			name: "with excludes",
			opts: TcpResetOpts{
				Filter: Filter{
					Include: []NetWithPortRange{
						mustParseNetWithPortRange("0.0.0.0/0", "*"),
						mustParseNetWithPortRange("0.0.0.0/0", "*"), // should deduplicate
						mustParseNetWithPortRange("::0/0", "*"),
					},
					Exclude: []NetWithPortRange{
						mustParseNetWithPortRange("192.168.2.1/32", "80"),
						mustParseNetWithPortRange("192.168.2.1/32", "80"), // should deduplicate
						mustParseNetWithPortRange("ff02::114/128", "8000-8999"),
					},
				},
			},
			wantAddV4: []string{
				"*filter",
				":STEADYBIT_TCP_RESET - [0:0]",
				"-A OUTPUT -j STEADYBIT_TCP_RESET",
				"-A INPUT -j STEADYBIT_TCP_RESET",
				"-A FORWARD -j STEADYBIT_TCP_RESET",
				"-A STEADYBIT_TCP_RESET -p tcp -d 192.168.2.1/32 --dport 80 -j ACCEPT",
				"-A STEADYBIT_TCP_RESET -p tcp -s 192.168.2.1/32 --sport 80 -j ACCEPT",
				"-A STEADYBIT_TCP_RESET -p tcp -d 0.0.0.0/0 --dport 1:65534 -j REJECT --reject-with tcp-reset",
				"-A STEADYBIT_TCP_RESET -p tcp -s 0.0.0.0/0 --sport 1:65534 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV4: stdDeleteScript,
			wantAddV6: []string{
				"*filter",
				":STEADYBIT_TCP_RESET - [0:0]",
				"-A OUTPUT -j STEADYBIT_TCP_RESET",
				"-A INPUT -j STEADYBIT_TCP_RESET",
				"-A FORWARD -j STEADYBIT_TCP_RESET",
				"-A STEADYBIT_TCP_RESET -p tcp -d ff02::114/128 --dport 8000:8999 -j ACCEPT",
				"-A STEADYBIT_TCP_RESET -p tcp -s ff02::114/128 --sport 8000:8999 -j ACCEPT",
				"-A STEADYBIT_TCP_RESET -p tcp -d ::/0 --dport 1:65534 -j REJECT --reject-with tcp-reset",
				"-A STEADYBIT_TCP_RESET -p tcp -s ::/0 --sport 1:65534 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV6: stdDeleteScript,
		},
		{
			name: "with interfaces",
			opts: TcpResetOpts{
				Filter: Filter{
					Include: NewNetWithPortRanges(NetAny, PortRange{From: 8080, To: 8080}),
				},
				Interfaces: []string{"eth0", "eth1"},
			},
			wantAddV4: []string{
				"*filter",
				":STEADYBIT_TCP_RESET - [0:0]",
				"-A OUTPUT -o eth0 -j STEADYBIT_TCP_RESET",
				"-A INPUT -i eth0 -j STEADYBIT_TCP_RESET",
				"-A FORWARD -i eth0 -j STEADYBIT_TCP_RESET",
				"-A FORWARD -o eth0 -j STEADYBIT_TCP_RESET",
				"-A OUTPUT -o eth1 -j STEADYBIT_TCP_RESET",
				"-A INPUT -i eth1 -j STEADYBIT_TCP_RESET",
				"-A FORWARD -i eth1 -j STEADYBIT_TCP_RESET",
				"-A FORWARD -o eth1 -j STEADYBIT_TCP_RESET",
				"-A STEADYBIT_TCP_RESET -p tcp -d 0.0.0.0/0 --dport 8080 -j REJECT --reject-with tcp-reset",
				"-A STEADYBIT_TCP_RESET -p tcp -s 0.0.0.0/0 --sport 8080 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV4: []string{
				"*filter",
				"-D OUTPUT -o eth0 -j STEADYBIT_TCP_RESET",
				"-D INPUT -i eth0 -j STEADYBIT_TCP_RESET",
				"-D FORWARD -i eth0 -j STEADYBIT_TCP_RESET",
				"-D FORWARD -o eth0 -j STEADYBIT_TCP_RESET",
				"-D OUTPUT -o eth1 -j STEADYBIT_TCP_RESET",
				"-D INPUT -i eth1 -j STEADYBIT_TCP_RESET",
				"-D FORWARD -i eth1 -j STEADYBIT_TCP_RESET",
				"-D FORWARD -o eth1 -j STEADYBIT_TCP_RESET",
				"-F STEADYBIT_TCP_RESET",
				"-X STEADYBIT_TCP_RESET",
				"COMMIT",
			},
			wantAddV6: []string{
				"*filter",
				":STEADYBIT_TCP_RESET - [0:0]",
				"-A OUTPUT -o eth0 -j STEADYBIT_TCP_RESET",
				"-A INPUT -i eth0 -j STEADYBIT_TCP_RESET",
				"-A FORWARD -i eth0 -j STEADYBIT_TCP_RESET",
				"-A FORWARD -o eth0 -j STEADYBIT_TCP_RESET",
				"-A OUTPUT -o eth1 -j STEADYBIT_TCP_RESET",
				"-A INPUT -i eth1 -j STEADYBIT_TCP_RESET",
				"-A FORWARD -i eth1 -j STEADYBIT_TCP_RESET",
				"-A FORWARD -o eth1 -j STEADYBIT_TCP_RESET",
				"-A STEADYBIT_TCP_RESET -p tcp -d ::/0 --dport 8080 -j REJECT --reject-with tcp-reset",
				"-A STEADYBIT_TCP_RESET -p tcp -s ::/0 --sport 8080 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV6: []string{
				"*filter",
				"-D OUTPUT -o eth0 -j STEADYBIT_TCP_RESET",
				"-D INPUT -i eth0 -j STEADYBIT_TCP_RESET",
				"-D FORWARD -i eth0 -j STEADYBIT_TCP_RESET",
				"-D FORWARD -o eth0 -j STEADYBIT_TCP_RESET",
				"-D OUTPUT -o eth1 -j STEADYBIT_TCP_RESET",
				"-D INPUT -i eth1 -j STEADYBIT_TCP_RESET",
				"-D FORWARD -i eth1 -j STEADYBIT_TCP_RESET",
				"-D FORWARD -o eth1 -j STEADYBIT_TCP_RESET",
				"-F STEADYBIT_TCP_RESET",
				"-X STEADYBIT_TCP_RESET",
				"COMMIT",
			},
		},
		{
			name: "ipv4-only include produces nil v6 script",
			opts: TcpResetOpts{
				Filter: Filter{
					Include: NewNetWithPortRanges([]net.IPNet{NetAnyIpv4}, PortRangeAny),
				},
			},
			wantAddV4: []string{
				"*filter",
				":STEADYBIT_TCP_RESET - [0:0]",
				"-A OUTPUT -j STEADYBIT_TCP_RESET",
				"-A INPUT -j STEADYBIT_TCP_RESET",
				"-A FORWARD -j STEADYBIT_TCP_RESET",
				"-A STEADYBIT_TCP_RESET -p tcp -d 0.0.0.0/0 --dport 1:65534 -j REJECT --reject-with tcp-reset",
				"-A STEADYBIT_TCP_RESET -p tcp -s 0.0.0.0/0 --sport 1:65534 -j REJECT --reject-with tcp-reset",
				"COMMIT",
			},
			wantDelV4: stdDeleteScript,
			wantAddV6: nil,
			wantDelV6: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addV4, addV6, err := tt.opts.IptablesScripts(ModeAdd)
			require.NoError(t, err)
			assert.Equal(t, tt.wantAddV4, addV4)
			assert.Equal(t, tt.wantAddV6, addV6)

			delV4, delV6, err := tt.opts.IptablesScripts(ModeDelete)
			require.NoError(t, err)
			assert.Equal(t, tt.wantDelV4, delV4)
			assert.Equal(t, tt.wantDelV6, delV6)
		})
	}
}

func TestTcpResetOpts_IpAndTcCommandsReturnNil(t *testing.T) {
	opts := &TcpResetOpts{
		Filter: Filter{
			Include: NewNetWithPortRanges(NetAny, PortRangeAny),
		},
	}

	ipCmds, err := opts.IpCommands(FamilyV4, ModeAdd)
	require.NoError(t, err)
	assert.Nil(t, ipCmds)

	ipCmds, err = opts.IpCommands(FamilyV6, ModeAdd)
	require.NoError(t, err)
	assert.Nil(t, ipCmds)

	tcCmds, err := opts.TcCommands(ModeAdd)
	require.NoError(t, err)
	assert.Nil(t, tcCmds)
}

func TestTcpResetOpts_DoesConflictWith(t *testing.T) {
	filter := Filter{Include: NewNetWithPortRanges(NetAny, PortRangeAny)}

	opts := &TcpResetOpts{Filter: filter}

	assert.False(t, opts.DoesConflictWith(&TcpResetOpts{Filter: filter}), "identical opts should not conflict")
	assert.False(t, opts.DoesConflictWith(&TcpResetOpts{Filter: filter, Interfaces: nil}), "nil and empty interfaces should not conflict")
	assert.True(t, opts.DoesConflictWith(&TcpResetOpts{Filter: filter, Interfaces: []string{"eth0"}}), "different interfaces should conflict")
	assert.True(t, opts.DoesConflictWith(&TcpResetOpts{Filter: Filter{Include: NewNetWithPortRanges(NetAny, PortRange{From: 80, To: 80})}}), "different filter should conflict")
	assert.True(t, opts.DoesConflictWith(&BlackholeOpts{Filter: filter}), "different opts type should conflict")
}

func TestTcpResetOpts_String(t *testing.T) {
	// all traffic, no interfaces
	assert.Equal(t,
		"resetting tcp connections\nto/from:\n 0.0.0.0/0\n ::/0\n",
		(&TcpResetOpts{
			Filter: Filter{Include: NewNetWithPortRanges(NetAny, PortRangeAny)},
		}).String(),
	)

	// single port, no interfaces
	assert.Equal(t,
		"resetting tcp connections\nto/from:\n 0.0.0.0/0 8080\n ::/0 8080\n",
		(&TcpResetOpts{
			Filter: Filter{Include: NewNetWithPortRanges(NetAny, PortRange{From: 8080, To: 8080})},
		}).String(),
	)

	// port range, with interfaces
	assert.Equal(t,
		"resetting tcp connections (interfaces: eth0, eth1)\nto/from:\n 0.0.0.0/0 8000-8999\n ::/0 8000-8999\n",
		(&TcpResetOpts{
			Filter:     Filter{Include: NewNetWithPortRanges(NetAny, PortRange{From: 8000, To: 8999})},
			Interfaces: []string{"eth0", "eth1"},
		}).String(),
	)

	// with excludes
	assert.Equal(t,
		"resetting tcp connections\nto/from:\n 0.0.0.0/0\n ::/0\nbut not from/to:\n 192.168.2.1/32 80\n ff02::114/128 8000-8999\n",
		(&TcpResetOpts{
			Filter: Filter{
				Include: []NetWithPortRange{
					mustParseNetWithPortRange("0.0.0.0/0", "*"),
					mustParseNetWithPortRange("::0/0", "*"),
				},
				Exclude: []NetWithPortRange{
					mustParseNetWithPortRange("192.168.2.1/32", "80"),
					mustParseNetWithPortRange("ff02::114/128", "8000-8999"),
				},
			},
		}).String(),
	)
}
