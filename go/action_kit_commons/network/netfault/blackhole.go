// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package netfault

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/steadybit/action-kit/go/action_kit_commons/network"
)

type IpProto string

const IpProtoTcp IpProto = "tcp"
const IpProtoUdp IpProto = "udp"

type BlackholeOpts struct {
	Filter
	ExecutionContext
	IpProto IpProto
}

func (o *BlackholeOpts) toExecutionContext() ExecutionContext {
	return o.ExecutionContext
}

func (o *BlackholeOpts) doesConflictWith(opts Opts) bool {
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

func (o *BlackholeOpts) ipCommands(family family, mode mode) ([]string, error) {
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

		cmds = append(cmds, fmt.Sprintf("rule %s blackhole to %s%s%s", mode, net.String(), ipprotoSelector, portSelector("dport", portRange)))
		cmds = append(cmds, fmt.Sprintf("rule %s blackhole from %s%s%s", mode, net.String(), ipprotoSelector, portSelector("sport", portRange)))
	}

	for _, nwp := range filter.Exclude {
		net := nwp.Net
		portRange := nwp.PortRange

		if netFamily, err := getFamily(net); err != nil {
			return nil, err
		} else if netFamily != family {
			continue
		}

		cmds = append(cmds, fmt.Sprintf("rule %s to %s%s%s table main", mode, net.String(), ipprotoSelector, portSelector("dport", portRange)))
		cmds = append(cmds, fmt.Sprintf("rule %s from %s%s%s table main", mode, net.String(), ipprotoSelector, portSelector("sport", portRange)))
	}
	reorderForMode(cmds, mode)
	return cmds, nil
}

// portSelector renders the ` dport <range>` / ` sport <range>` fragment for an
// ip rule. For PortRangeAny it returns an empty string so the rule matches all
// traffic regardless of L4 port — otherwise portless protocols such as ICMP
// would never match a `dport`/`sport` selector and escape the blackhole.
func portSelector(keyword string, portRange network.PortRange) string {
	if portRange.IsAny() {
		return ""
	}
	return fmt.Sprintf(" %s %s", keyword, portRange.String())
}

func (o *BlackholeOpts) String() string {
	var sb strings.Builder
	sb.WriteString("blocking traffic ")
	writeStringForFilters(&sb, o.Filter)
	return sb.String()
}
