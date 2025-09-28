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
	// When true, only delay TCP packets with PSH flag set. Uses iptables marks + tc fw filter.
	TcpPshOnly bool
}

func (o *DelayOpts) IpCommands(_ Family, _ Mode) ([]string, error) {
	return nil, nil
}

const steadybitDelayFwMark uint32 = 0x1

// IptablesScripts implements optional iptables marking when TcpPshOnly is enabled.
// It returns iptables-restore scripts for IPv4 and IPv6.
func (o *DelayOpts) IptablesScripts(mode Mode) (string, string, error) {
	if !o.TcpPshOnly {
		return "", "", nil
	}

	filter := optimizeFilter(o.Filter)

	switch mode {
	case ModeAdd:
		v4 := buildIptablesScript(filter, false)
		v6 := buildIptablesScript(filter, true)
		return v4, v6, nil
	case ModeDelete:
		v4 := buildIptablesDeleteScript(false)
		v6 := buildIptablesDeleteScript(true)
		return v4, v6, nil
	default:
		return "", "", fmt.Errorf("unsupported mode: %s", mode)
	}
}

func buildIptablesScript(f Filter, v6 bool) string {
	// We create chain STEADYBIT_DELAY and jump from OUTPUT to it.
	// First add early RETURN rules for all excludes, then mark includes.
	// Only mark TCP packets with PSH flag set.
	var b strings.Builder
	b.WriteString("*mangle\n")
	b.WriteString(":STEADYBIT_DELAY - [0:0]\n")
	b.WriteString("-A OUTPUT -j STEADYBIT_DELAY\n")
	b.WriteString("-A POSTROUTING -j STEADYBIT_DELAY\n")

	for _, nwp := range f.Exclude {
		if fam, err := getFamily(nwp.Net); err == nil {
			if (v6 && fam == FamilyV6) || (!v6 && fam == FamilyV4) {
				b.WriteString(iptablesRule(nwp, true))
				b.WriteString("\n")
			}
		}
	}

	for _, nwp := range f.Include {
		if fam, err := getFamily(nwp.Net); err == nil {
			if (v6 && fam == FamilyV6) || (!v6 && fam == FamilyV4) {
				b.WriteString(iptablesRule(nwp, false))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("COMMIT\n")
	return b.String()
}

func buildIptablesDeleteScript(v6 bool) string {
	var b strings.Builder
	b.WriteString("*mangle\n")
	b.WriteString("-D OUTPUT -j STEADYBIT_DELAY\n")
	b.WriteString("-D POSTROUTING -j STEADYBIT_DELAY\n")
	b.WriteString("-F STEADYBIT_DELAY\n")
	b.WriteString("-X STEADYBIT_DELAY\n")
	b.WriteString("COMMIT\n")
	return b.String()
}

func ipFamilySelector(v6 bool) string {
	if v6 {
		return "ip6"
	}
	return "ip"
}

func iptablesRule(nwp FilteredNetWithPortRange, isExclude bool) string {
	return buildIptablesRulesFromNwp(nwp, isExclude)
}

// Helper struct to allow direction expansion; reuse NetWithPortRange directly.
type FilteredNetWithPortRange = NetWithPortRange

func buildIptablesRulesFromNwp(nwp NetWithPortRange, isExclude bool) string {
	// We emit two rules: one for dst/dport and one for src/sport to match to/from.
	// Example: -A STEADYBIT_DELAY -p tcp --tcp-flags PSH PSH -d 1.2.3.0/24 --dport 80 -j MARK --set-mark 0x1
	// For exclude we jump to RETURN early, for include we MARK.
	var rules []string
	dst := buildSingleIptables(nwp, true, isExclude)
	src := buildSingleIptables(nwp, false, isExclude)
	rules = append(rules, dst, src)
	return strings.Join(rules, "\n")
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
