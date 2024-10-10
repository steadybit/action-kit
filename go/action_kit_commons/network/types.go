// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package network

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"net"
	"slices"
	"strconv"
	"strings"
)

type Mode string
type Family string

const (
	ModeAdd    Mode   = "add"
	ModeDelete Mode   = "del"
	FamilyV4   Family = "inet"
	FamilyV6   Family = "inet6"
)

var (
	NetAny = []net.IPNet{
		{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)},
		{IP: net.IPv6zero, Mask: net.CIDRMask(0, 128)},
	}
)

type Opts interface {
	IpCommands(family Family, mode Mode) ([]string, error)
	TcCommands(mode Mode) ([]string, error)
	String() string
}

type Filter struct {
	Include []NetWithPortRange
	Exclude []NetWithPortRange
}

var (
	PortRangeAny = PortRange{From: 1, To: 65534}
)

type PortRange struct {
	From uint16
	To   uint16
}

func (p *PortRange) String() string {
	if p.From == p.To {
		return strconv.Itoa(int(p.From))
	}
	return fmt.Sprintf("%d-%d", p.From, p.To)
}

func (p *PortRange) Overlap(other PortRange) bool {
	return p.From <= other.To && p.To >= other.From
}

func (p *PortRange) Contains(port uint16) bool {
	return p.From <= port && p.To >= port
}

func ParsePortRange(raw string) (PortRange, error) {
	parts := strings.Split(raw, "-")
	if len(parts) > 2 {
		return PortRange{}, fmt.Errorf("invalid port range \"%s\": invalid syntax", raw)
	}

	if parts[0] == "*" {
		return PortRangeAny, nil
	}

	from, err := strconv.Atoi(parts[0])
	if err != nil {
		return PortRange{}, fmt.Errorf("invalid port range \"%s\": invalid syntax", raw)
	}

	to := from
	if len(parts) == 2 && parts[1] != "" {
		to, err = strconv.Atoi(parts[1])
		if err != nil {
			return PortRange{}, fmt.Errorf("invalid port range \"%s\": invalid syntax", raw)
		}
	}

	if from < 1 || to > 65534 || from > to {
		return PortRange{}, fmt.Errorf("invalid port range \"%s\": not in range 1-65534", raw)
	}

	return PortRange{From: uint16(from), To: uint16(to)}, nil
}

func ParseCIDRs(raw []string) ([]net.IPNet, []string) {
	var cidrs []net.IPNet
	var nonCidrs []string

	for _, r := range raw {
		if len(r) == 0 {
			continue
		}

		if cidr, err := ParseCIDR(r); err == nil {
			cidrs = append(cidrs, *cidr)
		} else {
			nonCidrs = append(nonCidrs, r)
		}
	}

	return cidrs, nonCidrs
}

func ParseCIDR(s string) (*net.IPNet, error) {
	if _, cidr, err := net.ParseCIDR(s); err == nil {
		return cidr, nil
	}

	if ip := net.ParseIP(strings.TrimPrefix(strings.TrimSuffix(s, "]"), "[")); ip != nil {
		if cidr := IpToNet(ip); cidr != nil {
			return cidr, nil
		}
	}
	return nil, &net.ParseError{Type: "CIDR address", Text: s}
}

var (
	ipV4SingleAddressMask = net.CIDRMask(32, 32)
	ipV6SingleAddressMask = net.CIDRMask(128, 128)
)

func IpToNet(ip net.IP) *net.IPNet {
	if v4 := ip.To4(); v4 != nil {
		return &net.IPNet{IP: v4, Mask: ipV4SingleAddressMask}
	} else if v6 := ip.To16(); v6 != nil {
		return &net.IPNet{IP: v6, Mask: ipV6SingleAddressMask}
	}
	return nil
}

func IpsToNets(ips []net.IP) []net.IPNet {
	var nets []net.IPNet
	for _, ip := range ips {
		if n := IpToNet(ip); n != nil {
			nets = append(nets, *n)
		}
	}
	return nets
}

type NetWithPortRange struct {
	Net       net.IPNet
	PortRange PortRange
	Comment   string
}

func mustParseNetWithPortRange(net, port string) NetWithPortRange {
	parsedNet, err := ParseCIDR(net)
	if err != nil {
		panic(err)
	}
	parsedPort, err := ParsePortRange(port)
	if err != nil {
		panic(err)
	}
	return NetWithPortRange{Net: *parsedNet, PortRange: parsedPort}
}

func (nwp NetWithPortRange) String() string {
	var sb strings.Builder
	sb.WriteString(nwp.Net.String())
	if nwp.PortRange != PortRangeAny {
		sb.WriteString(" ")
		sb.WriteString(nwp.PortRange.String())
	}
	if nwp.Comment != "" {
		sb.WriteString(" # ")
		sb.WriteString(nwp.Comment)
	}
	return sb.String()
}

func (nwp NetWithPortRange) Overlap(other NetWithPortRange) bool {
	return (nwp.PortRange.Overlap(other.PortRange) && nwp.Net.Contains(other.Net.IP)) || (other.PortRange.Overlap(nwp.PortRange) && other.Net.Contains(nwp.Net.IP))
}

// Contains checks if the given NetWithPortRange is contained in the current NetWithPortRange
func (nwp NetWithPortRange) Contains(other NetWithPortRange) bool {
	return nwp.PortRange.Contains(other.PortRange.To) &&
		nwp.PortRange.Contains(other.PortRange.From) &&
		nwp.Net.Contains(other.Net.IP)
}

func NewNetWithPortRanges(nets []net.IPNet, portRanges ...PortRange) []NetWithPortRange {
	var result []NetWithPortRange
	for _, n := range nets {
		for _, portRange := range portRanges {
			result = append(result, NetWithPortRange{
				Net:       n,
				PortRange: portRange,
			})
		}
	}
	return result
}

func deduplicateNetWithPortRange(nwps []NetWithPortRange) []NetWithPortRange {
	slices.SortFunc(nwps, CompareNetWithPortRange)

	var deduplicated []NetWithPortRange
	for _, nwp := range nwps {
		found := false
		for i, o := range deduplicated {
			if o.Contains(nwp) {
				found = true
				log.Trace().Msgf("Deduplicating %s with %s", nwp, o)
				break
			} else if nwp.Contains(o) {
				log.Trace().Msgf("Deduplicating %s with %s", o, nwp)
				deduplicated[i] = nwp
				found = true
				break
			}
		}
		if !found {
			deduplicated = append(deduplicated, nwp)
		}
	}

	return deduplicated
}

func CompareNetWithPortRange(a, b NetWithPortRange) int {
	if a.Net.String() < b.Net.String() {
		return -1
	}
	if a.Net.String() > b.Net.String() {
		return 1
	}
	if a.PortRange.From < b.PortRange.From {
		return -1
	}
	if a.PortRange.From > b.PortRange.From {
		return 1
	}
	if a.PortRange.To < b.PortRange.To {
		return -1
	}
	if a.PortRange.To > b.PortRange.To {
		return 1
	}
	return 0
}
