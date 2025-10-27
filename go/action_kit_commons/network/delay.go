// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package network

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type DelayOpts struct {
	Filter
	Delay      time.Duration
	Jitter     time.Duration
	Interfaces []string
	// When true, only delay TCP packets with PSH flag set. Uses iptables marks + tc fw filter.
	TcpPshOnly bool
}

func (o *DelayOpts) IpCommands(_ Family, _ Mode) ([]string, error) {
	return nil, nil
}

const steadybitDelayFwMark uint32 = 0x1

func (o *DelayOpts) IptablesScripts(mode Mode) ([]string, []string, error) {
	if !o.TcpPshOnly {
		return nil, nil, nil
	}

	filter := optimizeFilter(o.Filter)

	switch mode {
	case ModeAdd:
		v4 := buildIptablesScript(filter, false)
		v6 := buildIptablesScript(filter, true)
		return v4, v6, nil
	case ModeDelete:
		script := ipTablesDeleteScript
		return script, script, nil
	default:
		return nil, nil, fmt.Errorf("unsupported mode: %s", mode)
	}
}

var ipTablesHeader = []string{
	"*mangle",
	":STEADYBIT_DELAY - [0:0]",
	"-A OUTPUT -j STEADYBIT_DELAY",
	"-A POSTROUTING -j STEADYBIT_DELAY",
}

var ipTablesDeleteScript = []string{
	"*mangle",
	"-D OUTPUT -j STEADYBIT_DELAY",
	"-D POSTROUTING -j STEADYBIT_DELAY",
	"-F STEADYBIT_DELAY",
	"-X STEADYBIT_DELAY",
	"COMMIT",
}

func buildIptablesScript(f Filter, v6 bool) []string {
	script := make([]string, 0, 10)
	script = append(script, ipTablesHeader...)
	script = append(script, writeIptablesRules(f.Exclude, v6, true)...)
	script = append(script, writeIptablesRules(f.Include, v6, false)...)
	script = append(script, "COMMIT")
	return script
}

func writeIptablesRules(nwps []NetWithPortRange, v6 bool, isExclude bool) []string {
	rules := make([]string, 0, len(nwps))
	for _, nwp := range nwps {
		if shouldIncludeRule(nwp.Net, v6) {
			rules = append(rules,
				// We emit two rules: one for dst/dport and one for src/sport to match to/from.
				buildSingleIptables(nwp, true, isExclude),
				buildSingleIptables(nwp, false, isExclude),
			)
		}
	}
	return rules
}

func shouldIncludeRule(net net.IPNet, v6 bool) bool {
	fam, err := getFamily(net)
	if err != nil {
		return false
	}
	return (v6 && fam == FamilyV6) || (!v6 && fam == FamilyV4)
}

func buildSingleIptables(nwp NetWithPortRange, isDst bool, isExclude bool) string {
	// Base: chain and proto/flags
	var sb strings.Builder
	sb.WriteString("-A STEADYBIT_DELAY -p tcp --tcp-flags PSH PSH ")

	// Address match
	if isDst {
		sb.WriteString("-d ")
	} else {
		sb.WriteString("-s ")
	}
	sb.WriteString(nwp.Net.String())
	sb.WriteString(" ")

	// Port match if not any
	if nwp.PortRange != PortRangeAny {
		if isDst {
			sb.WriteString("--dport ")
		} else {
			sb.WriteString("--sport ")
		}
		if nwp.PortRange.From == nwp.PortRange.To {
			sb.WriteString(fmt.Sprintf("%d ", nwp.PortRange.From))
		} else {
			sb.WriteString(fmt.Sprintf("%d:%d ", nwp.PortRange.From, nwp.PortRange.To))
		}
	}

	// For exclude we jump to RETURN early, for include we MARK.
	if isExclude {
		sb.WriteString("-j RETURN")
	} else {
		sb.WriteString(fmt.Sprintf("-j MARK --set-mark %#x", steadybitDelayFwMark))
	}
	return sb.String()
}

func (o *DelayOpts) TcCommands(mode Mode) ([]string, error) {
	var cmds []string

	filter := optimizeFilter(o.Filter)
	for _, ifc := range o.Interfaces {
		cmds = append(cmds, fmt.Sprintf("qdisc %s dev %s root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0", mode, ifc))
		cmds = append(cmds, fmt.Sprintf("qdisc %s dev %s parent %s handle 30: netem delay %dms %dms", mode, ifc, handleInclude, o.Delay.Milliseconds(), o.Jitter.Milliseconds()))

		if o.TcpPshOnly {
			// When using PSH-only path, rely on fwmark created by iptables and a single tc fw filter.
			// Add both IPv4 and IPv6 protocol filters to match the mark.
			cmds = append(cmds, fmt.Sprintf("filter %s dev %s protocol ip parent 1: prio 1 handle %#x fw flowid %s", mode, ifc, steadybitDelayFwMark, handleInclude))
			cmds = append(cmds, fmt.Sprintf("filter %s dev %s protocol ipv6 parent 1: prio 2 handle %#x fw flowid %s", mode, ifc, steadybitDelayFwMark, handleInclude))
		} else {
			filterCmds, err := tcCommandsForFilter(mode, filter, ifc)
			if err != nil {
				return nil, err
			}
			cmds = append(cmds, filterCmds...)
		}
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
	sb.WriteString(", tcpPshOnly: ")
	sb.WriteString(fmt.Sprintf("%t", o.TcpPshOnly))
	sb.WriteString(")")
	writeStringForFilters(&sb, optimizeFilter(o.Filter))
	return sb.String()
}
