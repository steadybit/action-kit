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
	// BlockLocalhost additionally blocks matching loopback traffic (127.0.0.0/8, ::1).
	// The kernel's default `pref 0 lookup local` rule resolves loopback traffic before
	// any blackhole rule is evaluated, so it is replaced for the duration of the attack
	// by a `not to <loopback> lookup local` rule above and a `to <loopback> lookup local`
	// rule below the blackhole rules. Non-loopback traffic keeps being resolved by the
	// local table first, exactly as without this option.
	BlockLocalhost bool
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

	if o.BlockLocalhost != other.BlockLocalhost {
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

		if o.BlockLocalhost {
			// the local table is no longer consulted first, so excluded traffic
			// addressed to this host must be resolved there before falling
			// through to the main table.
			cmds = append(cmds, fmt.Sprintf("rule %s to %s%s%s table local", mode, net.String(), ipprotoSelector, portSelector("dport", portRange)))
			cmds = append(cmds, fmt.Sprintf("rule %s from %s%s%s table local", mode, net.String(), ipprotoSelector, portSelector("sport", portRange)))
		}
		cmds = append(cmds, fmt.Sprintf("rule %s to %s%s%s table main", mode, net.String(), ipprotoSelector, portSelector("dport", portRange)))
		cmds = append(cmds, fmt.Sprintf("rule %s from %s%s%s table main", mode, net.String(), ipprotoSelector, portSelector("sport", portRange)))
	}
	reorderForMode(cmds, mode)

	if o.BlockLocalhost {
		cmds = wrapWithLocalTableRules(cmds, family, mode)
	}
	return cmds, nil
}

// wrapWithLocalTableRules moves the kernel's default `pref 0 lookup local` rule
// out of the way so loopback traffic reaches the blackhole rules. Rules without
// an explicit preference are inserted above all existing ones (except pref 0),
// so the add order below yields:
//
//	not to <loopback> lookup local   (non-loopback traffic behaves as before)
//	<blackhole / exclude rules>
//	to <loopback> lookup local       (loopback traffic not blocked above)
//
// and finally drops pref 0. The revert re-adds pref 0 first so loopback traffic
// is never left without a route, then removes the attack's rules.
func wrapWithLocalTableRules(cmds []string, family family, mode mode) []string {
	loopback := loopbackNet(family)
	above := fmt.Sprintf("not to %s lookup local", loopback)
	below := fmt.Sprintf("to %s lookup local", loopback)

	if mode == modeAdd {
		result := []string{fmt.Sprintf("rule add %s", below)}
		result = append(result, cmds...)
		return append(result, fmt.Sprintf("rule add %s", above), "rule del pref 0 lookup local")
	}

	result := []string{"rule add pref 0 lookup local", fmt.Sprintf("rule del %s", above)}
	result = append(result, cmds...)
	return append(result, fmt.Sprintf("rule del %s", below))
}

func loopbackNet(family family) string {
	if family == familyV6 {
		return "::1/128"
	}
	return "127.0.0.0/8"
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
	if o.BlockLocalhost {
		sb.WriteString("(including localhost) ")
	}
	writeStringForFilters(&sb, o.Filter)
	return sb.String()
}
