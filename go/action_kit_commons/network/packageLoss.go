// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package network

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/rs/zerolog/log"
)

type PackageLossOpts struct {
	Filter
	ExecutionContext
	Loss       uint
	Interfaces []string
}

func (o *PackageLossOpts) ToExecutionContext() ExecutionContext {
	return o.ExecutionContext
}

func (o *PackageLossOpts) DoesConflictWith(opts Opts) bool {
	other, ok := opts.(*PackageLossOpts)

	if !ok {
		return true
	}

	if o.Loss != other.Loss {
		return true
	}

	if !reflect.DeepEqual(o.Filter, other.Filter) {
		return true
	}

	if !reflect.DeepEqual(o.Interfaces, other.Interfaces) {
		return true
	}

	return false
}
func (o *PackageLossOpts) IpCommands(_ Family, _ Mode) ([]string, error) {
	return nil, nil
}

func (o *PackageLossOpts) TcCommands(mode Mode) ([]string, error) {
	var cmds []string

	filter := optimizeFilter(o.Filter)
	for _, ifc := range o.Interfaces {
		cmds = append(cmds, fmt.Sprintf("qdisc %s dev %s root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0", mode, ifc))
		cmds = append(cmds, fmt.Sprintf("qdisc %s dev %s parent %s handle 30: netem loss random %d%%", mode, ifc, handleInclude, o.Loss))

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

func (o *PackageLossOpts) String() string {
	var sb strings.Builder
	sb.WriteString("loosing packages of ")
	sb.WriteString(fmt.Sprintf("%d%%", o.Loss))
	sb.WriteString("(interfaces: ")
	sb.WriteString(strings.Join(o.Interfaces, ", "))
	sb.WriteString(")")
	writeStringForFilters(&sb, optimizeFilter(o.Filter))
	return sb.String()
}
