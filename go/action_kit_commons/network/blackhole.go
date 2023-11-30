// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package network

import (
	"fmt"
	"strings"
)

type BlackholeOpts struct {
	Filter
}

func (o *BlackholeOpts) IpCommands(family Family, mode Mode) ([]string, error) {
	var cmds []string

	for _, nwp := range uniqueNetWithPortRange(o.Include) {
		net := nwp.Net
		portRange := nwp.PortRange

		if netFamily, err := getFamily(net); err != nil {
			return nil, err
		} else if netFamily != family {
			continue
		}

		cmds = append(cmds, fmt.Sprintf("rule %s blackhole to %s dport %s", mode, net.String(), portRange.String()))
		cmds = append(cmds, fmt.Sprintf("rule %s blackhole from %s sport %s", mode, net.String(), portRange.String()))
	}

	for _, nwp := range uniqueNetWithPortRange(o.Exclude) {
		net := nwp.Net
		portRange := nwp.PortRange

		if netFamily, err := getFamily(net); err != nil {
			return nil, err
		} else if netFamily != family {
			continue
		}

		cmds = append(cmds, fmt.Sprintf("rule %s to %s dport %s table main", mode, net.String(), portRange.String()))
		cmds = append(cmds, fmt.Sprintf("rule %s from %s sport %s table main", mode, net.String(), portRange.String()))
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
