package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWinDivertGetStartEndIP1(t *testing.T) {
	parsedNet, err := ParseCIDR("1.1.1.1/24")
	require.NoError(t, err)

	startIp, endIp, err := getStartEndIP(*parsedNet)
	require.NoError(t, err)

	assert.Equal(t, "1.1.1.0", startIp.String())
	assert.Equal(t, "1.1.1.255", endIp.String())
}

func TestWinDivertGetStartEndIP2(t *testing.T) {
	parsedNet, err := ParseCIDR("1.1.3.120/22")
	require.NoError(t, err)

	startIp, endIp, err := getStartEndIP(*parsedNet)
	require.NoError(t, err)

	assert.Equal(t, "1.1.0.0", startIp.String())
	assert.Equal(t, "1.1.3.255", endIp.String())
}

func TestWinDivertBuildFilterInclude(t *testing.T) {
	net1, err := ParseCIDR("1.1.1.1/24")
	require.NoError(t, err)
	f := Filter{
		Include: []NetWithPortRange{
			{
				Net:       *net1,
				Comment:   "",
				PortRange: PortRange{From: 8000, To: 8002},
			},
		},
	}

	filter, err := buildWinDivertFilter(f, nil)
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and outbound and (( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.255 and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 ))))", filter)
}

func TestWinDivertBuildFilterIncludeIpv6(t *testing.T) {
	net1, err := ParseCIDR("::/0")
	require.NoError(t, err)
	f := Filter{
		Include: []NetWithPortRange{
			{
				Net:       *net1,
				Comment:   "",
				PortRange: PortRange{From: 8000, To: 8002},
			},
		},
	}

	filter, err := buildWinDivertFilter(f, nil)
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and outbound and (( ipv6.DstAddr >= :: and ipv6.DstAddr <= ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 ))))", filter)
}

func TestWinDivertBuildFilterExclude(t *testing.T) {
	net1, err := ParseCIDR("1.1.1.1/24")
	require.NoError(t, err)
	exemptNet, err := ParseCIDR("1.1.1.0")
	require.NoError(t, err)
	f := Filter{
		Include: []NetWithPortRange{
			{
				Net:       *net1,
				Comment:   "",
				PortRange: PortRange{From: 8000, To: 8002},
			},
		},
		Exclude: []NetWithPortRange{
			{
				Net:       *exemptNet,
				Comment:   "",
				PortRange: PortRange{From: 8000, To: 8002},
			},
		},
	}

	filter, err := buildWinDivertFilter(f, nil)
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and outbound and (( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.255 and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 )))) and ((( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.0 )? (( tcp.DstPort < 8000 or tcp.DstPort > 8002 ) or ( udp.DstPort < 8000 or udp.DstPort > 8002 )): true))", filter)
}

func TestWinDivertBuildFilterMultipleExcludes(t *testing.T) {
	net1, err := ParseCIDR("1.1.1.1/24")
	require.NoError(t, err)
	exemptNet, err := ParseCIDR("1.1.1.0")
	require.NoError(t, err)
	exemptNet2, err := ParseCIDR("1.1.1.1")
	require.NoError(t, err)
	f := Filter{
		Include: []NetWithPortRange{
			{
				Net:       *net1,
				Comment:   "",
				PortRange: PortRange{From: 8000, To: 8002},
			},
		},
		Exclude: []NetWithPortRange{
			{
				Net:       *exemptNet,
				Comment:   "",
				PortRange: PortRange{From: 8000, To: 8002},
			},
			{
				Net:       *exemptNet2,
				Comment:   "",
				PortRange: PortRange{From: 8000, To: 8002},
			},
		},
	}

	filter, err := buildWinDivertFilter(f, nil)
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and outbound and (( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.255 and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 )))) and ((( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.0 )? (( tcp.DstPort < 8000 or tcp.DstPort > 8002 ) or ( udp.DstPort < 8000 or udp.DstPort > 8002 )): true) and (( ip.DstAddr >= 1.1.1.1 and ip.DstAddr <= 1.1.1.1 )? (( tcp.DstPort < 8000 or tcp.DstPort > 8002 ) or ( udp.DstPort < 8000 or udp.DstPort > 8002 )): true))", filter)
}

func TestWinDivertBuildFilterMultipleIncludes(t *testing.T) {
	net1, err := ParseCIDR("1.1.1.1/24")
	require.NoError(t, err)
	net2, err := ParseCIDR("1.1.2.1/24")
	require.NoError(t, err)
	exemptNet, err := ParseCIDR("1.1.1.0")
	require.NoError(t, err)
	f := Filter{
		Include: []NetWithPortRange{
			{
				Net:       *net1,
				Comment:   "",
				PortRange: PortRange{From: 8000, To: 8002},
			},
			{
				Net:       *net2,
				Comment:   "",
				PortRange: PortRange{From: 8000, To: 8002},
			},
		},
		Exclude: []NetWithPortRange{
			{
				Net:       *exemptNet,
				Comment:   "",
				PortRange: PortRange{From: 8000, To: 8002},
			},
		},
	}

	filter, err := buildWinDivertFilter(f, nil)
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and outbound and (( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.255 and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 ))) or ( ip.DstAddr >= 1.1.2.0 and ip.DstAddr <= 1.1.2.255 and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 )))) and ((( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.0 )? (( tcp.DstPort < 8000 or tcp.DstPort > 8002 ) or ( udp.DstPort < 8000 or udp.DstPort > 8002 )): true))", filter)
}

func TestWinDivertBuildFilterOnlyExclude(t *testing.T) {
	excludeNet, err := ParseCIDR("1.1.1.14")
	require.NoError(t, err)
	f := Filter{
		Exclude: []NetWithPortRange{
			{
				Net:       *excludeNet,
				Comment:   "",
				PortRange: PortRange{From: 8000, To: 8002},
			},
		},
	}

	filter, err := buildWinDivertFilter(f, nil)
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and outbound and ((( ip.DstAddr >= 1.1.1.14 and ip.DstAddr <= 1.1.1.14 )? (( tcp.DstPort < 8000 or tcp.DstPort > 8002 ) or ( udp.DstPort < 8000 or udp.DstPort > 8002 )): true))", filter)
}

func TestWinDivertBuildFilterInterfaces(t *testing.T) {
	filter, err := buildWinDivertFilter(Filter{}, []int{1, 2, 3})
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and outbound and (ifIdx == 1 or ifIdx == 2 or ifIdx == 3)", filter)
}

func TestWinDivertBuildFilterInterfacesAndInclude(t *testing.T) {
	excludeNet, err := ParseCIDR("1.1.1.14")
	require.NoError(t, err)
	f := Filter{
		Exclude: []NetWithPortRange{
			{
				Net:       *excludeNet,
				Comment:   "",
				PortRange: PortRange{From: 8000, To: 8002},
			},
		},
	}

	filter, err := buildWinDivertFilter(f, []int{1, 2, 3})
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and outbound and (ifIdx == 1 or ifIdx == 2 or ifIdx == 3) and ((( ip.DstAddr >= 1.1.1.14 and ip.DstAddr <= 1.1.1.14 )? (( tcp.DstPort < 8000 or tcp.DstPort > 8002 ) or ( udp.DstPort < 8000 or udp.DstPort > 8002 )): true))", filter)
}
