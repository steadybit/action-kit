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

const mangleMarkValue = "0x5B"

type TcpResetOpts struct {
	Filter
	ExecutionContext
	Interfaces    []string
	Prepend       bool
	UseMangleChain bool
}

type iptablesChainName string

const (
	iptablesChainNameInput      = "INPUT"
	iptablesChainNameOutput     = "OUTPUT"
	iptablesChainNameForward    = "FORWARD"
	iptablesChainNamePrerouting = "PREROUTING"
)

var (
	iptablesChainsMangle = []iptablesChainName{iptablesChainNameOutput, iptablesChainNamePrerouting, iptablesChainNameForward}
	iptablesChainsFilter = []iptablesChainName{iptablesChainNameOutput, iptablesChainNameInput, iptablesChainNameForward}
)

func (o *TcpResetOpts) rstFilterChainName() string {
	return fmt.Sprintf("SB_TCP_RST_%s", o.idSuffix())
}

func (o *TcpResetOpts) rstMangleChainName() string {
	return fmt.Sprintf("SB_TCP_RST_M_%s", o.idSuffix())
}

func (o *TcpResetOpts) idSuffix() string {
	id := o.ExecutionContext.TargetExecutionId
	if len(id) > 12 {
		id = id[len(id)-12:]
	}
	if id == "" {
		id = "default"
	}
	return id
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
	if o.UseMangleChain {
		return o.mangleMarkScripts(mode)
	}
	return o.filterOnlyScripts(mode)
}

func (o *TcpResetOpts) filterOnlyScripts(mode mode) (v4 []string, v6 []string, err error) {
	v4, err = o.filterOnlyScript(mode, familyV4)
	if err != nil {
		return nil, nil, err
	}
	v6, err = o.filterOnlyScript(mode, familyV6)
	if err != nil {
		return nil, nil, err
	}
	return v4, v6, nil
}

func (o *TcpResetOpts) filterOnlyScript(mode mode, fam family) ([]string, error) {
	filter := optimizeFilter(o.Filter)
	if !anyNetHasFamily(filter.Include, fam) {
		return nil, nil
	}
	if mode == modeAdd {
		chain := o.rstFilterChainName()

		script := []string{
			"*filter",
			fmt.Sprintf(":%s - [0:0]", chain),
		}

		cmd, pos := o.appendOrInsert()
		script = append(script, o.chainJumpRules(cmd, pos, iptablesChainsFilter, chain)...)
		script = append(script, excludeRules(chain, filter.Exclude, fam, "ACCEPT")...)
		script = append(script, includeRejectRules(chain, filter.Include, fam)...)
		script = append(script, "COMMIT")
		return script, nil
	}
	return o.tableDeleteScript("filter", o.rstFilterChainName(), iptablesChainsFilter), nil
}

// mangleMarkScripts generates a two-table script: mangle (MARK) + filter (REJECT on mark).
func (o *TcpResetOpts) mangleMarkScripts(mode mode) (v4 []string, v6 []string, err error) {
	v4, err = o.mangleMarkScript(mode, familyV4)
	if err != nil {
		return nil, nil, err
	}
	v6, err = o.mangleMarkScript(mode, familyV6)
	if err != nil {
		return nil, nil, err
	}
	return v4, v6, nil
}

func (o *TcpResetOpts) mangleMarkScript(mode mode, fam family) ([]string, error) {
	filter := optimizeFilter(o.Filter)
	if !anyNetHasFamily(filter.Include, fam) {
		return nil, nil
	}

	filterChain := o.rstFilterChainName()

	if mode == modeAdd {
		mangleChain := o.rstMangleChainName()
		cmd, pos := o.appendOrInsert()

		// mangle table: mark matching packets
		script := []string{
			"*mangle",
			fmt.Sprintf(":%s - [0:0]", mangleChain),
		}
		script = append(script, o.chainJumpRules(cmd, pos, iptablesChainsMangle, mangleChain)...)
		script = append(script, excludeRules(mangleChain, filter.Exclude, fam, "RETURN")...)
		script = append(script, includeMarkRules(mangleChain, filter.Include, fam)...)
		script = append(script, "COMMIT")

		// filter table: reject marked packets
		script = append(script,
			"*filter",
			fmt.Sprintf(":%s - [0:0]", filterChain),
		)
		script = append(script, o.chainJumpRules(cmd, pos, iptablesChainsFilter, filterChain)...)
		script = append(script, fmt.Sprintf("-A %s -p tcp -m mark --mark %s -j REJECT --reject-with tcp-reset", filterChain, mangleMarkValue))
		script = append(script, "COMMIT")

		return script, nil
	}
	mangleChain := o.rstMangleChainName()
	script := o.tableDeleteScript("mangle", mangleChain, iptablesChainsMangle)
	script = append(script, o.tableDeleteScript("filter", filterChain, iptablesChainsFilter)...)
	return script, nil
}

func (o *TcpResetOpts) tableDeleteScript(table, chain string, chains []iptablesChainName) []string {
	script := []string{fmt.Sprintf("*%s", table)}
	script = append(script, o.chainJumpRules("-D", "", chains, chain)...)
	script = append(script,
		fmt.Sprintf("-F %s", chain),
		fmt.Sprintf("-X %s", chain),
		"COMMIT",
	)
	return script
}

func (o *TcpResetOpts) interfacesOrAll() []string {
	if len(o.Interfaces) == 0 {
		return []string{""}
	}
	return o.Interfaces
}

func (o *TcpResetOpts) appendOrInsert() (cmd, pos string) {
	if o.Prepend {
		return "-I", " 1"
	}
	return "-A", ""
}

func excludeRules(chain string, excludes []network.NetWithPortRange, f family, target string) []string {
	var rules []string
	for _, nwp := range excludes {
		if !netHasFamily(nwp, f) {
			continue
		}
		net := nwp.Net.String()
		port := tcpResetPortRange(nwp.PortRange)
		rules = append(rules,
			fmt.Sprintf("-A %s -p tcp -d %s --dport %s -j %s", chain, net, port, target),
			fmt.Sprintf("-A %s -p tcp -s %s --sport %s -j %s", chain, net, port, target),
		)
	}
	return rules
}

func includeRejectRules(chain string, includes []network.NetWithPortRange, f family) []string {
	var rules []string
	for _, nwp := range includes {
		if !netHasFamily(nwp, f) {
			continue
		}
		net := nwp.Net.String()
		port := tcpResetPortRange(nwp.PortRange)
		rules = append(rules,
			fmt.Sprintf("-A %s -p tcp -d %s --dport %s -j REJECT --reject-with tcp-reset", chain, net, port),
			fmt.Sprintf("-A %s -p tcp -s %s --sport %s -j REJECT --reject-with tcp-reset", chain, net, port),
		)
	}
	return rules
}

func includeMarkRules(chain string, includes []network.NetWithPortRange, f family) []string {
	var rules []string
	for _, nwp := range includes {
		if !netHasFamily(nwp, f) {
			continue
		}
		net := nwp.Net.String()
		port := tcpResetPortRange(nwp.PortRange)
		rules = append(rules,
			fmt.Sprintf("-A %s -p tcp -d %s --dport %s -j MARK --set-mark %s", chain, net, port, mangleMarkValue),
			fmt.Sprintf("-A %s -p tcp -s %s --sport %s -j MARK --set-mark %s", chain, net, port, mangleMarkValue),
		)
	}
	return rules
}

func anyNetHasFamily(nwps []network.NetWithPortRange, fam family) bool {
	for _, nwp := range nwps {
		if netHasFamily(nwp, fam) {
			return true
		}
	}
	return false
}

func netHasFamily(nwp network.NetWithPortRange, fam family) bool {
	f, _ := getFamily(nwp.Net)
	return f == fam
}

func (o *TcpResetOpts) chainJumpRules(cmd, pos string, chains []iptablesChainName, targetChain string) []string {
	var rules []string
	for _, ifc := range o.interfacesOrAll() {
		var outFlag, inFlag string
		if ifc != "" {
			outFlag = fmt.Sprintf(" -o %s", ifc)
			inFlag = fmt.Sprintf(" -i %s", ifc)
		}

		for _, chain := range chains {
			switch chain {
			case "INPUT", "PREROUTING":
				rules = append(rules, fmt.Sprintf("%s %s%s%s -j %s", cmd, chain, pos, inFlag, targetChain))
			case "OUTPUT":
				rules = append(rules, fmt.Sprintf("%s %s%s%s -j %s", cmd, chain, pos, outFlag, targetChain))
			case "FORWARD":
				rules = append(rules, fmt.Sprintf("%s %s%s%s -j %s", cmd, chain, pos, inFlag, targetChain))
				if inFlag != outFlag {
					rules = append(rules, fmt.Sprintf("%s %s%s%s -j %s", cmd, chain, pos, outFlag, targetChain))
				}
			}
		}
	}
	return rules
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
	if o.UseMangleChain {
		sb.WriteString(" (mangle+filter mark")
	} else {
		sb.WriteString(" (filter")
	}
	if len(o.Interfaces) > 0 {
		sb.WriteString(", interfaces: ")
		sb.WriteString(strings.Join(o.Interfaces, ", "))
	}
	sb.WriteString(")")
	writeStringForFilters(&sb, o.Filter)
	return sb.String()
}
