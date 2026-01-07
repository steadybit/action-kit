// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package network

import (
	"fmt"
	"reflect"
	"strings"
)

type IpProto string

const IpProtoTcp IpProto = "tcp"
const IpProtoUdp IpProto = "udp"

type BlackholeOpts struct {
	Filter
	ExecutionContext
	IpProto IpProto
}

func (o *BlackholeOpts) ToExecutionContext() ExecutionContext {
	return o.ExecutionContext
}

func (o *BlackholeOpts) DoesConflictWith(opts Opts) bool {
	other, ok := opts.(*BlackholeOpts)

	if !ok {
		return true
	}

	if o.IpProto != other.IpProto {
		return true
	}

	if !reflect.DeepEqual(o.Filter, other.Filter) {
		return true
	}

	return false
}

func (o *BlackholeOpts) IpCommands(family Family, mode Mode) ([]string, error) {
	var cmds []string

	ipprotoSelector := ""
	if len(o.IpProto) > 0 {
		ipprotoSelector = fmt.Sprintf(" ipproto %s", o.IpProto)
	}

	filter := optimizeFilter(o.Filter)
	for _, nwp := range filter.Include {
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

	for _, nwp := range filter.Exclude {
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
