// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package networkutils

import (
	"fmt"
	"strings"
)

type PackageLossOpts struct {
	Filter
	Loss       uint
	Interfaces []string
}

func (o *PackageLossOpts) IpCommands(_ Family, _ Mode) ([]string, error) {
	return nil, nil
}

func (o *PackageLossOpts) TcCommands(mode Mode) ([]string, error) {
	var cmds []string

	for _, ifc := range o.Interfaces {
		cmds = append(cmds, fmt.Sprintf("qdisc %s dev %s root handle 1: prio", mode, ifc))
		cmds = append(cmds, fmt.Sprintf("qdisc %s dev %s parent %s handle 30: netem loss random %d%%", mode, ifc, handleInclude, o.Loss))

		filterCmds, err := tcCommandsForFilter(mode, &o.Filter, ifc)
		if err != nil {
			return nil, err
		}
		cmds = append(cmds, filterCmds...)
	}
	reorderForMode(cmds, mode)
	return cmds, nil
}

func (o *PackageLossOpts) String() string {
	var sb strings.Builder
	sb.WriteString("Loosing traffic (loss: ")
	sb.WriteString(fmt.Sprintf("%d%%", o.Loss))
	sb.WriteString(", Interfaces: ")
	sb.WriteString(strings.Join(o.Interfaces, ", "))
	sb.WriteString(")")
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
