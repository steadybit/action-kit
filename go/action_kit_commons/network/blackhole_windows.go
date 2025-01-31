package network

import (
	"fmt"
	"strings"
)

type IpProto string

const IpProtoTcp IpProto = "TCP"
const IpProtoUdp IpProto = "UDP"

type BlackholeOpts struct {
	Filter
	IpProto IpProto
}

func (o *BlackholeOpts) FwCommands(family Family, mode Mode) ([]string, error) {
	var cmds []string

	filter := optimizeFilter(o.Filter)
	for _, nwp := range filter.Include {
		net := nwp.Net
		portRange := nwp.PortRange

		if netFamily, err := getFamily(net); err != nil {
			return nil, err
		} else if netFamily != family {
			continue
		}

		remoteAddress := ""

		if net.IP.Equal(NetAnyIpv4.IP) {
			remoteAddress = fmt.Sprintf("-RemoteAddress %s", remoteAddress)
		}

		if mode == ModeAdd {
			cmds = append(cmds, fmt.Sprintf("New-NetFirewallRule -Name STEADYBIT_%s_%d_%d_%s -Direction Outbound -RemotePort %d-%d %s", family, portRange.From, portRange.To, net.IP, portRange.From, portRange.To, remoteAddress))
		} else if mode == ModeDelete {
			cmds = append(cmds, fmt.Sprintf("Remove-NetFirewallRule -Name STEADYBIT_%s_%d_%d_%s", family, portRange.From, portRange.To, net.IP))
		}
	}

	for _, nwp := range filter.Exclude {
		net := nwp.Net
		portRange := nwp.PortRange

		if netFamily, err := getFamily(net); err != nil {
			return nil, err
		} else if netFamily != family {
			continue
		}

		remoteAddress := ""

		if net.IP.Equal(NetAnyIpv4.IP) {
			remoteAddress = fmt.Sprintf("-RemoteAddress %s", remoteAddress)
		}

		if mode == ModeAdd {
			cmds = append(cmds, fmt.Sprintf("New-NetFirewallRule -Name STEADYBIT_%s_%d_%d_%s -Direction Outbound -RemotePort %d-%d %s", family, portRange.From, portRange.To, net.IP, portRange.From, portRange.To, remoteAddress))
		} else if mode == ModeDelete {
			cmds = append(cmds, fmt.Sprintf("Remove-NetFirewallRule -Name STEADYBIT_%s_%d_%d_%s", family, portRange.From, portRange.To, net.IP))
		}
	}

	return cmds, nil
}

func (o *BlackholeOpts) QoSCommands(mode Mode) ([]string, error) {
	return nil, nil
}

func (o *BlackholeOpts) String() string {
	var sb strings.Builder
	sb.WriteString("blocking traffic ")
	writeStringForFilters(&sb, o.Filter)
	return sb.String()
}
