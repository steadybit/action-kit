// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package network

import (
	"fmt"
	"io"
	"net"
	"strings"
)

func reorderForMode(cmds []string, mode Mode) {
	if mode == ModeDelete {
		for i, j := 0, len(cmds)-1; i < j; i, j = i+1, j-1 {
			cmds[i], cmds[j] = cmds[j], cmds[i]
		}
	}
}

func ToReader(strs []string) io.Reader {
	return strings.NewReader(fmt.Sprintf("%s\n", strings.Join(strs, "\n")))
}

func getFamily(net net.IPNet) (Family, error) {
	switch {
	case net.IP.To4() != nil:
		return FamilyV4, nil
	case net.IP.To16() != nil:
		return FamilyV6, nil
	default:
		return "", fmt.Errorf("unknown family for %s", net)
	}
}

const handleExclude = "1:1"
const handleInclude = "1:3"

func tcCommandsForFilter(mode Mode, f *Filter, ifc string) ([]string, error) {
	var cmds []string
	if filterCmds, err := tcCommandsForNets(NecessaryExcludes(f.Exclude, f.Include), mode, ifc, "1:", handleExclude, len(cmds)); err == nil {
		cmds = append(cmds, filterCmds...)
	} else {
		return nil, err
	}

	if filterCmds, err := tcCommandsForNets(f.Include, mode, ifc, "1:", handleInclude, len(cmds)); err == nil {
		cmds = append(cmds, filterCmds...)
	} else {
		return nil, err
	}

	return cmds, nil
}

// NecessaryExcludes returns those excludes, which are overlapping with one of the includes.
func NecessaryExcludes(excludes []NetWithPortRange, includes []NetWithPortRange) []NetWithPortRange {
	result := make([]NetWithPortRange, 0, len(excludes))
	for _, exclude := range excludes {
		for _, include := range includes {
			if include.Overlap(exclude) {
				result = append(result, exclude)
				break
			}
		}
	}
	return result
}

func tcCommandsForNets(netWithPortRanges []NetWithPortRange, mode Mode, ifc, parent, flowId string, prio int) ([]string, error) {
	var cmds []string
	for _, nwp := range uniqueNetWithPortRange(netWithPortRanges) {
		protocol, err := getProtocol(nwp.Net)
		if err != nil {
			return nil, err
		}

		matchers, err := getMatchers(nwp)
		if err != nil {
			return nil, err
		}

		for _, matcher := range matchers {
			prio += 1
			cmds = append(cmds, fmt.Sprintf("filter %s dev %s protocol %s parent %s prio %d u32 %s flowid %s", mode, ifc, protocol, parent, prio, matcher, flowId))
		}
	}
	return cmds, nil
}

func getMatchers(nwp NetWithPortRange) ([]string, error) {
	family, err := getFamily(nwp.Net)
	if err != nil {
		return nil, err
	}

	var selector string
	switch family {
	case FamilyV4:
		selector = "ip"
	case FamilyV6:
		selector = "ip6"
	default:
		return nil, fmt.Errorf("unknown family %s", family)
	}

	var matchers []string
	for _, pr := range getMask(nwp.PortRange) {
		matchers = append(matchers, fmt.Sprintf("match %s src %s match %s sport %s", selector, nwp.Net.String(), selector, pr))
		matchers = append(matchers, fmt.Sprintf("match %s dst %s match %s dport %s", selector, nwp.Net.String(), selector, pr))
	}
	return matchers, nil
}

const portMaxValue uint16 = 0xffff

func getMask(r PortRange) []string {
	if r == PortRangeAny {
		return []string{"0 0x0000"}
	} else if r.From == r.To {
		return []string{fmt.Sprintf("%d 0xffff", r.From)}
	}

	var masks []string
	if r.To <= r.From || r.To > portMaxValue {
		return masks
	}

	port := r.From
	for port <= r.To {
		mask := portMask(port, r.To)
		masks = append(masks, fmt.Sprintf("%d %#x", port, mask))
		maxPort := maxPortForMask(port, mask)
		if maxPort == portMaxValue {
			break
		}
		port = maxPort + 1
	}

	return masks
}

func maxPortForMask(port, mask uint16) uint16 {
	maxValueInMask := portMaxValue - mask
	baseValue := port & mask
	return baseValue + maxValueInMask
}

func portMask(port, to uint16) uint16 {
	bit := uint16(1)
	mask := portMaxValue
	nextMask := portMaxValue
	effective := port & nextMask

	maxPort := maxPortForMask(effective, portMaxValue)

	for effective != 0 && maxPort < to {
		effective = port & nextMask
		if effective < port {
			break
		}
		maxPort = maxPortForMask(effective, nextMask)
		if maxPort <= to {
			mask = nextMask
		}
		nextMask -= bit
		bit <<= 1
	}

	return mask
}

func getProtocol(net net.IPNet) (string, error) {
	family, err := getFamily(net)
	if err != nil {
		return "", err
	}

	switch family {
	case FamilyV4:
		return "ip", nil
	case FamilyV6:
		return "ipv6", nil
	default:
		return "", fmt.Errorf("unknown family %s", family)
	}
}

func writeStringForFilters(sb *strings.Builder, f Filter) {
	sb.WriteString("\nto/from:\n")
	for _, inc := range f.Include {
		sb.WriteString(" ")
		sb.WriteString(inc.String())
		sb.WriteString("\n")
	}
	excludes := NecessaryExcludes(f.Exclude, f.Include)
	if len(excludes) > 0 {
		sb.WriteString("but not from/to:\n")
		for _, exc := range excludes {
			sb.WriteString(" ")
			sb.WriteString(exc.String())
			sb.WriteString("\n")
		}
	}
}
