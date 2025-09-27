// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package network

import (
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/rs/zerolog/log"
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

func optimizeFilter(f Filter) Filter {
	include := deduplicateNetWithPortRange(f.Include)
	exclude := deduplicateNetWithPortRange(necessaryExcludes(f.Exclude, include))
	return Filter{Include: include, Exclude: exclude}
}

func tcCommandsForFilter(mode Mode, f Filter, ifc string) ([]string, error) {
	var cmds []string
	if filterCmds, err := tcCommandsForNets(f.Exclude, mode, ifc, "1:", handleExclude, len(cmds)); err == nil {
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

func tcCommandsForDelayFilter(mode Mode, f Filter, ifc string, tcpPshOnly bool) ([]string, error) {
	var cmds []string
	if filterCmds, err := tcCommandsForDelayNets(f.Exclude, mode, ifc, "1:", handleExclude, len(cmds), tcpPshOnly); err == nil {
		cmds = append(cmds, filterCmds...)
	} else {
		return nil, err
	}

	if filterCmds, err := tcCommandsForDelayNets(f.Include, mode, ifc, "1:", handleInclude, len(cmds), tcpPshOnly); err == nil {
		cmds = append(cmds, filterCmds...)
	} else {
		return nil, err
	}

	return cmds, nil
}

// necessaryExcludes returns the excludes, which are overlapping with one of the includes.
func necessaryExcludes(excludes []NetWithPortRange, includes []NetWithPortRange) []NetWithPortRange {
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
	for _, nwp := range netWithPortRanges {
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

func tcCommandsForDelayNets(netWithPortRanges []NetWithPortRange, mode Mode, ifc, parent, flowId string, prio int, tcpPshOnly bool) ([]string, error) {
	var cmds []string
	for _, nwp := range netWithPortRanges {
		protocol, err := getProtocol(nwp.Net)
		if err != nil {
			return nil, err
		}

		matchers, err := getMatchersForDelay(nwp, tcpPshOnly)
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

func getMatchersForDelay(nwp NetWithPortRange, tcpPshOnly bool) ([]string, error) {
	family, err := getFamily(nwp.Net)
	if err != nil {
		return nil, err
	}

	var selector, protoTcp, protoUdp string
	var tcpFlagsOffset int
	switch family {
	case FamilyV4:
		selector = "ip"
		protoTcp = "match ip protocol 6 0xff"
		protoUdp = "match ip protocol 17 0xff"
		// TCP flags offset calculation assumes:
		// - Standard 20-byte IPv4 header with no IP options
		// - Standard TCP header with flags at byte 13
		tcpFlagsOffset = 33 // 20 bytes IPv4 header + 13 bytes to TCP flags
	case FamilyV6:
		selector = "ip6"
		protoTcp = "match ip6 nexthdr 6 0xff"
		protoUdp = "match ip6 nexthdr 17 0xff"
		// TCP flags offset calculation assumes:
		// - Standard 40-byte IPv6 header with no extension headers
		// - Standard TCP header with flags at byte 13
		tcpFlagsOffset = 53 // 40 bytes IPv6 header + 13 bytes to TCP flags
	default:
		return nil, fmt.Errorf("unknown family %s", family)
	}

	var matchers []string
	for _, pr := range getMask(nwp.PortRange) {
		if tcpPshOnly {
			// TCP PSH: src direction
			matchers = append(matchers, fmt.Sprintf("%s match %s src %s match u16 %s at %d match u8 0x08 0x08 at %d",
				protoTcp, selector, nwp.Net.String(), pr, getTcpSrcPortOffset(family), tcpFlagsOffset))
			// TCP PSH: dst direction
			matchers = append(matchers, fmt.Sprintf("%s match %s dst %s match u16 %s at %d match u8 0x08 0x08 at %d",
				protoTcp, selector, nwp.Net.String(), pr, getTcpDstPortOffset(family), tcpFlagsOffset))
			// UDP pass-through (delay all UDP too)
			matchers = append(matchers, fmt.Sprintf("%s match %s src %s match u16 %s at %d",
				protoUdp, selector, nwp.Net.String(), pr, getTcpSrcPortOffset(family)))
			matchers = append(matchers, fmt.Sprintf("%s match %s dst %s match u16 %s at %d",
				protoUdp, selector, nwp.Net.String(), pr, getTcpDstPortOffset(family)))
		} else {
			// Normal filtering without PSH flag
			matchers = append(matchers, fmt.Sprintf("match %s src %s match %s sport %s", selector, nwp.Net.String(), selector, pr))
			matchers = append(matchers, fmt.Sprintf("match %s dst %s match %s dport %s", selector, nwp.Net.String(), selector, pr))
		}
	}
	return matchers, nil
}

// getTcpSrcPortOffset returns the byte offset for TCP source port in the packet.
//
// IMPORTANT ASSUMPTIONS:
// - IPv4: Assumes standard 20-byte IP header with no IP options
// - IPv6: Assumes standard 40-byte IPv6 header with no extension headers
// - TCP: Assumes standard TCP header with source port at offset 0
// - These offsets are used by tc u32 classifier for packet matching
// - If IP options or IPv6 extension headers are present, these offsets will be incorrect
func getTcpSrcPortOffset(family Family) int {
	switch family {
	case FamilyV4:
		return 20 // IP header length (20 bytes) + 0 (TCP src port offset)
	case FamilyV6:
		return 40 // IPv6 header length (40 bytes) + 0 (TCP src port offset)
	default:
		return 0
	}
}

// getTcpDstPortOffset returns the byte offset for TCP destination port in the packet.
//
// IMPORTANT ASSUMPTIONS:
// - IPv4: Assumes standard 20-byte IP header with no IP options
// - IPv6: Assumes standard 40-byte IPv6 header with no extension headers
// - TCP: Assumes standard TCP header with destination port at offset 2
// - These offsets are used by tc u32 classifier for packet matching
// - If IP options or IPv6 extension headers are present, these offsets will be incorrect
func getTcpDstPortOffset(family Family) int {
	switch family {
	case FamilyV4:
		return 22 // IP header length (20 bytes) + 2 (TCP dst port offset)
	case FamilyV6:
		return 42 // IPv6 header length (40 bytes) + 2 (TCP dst port offset)
	default:
		return 0
	}
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
	if len(f.Exclude) > 0 {
		sb.WriteString("but not from/to:\n")
		for _, exc := range f.Exclude {
			sb.WriteString(" ")
			sb.WriteString(exc.String())
			sb.WriteString("\n")
		}
	}
}

func ComputeExcludesForOwnIpAndPorts(port, healthPort uint16) []NetWithPortRange {
	ownIps := GetOwnIPs()
	nets := IpsToNets(ownIps)

	log.Debug().Msgf("Adding own ip %s to exclude list (Ports %d and %d)", ownIps, port, healthPort)

	var exclHealth, exclPort []NetWithPortRange
	rPort := PortRange{From: port, To: port}
	if healthPort > 0 && healthPort != port {
		rHealth := PortRange{From: healthPort, To: healthPort}
		if rPort.IsNeighbor(rHealth) {
			rPort = rPort.Merge(rHealth)
		} else {
			exclHealth = NewNetWithPortRanges(nets, rHealth)
			for i := range exclHealth {
				exclHealth[i].Comment = "ext. health port"
			}
		}
	}

	exclPort = NewNetWithPortRanges(nets, rPort)
	for i := range exclPort {
		exclPort[i].Comment = "ext. port"
	}

	return append(exclPort, exclHealth...)
}
