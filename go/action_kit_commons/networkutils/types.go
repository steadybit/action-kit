// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package networkutils

import (
	"errors"
	"fmt"
	"net"
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

func ParsePortRange(raw string) (PortRange, error) {
	parts := strings.Split(raw, "-")
	if len(parts) > 2 {
		return PortRange{}, errors.New("invalid port range")
	}

	from, err := strconv.Atoi(parts[0])
	if err != nil {
		return PortRange{}, err
	}

	to := from
	if len(parts) == 2 && parts[1] != "" {
		to, err = strconv.Atoi(parts[1])
		if err != nil {
			return PortRange{}, err
		}
	}

	if from < 1 || to > 65534 || from > to {
		return PortRange{}, errors.New("invalid port range")
	}

	return PortRange{From: uint16(from), To: uint16(to)}, nil
}

type NetWithPortRange struct {
	Net       net.IPNet
	PortRange PortRange
}

func IpToNet(ips []string) []net.IPNet {
	var nets []net.IPNet
	for _, ip := range ips {
		addr := net.ParseIP(ip)
		if v4 := addr.To4(); v4 != nil {
			nets = append(nets, net.IPNet{IP: v4, Mask: net.CIDRMask(32, 32)})
		} else if v6 := addr.To16(); v6 != nil {
			nets = append(nets, net.IPNet{IP: v6, Mask: net.CIDRMask(128, 128)})
		}
	}
	return nets
}

func (nwp NetWithPortRange) String() string {
	if nwp.PortRange == PortRangeAny {
		return nwp.Net.String()
	}
	return fmt.Sprintf("%s port %s", nwp.Net.String(), nwp.PortRange.String())
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
