// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package networkutils

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"strings"
)

type CorruptPackagesOpts struct {
	Filter
	Corruption uint
	Interfaces []string
}

func (o *CorruptPackagesOpts) IpCommands(_ Family, _ Mode) (io.Reader, error) {
	return nil, nil
}

func (o *CorruptPackagesOpts) TcCommands(mode Mode) (io.Reader, error) {
	var cmds []string

	for _, ifc := range o.Interfaces {
		cmds = append(cmds, fmt.Sprintf("qdisc %s dev %s root handle 1: prio", mode, ifc))
		cmds = append(cmds, fmt.Sprintf("qdisc %s dev %s parent %s handle 30: netem corrupt %d%%", mode, ifc, handleInclude, o.Corruption))

		filterCmds, err := tcCommandsForFilter(mode, &o.Filter, ifc)
		if err != nil {
			return nil, err
		}
		cmds = append(cmds, filterCmds...)
	}

	log.Debug().Strs("commands", cmds).Msg("generated tc commands")
	return toReader(cmds, mode)
}

func (o *CorruptPackagesOpts) String() string {
	var sb strings.Builder
	sb.WriteString("Corrupting Packages (corruption: ")
	sb.WriteString(fmt.Sprintf("%d%%", o.Corruption))
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
