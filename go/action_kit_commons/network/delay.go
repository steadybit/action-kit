// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package network

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type DelayOpts struct {
	Filter
	Delay      time.Duration
	Jitter     time.Duration
	Interfaces []string
	TcpPshOnly bool
}

func (o *DelayOpts) IpCommands(_ Family, _ Mode) ([]string, error) {
	return nil, nil
}

func (o *DelayOpts) TcCommands(mode Mode) ([]string, error) {
	var cmds []string

	filter := optimizeFilter(o.Filter)
	for _, ifc := range o.Interfaces {
		cmds = append(cmds, fmt.Sprintf("qdisc %s dev %s root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0", mode, ifc))
		cmds = append(cmds, fmt.Sprintf("qdisc %s dev %s parent %s handle 30: netem delay %dms %dms", mode, ifc, handleInclude, o.Delay.Milliseconds(), o.Jitter.Milliseconds()))

		filterCmds, err := tcCommandsForDelayFilter(mode, filter, ifc, o.TcpPshOnly)
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

func (o *DelayOpts) String() string {
	var sb strings.Builder
	sb.WriteString("delaying traffic by ")
	sb.WriteString(o.Delay.String())
	sb.WriteString(" (jitter: ")
	sb.WriteString(o.Jitter.String())
	sb.WriteString(", interfaces: ")
	sb.WriteString(strings.Join(o.Interfaces, ", "))
	sb.WriteString(")")
	writeStringForFilters(&sb, optimizeFilter(o.Filter))
	return sb.String()
}
