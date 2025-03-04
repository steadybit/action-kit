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

func TestWinDivertBuildFilter1(t *testing.T) {
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
	filter, err := buildWinDivertFilter(f)
	assert.NoError(t, err)
	assert.Equal(t, "(( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.255 and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 ))))", filter)
}

func TestWinDivertBuildFilter2(t *testing.T) {
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
	filter, err := buildWinDivertFilter(f)
	assert.NoError(t, err)
	assert.Equal(t, "(( ipv6.DstAddr >= :: and ipv6.DstAddr <= ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 ))))", filter)
}

func TestWinDivertBuildFilter3(t *testing.T) {
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
	filter, err := buildWinDivertFilter(f)
	assert.NoError(t, err)

	assert.Equal(t, "(( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.255 and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 )))) and ((( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.0 )? (( not tcp.DstPort >= 8000 and not tcp.DstPort <= 8002 ) and ( not udp.DstPort >= 8000 and not udp.DstPort <= 8002 )): true))", filter)
}

func TestWinDivertBuildFilter4(t *testing.T) {
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
	filter, err := buildWinDivertFilter(f)
	assert.NoError(t, err)

	assert.Equal(t, "(( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.255 and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 ))))"+
		" and ((( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.0 )? (( not tcp.DstPort >= 8000 and not tcp.DstPort <= 8002 ) and ( not udp.DstPort >= 8000 and not udp.DstPort <= 8002 )): true)"+
		" and (( ip.DstAddr >= 1.1.1.1 and ip.DstAddr <= 1.1.1.1 )? (( not tcp.DstPort >= 8000 and not tcp.DstPort <= 8002 ) and ( not udp.DstPort >= 8000 and not udp.DstPort <= 8002 )): true))", filter)
}

func TestWinDivertBuildFilter5(t *testing.T) {
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
	filter, err := buildWinDivertFilter(f)
	assert.NoError(t, err)

	assert.Equal(t, "(( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.255 and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 )))"+
		" or ( ip.DstAddr >= 1.1.2.0 and ip.DstAddr <= 1.1.2.255 and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 ))))"+
		" and ((( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.0 )? (( not tcp.DstPort >= 8000 and not tcp.DstPort <= 8002 ) and ( not udp.DstPort >= 8000 and not udp.DstPort <= 8002 )): true))", filter)
}

func TestWinDivertBuildFilter6(t *testing.T) {
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
	filter, err := buildWinDivertFilter(f)
	assert.NoError(t, err)

	assert.Equal(t, "((( ip.DstAddr >= 1.1.1.14 and ip.DstAddr <= 1.1.1.14 )? (( not tcp.DstPort >= 8000 and not tcp.DstPort <= 8002 ) and ( not udp.DstPort >= 8000 and not udp.DstPort <= 8002 )): true))", filter)
}
