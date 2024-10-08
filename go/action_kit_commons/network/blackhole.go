// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package network

import (
	"fmt"
	"strings"
)

type IpProto string

const IpProtoTcp IpProto = "tcp"
const IpProtoUdp IpProto = "udp"

type BlackholeOpts struct {
	Filter
	IpProto IpProto
}

func (o *BlackholeOpts) IpCommands(family Family, mode Mode) ([]string, error) {
	var cmds []string

	ipprotoSelector := ""
	if len(o.IpProto) > 0 {
		ipprotoSelector = fmt.Sprintf(" ipproto %s", o.IpProto)
	}

	for _, nwp := range uniqueNetWithPortRange(o.Include) {
		net := nwp.Net
		portRange := nwp.PortRange

		if netFamily, err := getFamily(net); err != nil {
			return nil, err
		} else if netFamily != family {
			continue
		}

		cmds = append(cmds, fmt.Sprintf("rule %s blackhole to %s%s dport %s", mode, net.String(), ipprotoSelector, portRange.String()))
		cmds = append(cmds, fmt.Sprintf("rule %s blackhole from %s%s sport %s", mode, net.String(), ipprotoSelector, portRange.String()))
	}

	for _, nwp := range uniqueNetWithPortRange(NecessaryExcludes(o.Exclude, o.Include)) {
		net := nwp.Net
		portRange := nwp.PortRange

		if netFamily, err := getFamily(net); err != nil {
			return nil, err
		} else if netFamily != family {
			continue
		}

		cmds = append(cmds, fmt.Sprintf("rule %s to %s%s dport %s table main", mode, net.String(), ipprotoSelector, portRange.String()))
		cmds = append(cmds, fmt.Sprintf("rule %s from %s%s sport %s table main", mode, net.String(), ipprotoSelector, portRange.String()))
	}
	reorderForMode(cmds, mode)
	return cmds, nil
}

func (o *BlackholeOpts) TcCommands(_ Mode) ([]string, error) {
	return nil, nil
}

func (o *BlackholeOpts) String() string {
	var sb strings.Builder
	sb.WriteString("blocking traffic ")
	writeStringForFilters(&sb, o.Filter)
	return sb.String()
}
