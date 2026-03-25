// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package network

import (
	"fmt"
	"reflect"
	"strings"
)

type TcpResetOpts struct {
	Filter
	ExecutionContext
	Interfaces []string
}

func (o *TcpResetOpts) ToExecutionContext() ExecutionContext {
	return o.ExecutionContext
}

func (o *TcpResetOpts) DoesConflictWith(opts Opts) bool {
	other, ok := opts.(*TcpResetOpts)
	if !ok {
		return true
	}
	if !reflect.DeepEqual(o.Filter, other.Filter) {
		return true
	}
	return !reflect.DeepEqual(o.Interfaces, other.Interfaces)
}

func (o *TcpResetOpts) IpCommands(_ Family, _ Mode) ([]string, error) {
	return nil, nil
}

func (o *TcpResetOpts) TcCommands(_ Mode) ([]string, error) {
	return nil, nil
}

const steadybitTcpResetChain = "STEADYBIT_TCP_RESET"

func (o *TcpResetOpts) IptablesScripts(mode Mode) (v4 []string, v6 []string, err error) {
	v4Script, err := o.iptablesScript(mode, false)
	if err != nil {
		return nil, nil, err
	}
	v6Script, err := o.iptablesScript(mode, true)
	if err != nil {
		return nil, nil, err
	}
	return v4Script, v6Script, nil
}

func (o *TcpResetOpts) iptablesScript(mode Mode, ipv6 bool) ([]string, error) {
	filter := optimizeFilter(o.Filter)

	// If no includes match this IP family there is nothing to install or remove.
	if !tcpResetFilterHasFamily(filter.Include, ipv6) {
		return nil, nil
	}

	ifcs := o.Interfaces
	if len(ifcs) == 0 {
		ifcs = []string{""}
	}

	if mode == ModeAdd {
		return tcpResetAddScript(filter, ifcs, ipv6)
	}
	return tcpResetDeleteScript(ifcs), nil
}

// tcpResetFilterHasFamily reports whether any entry in nwps belongs to the requested IP family.
func tcpResetFilterHasFamily(nwps []NetWithPortRange, ipv6 bool) bool {
	for _, nwp := range nwps {
		family, err := getFamily(nwp.Net)
		if err != nil {
			continue
		}
		if (family == FamilyV6) == ipv6 {
			return true
		}
	}
	return false
}

// tcpResetAddScript builds an iptables-restore script that:
//  1. Creates the STEADYBIT_TCP_RESET chain.
//  2. Jumps to it from OUTPUT, INPUT, and FORWARD (optionally restricted to named interfaces).
//  3. Populates the chain with ACCEPT rules for excluded nets (before REJECT rules so they win).
//  4. Populates the chain with REJECT --reject-with tcp-reset rules for included nets,
//     matching both --dport (arriving at) and --sport (departing from) directions so that
//     port-forwarded traffic (e.g. Kubernetes NodePort) is also caught.
func tcpResetAddScript(filter Filter, ifcs []string, ipv6 bool) ([]string, error) {
	script := []string{
		"*filter",
		fmt.Sprintf(":%s - [0:0]", steadybitTcpResetChain),
	}

	// Jump rules: steer matching traffic into the custom chain.
	// For FORWARD with an explicit interface we add both -i and -o variants so that
	// both inbound (to service) and outbound (responses) directions are covered.
	for _, ifc := range ifcs {
		outFlag, inFlag := ifaceFlags(ifc)
		script = append(script,
			fmt.Sprintf("-A OUTPUT%s -j %s", outFlag, steadybitTcpResetChain),
			fmt.Sprintf("-A INPUT%s -j %s", inFlag, steadybitTcpResetChain),
		)
		if ifc == "" {
			script = append(script, fmt.Sprintf("-A FORWARD -j %s", steadybitTcpResetChain))
		} else {
			script = append(script,
				fmt.Sprintf("-A FORWARD%s -j %s", inFlag, steadybitTcpResetChain),
				fmt.Sprintf("-A FORWARD%s -j %s", outFlag, steadybitTcpResetChain),
			)
		}
	}

	// ACCEPT rules for excluded nets (must precede REJECT rules).
	for _, nwp := range filter.Exclude {
		family, err := getFamily(nwp.Net)
		if err != nil {
			return nil, err
		}
		if (family == FamilyV6) != ipv6 {
			continue
		}
		net := nwp.Net.String()
		port := tcpResetPortRange(nwp.PortRange)
		script = append(script,
			fmt.Sprintf("-A %s -p tcp -d %s --dport %s -j ACCEPT", steadybitTcpResetChain, net, port),
			fmt.Sprintf("-A %s -p tcp -s %s --sport %s -j ACCEPT", steadybitTcpResetChain, net, port),
		)
	}

	// REJECT rules for included nets.
	for _, nwp := range filter.Include {
		family, err := getFamily(nwp.Net)
		if err != nil {
			return nil, err
		}
		if (family == FamilyV6) != ipv6 {
			continue
		}
		net := nwp.Net.String()
		port := tcpResetPortRange(nwp.PortRange)
		script = append(script,
			fmt.Sprintf("-A %s -p tcp -d %s --dport %s -j REJECT --reject-with tcp-reset", steadybitTcpResetChain, net, port),
			fmt.Sprintf("-A %s -p tcp -s %s --sport %s -j REJECT --reject-with tcp-reset", steadybitTcpResetChain, net, port),
		)
	}

	script = append(script, "COMMIT")
	return script, nil
}

// tcpResetDeleteScript builds an iptables-restore script that removes the jump rules
// added by tcpResetAddScript, then flushes and deletes the STEADYBIT_TCP_RESET chain.
// Because the chain is flushed wholesale, individual rule details are not needed here.
func tcpResetDeleteScript(ifcs []string) []string {
	script := []string{"*filter"}

	for _, ifc := range ifcs {
		outFlag, inFlag := ifaceFlags(ifc)
		script = append(script,
			fmt.Sprintf("-D OUTPUT%s -j %s", outFlag, steadybitTcpResetChain),
			fmt.Sprintf("-D INPUT%s -j %s", inFlag, steadybitTcpResetChain),
		)
		if ifc == "" {
			script = append(script, fmt.Sprintf("-D FORWARD -j %s", steadybitTcpResetChain))
		} else {
			script = append(script,
				fmt.Sprintf("-D FORWARD%s -j %s", inFlag, steadybitTcpResetChain),
				fmt.Sprintf("-D FORWARD%s -j %s", outFlag, steadybitTcpResetChain),
			)
		}
	}

	script = append(script,
		fmt.Sprintf("-F %s", steadybitTcpResetChain),
		fmt.Sprintf("-X %s", steadybitTcpResetChain),
		"COMMIT",
	)
	return script
}

// ifaceFlags returns the iptables -o and -i flag strings for the given interface name.
// Returns empty strings when ifc is empty (no interface restriction).
func ifaceFlags(ifc string) (outFlag, inFlag string) {
	if ifc == "" {
		return "", ""
	}
	return fmt.Sprintf(" -o %s", ifc), fmt.Sprintf(" -i %s", ifc)
}

// tcpResetPortRange formats a PortRange for use in iptables --dport/--sport arguments.
// iptables uses colon as the range separator (e.g. "8000:8999") rather than a dash.
func tcpResetPortRange(p PortRange) string {
	if p.From == p.To {
		return fmt.Sprintf("%d", p.From)
	}
	return fmt.Sprintf("%d:%d", p.From, p.To)
}

func (o *TcpResetOpts) String() string {
	var sb strings.Builder
	sb.WriteString("resetting tcp connections")
	if len(o.Interfaces) > 0 {
		sb.WriteString(" (interfaces: ")
		sb.WriteString(strings.Join(o.Interfaces, ", "))
		sb.WriteString(")")
	}
	writeStringForFilters(&sb, o.Filter)
	return sb.String()
}
