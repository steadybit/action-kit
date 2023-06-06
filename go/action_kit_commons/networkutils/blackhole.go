// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package networkutils

import (
	"fmt"
	"strings"
)

type BlackholeOpts struct {
	Filter
}

func (o *BlackholeOpts) IpCommands(family Family, mode Mode) ([]string, error) {
	var cmds []string

	for _, nwp := range o.Include {
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

	for _, nwp := range o.Exclude {
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
	sb.WriteString("Blocking traffic ")
	sb.WriteString("\nto/from:\n")
	for _, inc := range o.Include {
		sb.WriteString(" ")
		sb.WriteString(inc.String())
		sb.WriteString("\n")
	}
	if len(o.Exclude) > 0 {
		sb.WriteString("but not from/to:\n")
		for _, exc := range o.Exclude {
			sb.WriteString(" ")
			sb.WriteString(exc.String())
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
