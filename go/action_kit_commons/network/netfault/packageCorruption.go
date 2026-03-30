// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package netfault

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/rs/zerolog/log"
)

type CorruptPackagesOpts struct {
	Filter
	ExecutionContext
	Corruption uint
	Interfaces []string
}

func (o *CorruptPackagesOpts) toExecutionContext() ExecutionContext {
	return o.ExecutionContext
}

func (o *CorruptPackagesOpts) doesConflictWith(opts Opts) bool {
	other, ok := opts.(*CorruptPackagesOpts)

	if !ok {
		return true
	}

	if o.Corruption != other.Corruption {
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

func (o *CorruptPackagesOpts) ipCommands(_ family, _ mode) ([]string, error) {
	return nil, nil
}

func (o *CorruptPackagesOpts) tcCommands(mode mode) ([]string, error) {
	var cmds []string

	filter := optimizeFilter(o.Filter)
	for _, ifc := range o.Interfaces {
		cmds = append(cmds, fmt.Sprintf("qdisc %s dev %s root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0", mode, ifc))
		cmds = append(cmds, fmt.Sprintf("qdisc %s dev %s parent %s handle 30: netem corrupt %d%%", mode, ifc, handleInclude, o.Corruption))

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

func (o *CorruptPackagesOpts) String() string {
	var sb strings.Builder
	sb.WriteString("corrupting packages of ")
	sb.WriteString(fmt.Sprintf("%d%%", o.Corruption))
	sb.WriteString(" (interfaces: ")
	sb.WriteString(strings.Join(o.Interfaces, ", "))
	sb.WriteString(")")
	writeStringForFilters(&sb, optimizeFilter(o.Filter))
	return sb.String()
}
