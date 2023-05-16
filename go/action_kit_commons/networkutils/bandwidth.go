// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package networkutils

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"strings"
)

type LimitBandwidthOpts struct {
	Filter
	Bandwidth  string
	Interfaces []string
}

func (o *LimitBandwidthOpts) IpCommands(_ Family, _ Mode) (io.Reader, error) {
	return nil, nil
}

func (o *LimitBandwidthOpts) TcCommands(mode Mode) (io.Reader, error) {
	var cmds []string

	for _, ifc := range o.Interfaces {
		cmds = append(cmds, fmt.Sprintf("qdisc %s dev %s root handle 1: htb default 30", mode, ifc))
		cmds = append(cmds, fmt.Sprintf("class %s dev %s parent 1: classid %s htb rate %s", mode, ifc, handleInclude, o.Bandwidth))

		filterCmds, err := tcCommandsForFilter(mode, &o.Filter, ifc)
		if err != nil {
			return nil, err
		}
		cmds = append(cmds, filterCmds...)
	}

	log.Debug().Strs("commands", cmds).Msg("generated tc commands")
	return toReader(cmds, mode)
}

func (o *LimitBandwidthOpts) String() string {
	var sb strings.Builder
	sb.WriteString("Limit bandwidth (bandwidth: ")
	sb.WriteString(o.Bandwidth)
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
