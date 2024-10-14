// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package network

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"regexp"
	"strings"
)

type LimitBandwidthOpts struct {
	Filter
	Bandwidth  string
	Interfaces []string
}

func (o *LimitBandwidthOpts) IpCommands(_ Family, _ Mode) ([]string, error) {
	return nil, nil
}

func (o *LimitBandwidthOpts) TcCommands(mode Mode) ([]string, error) {
	var cmds []string

	expression, err := regexp.Compile("^[0-7]bit$")
	if err != nil {
		return nil, err
	}
	if expression.MatchString(o.Bandwidth) {
		return nil, fmt.Errorf("TC does not support rate settings below 8bit/s. (%s)", o.Bandwidth)
	}

	filter := optimizeFilter(o.Filter)
	for _, ifc := range o.Interfaces {
		cmds = append(cmds, fmt.Sprintf("qdisc %s dev %s root handle 1: htb default 30", mode, ifc))
		cmds = append(cmds, fmt.Sprintf("class %s dev %s parent 1: classid %s htb rate %s", mode, ifc, handleInclude, o.Bandwidth))

		filterCmds, err := tcCommandsForFilter(mode, filter, ifc)
		if err != nil {
			return nil, err
		}
		cmds = append(cmds, filterCmds...)
	}
	reorderForMode(cmds, mode)

	if len(cmds)/len(o.Interfaces) > maxTcCommands {
		log.Trace().Strs("cmds", cmds).Msg("too many tc commands")
		return nil, &ErrTooManyTcCommands{Count: len(cmds)}
	}
	return cmds, nil
}

func (o *LimitBandwidthOpts) String() string {
	var sb strings.Builder
	sb.WriteString("limit bandwidth to ")
	sb.WriteString(o.Bandwidth)
	sb.WriteString(" (interfaces: ")
	sb.WriteString(strings.Join(o.Interfaces, ", "))
	sb.WriteString(")")
	writeStringForFilters(&sb, optimizeFilter(o.Filter))
	return sb.String()
}
