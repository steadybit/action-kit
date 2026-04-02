// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package netfault

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/steadybit/action-kit/go/action_kit_commons/network"
)

type TcpResetOpts struct {
	Filter
	ExecutionContext
	Interfaces  []string
	InsertAtTop bool
}

func (o *TcpResetOpts) chainName() string {
	id := o.ExecutionContext.TargetExecutionId
	if len(id) > 12 {
		id = id[len(id)-12:]
	}
	if id == "" {
		id = "default"
	}
	return fmt.Sprintf("SB_TCP_RST_%s", id)
}

func (o *TcpResetOpts) toExecutionContext() ExecutionContext {
	return o.ExecutionContext
}

func (o *TcpResetOpts) doesConflictWith(opts Opts) bool {
	other, ok := opts.(*TcpResetOpts)
	if !ok {
		return true
	}
	if !reflect.DeepEqual(o.Filter, other.Filter) {
		return true
	}
	return !reflect.DeepEqual(o.Interfaces, other.Interfaces)
}

func (o *TcpResetOpts) ipCommands(_ family, _ mode) ([]string, error) {
	return nil, nil
}

func (o *TcpResetOpts) tcCommands(_ mode) ([]string, error) {
	return nil, nil
}

func (o *TcpResetOpts) iptablesScripts(mode mode) (v4 []string, v6 []string, err error) {
	v4Script, err := o.iptablesScript(mode, familyV4)
	if err != nil {
		return nil, nil, err
	}
	v6Script, err := o.iptablesScript(mode, familyV6)
	if err != nil {
		return nil, nil, err
	}
	return v4Script, v6Script, nil
}

func (o *TcpResetOpts) iptablesScript(mode mode, fam family) ([]string, error) {
	filter := optimizeFilter(o.Filter)

	if !tcpResetFilterHasFamily(filter.Include, fam) {
		return nil, nil
	}

	ifcs := o.Interfaces
	if len(ifcs) == 0 {
		ifcs = []string{""}
	}

	chain := o.chainName()
	if mode == modeAdd {
		return tcpResetAddScript(chain, filter, ifcs, fam, o.InsertAtTop), nil
	}
	return tcpResetDeleteScript(chain, ifcs), nil
}

func tcpResetFilterHasFamily(nwps []network.NetWithPortRange, fam family) bool {
	for _, nwp := range nwps {
		if hasFamily(nwp, fam) {
			return true
		}
	}
	return false
}

func hasFamily(nwp network.NetWithPortRange, fam family) bool {
	f, _ := getFamily(nwp.Net)
	return f == fam
}

func tcpResetAddScript(chain string, filter Filter, ifcs []string, f family, insertAtTop bool) []string {
	script := []string{
		"*filter",
		fmt.Sprintf(":%s - [0:0]", chain),
	}

	jumpCmd := "-A"
	jumpPos := ""
	if insertAtTop {
		jumpCmd = "-I"
		jumpPos = " 1"
	}

	for _, ifc := range ifcs {
		outFlag, inFlag := ifaceFlags(ifc)
		script = append(script,
			fmt.Sprintf("%s OUTPUT%s%s -j %s", jumpCmd, jumpPos, outFlag, chain),
			fmt.Sprintf("%s INPUT%s%s -j %s", jumpCmd, jumpPos, inFlag, chain),
		)
		if ifc == "" {
			script = append(script, fmt.Sprintf("%s FORWARD%s -j %s", jumpCmd, jumpPos, chain))
		} else {
			script = append(script,
				fmt.Sprintf("%s FORWARD%s%s -j %s", jumpCmd, jumpPos, inFlag, chain),
				fmt.Sprintf("%s FORWARD%s%s -j %s", jumpCmd, jumpPos, outFlag, chain),
			)
		}
	}

	for _, nwp := range filter.Exclude {
		if !hasFamily(nwp, f) {
			continue
		}
		net := nwp.Net.String()
		port := tcpResetPortRange(nwp.PortRange)
		script = append(script,
			fmt.Sprintf("-A %s -p tcp -d %s --dport %s -j ACCEPT", chain, net, port),
			fmt.Sprintf("-A %s -p tcp -s %s --sport %s -j ACCEPT", chain, net, port),
		)
	}

	for _, nwp := range filter.Include {
		if !hasFamily(nwp, f) {
			continue
		}
		net := nwp.Net.String()
		port := tcpResetPortRange(nwp.PortRange)
		script = append(script,
			fmt.Sprintf("-A %s -p tcp -d %s --dport %s -j REJECT --reject-with tcp-reset", chain, net, port),
			fmt.Sprintf("-A %s -p tcp -s %s --sport %s -j REJECT --reject-with tcp-reset", chain, net, port),
		)
	}

	script = append(script, "COMMIT")
	return script
}

func tcpResetDeleteScript(chain string, ifcs []string) []string {
	script := []string{"*filter"}

	for _, ifc := range ifcs {
		outFlag, inFlag := ifaceFlags(ifc)
		script = append(script,
			fmt.Sprintf("-D OUTPUT%s -j %s", outFlag, chain),
			fmt.Sprintf("-D INPUT%s -j %s", inFlag, chain),
		)
		if ifc == "" {
			script = append(script, fmt.Sprintf("-D FORWARD -j %s", chain))
		} else {
			script = append(script,
				fmt.Sprintf("-D FORWARD%s -j %s", inFlag, chain),
				fmt.Sprintf("-D FORWARD%s -j %s", outFlag, chain),
			)
		}
	}

	script = append(script,
		fmt.Sprintf("-F %s", chain),
		fmt.Sprintf("-X %s", chain),
		"COMMIT",
	)
	return script
}

func ifaceFlags(ifc string) (outFlag, inFlag string) {
	if ifc == "" {
		return "", ""
	}
	return fmt.Sprintf(" -o %s", ifc), fmt.Sprintf(" -i %s", ifc)
}

func tcpResetPortRange(p network.PortRange) string {
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
