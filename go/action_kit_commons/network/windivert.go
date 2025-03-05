package network

import (
	"fmt"
	"net"
	"strings"
	"text/template"
)

func getStartEndIP(ipNet net.IPNet) (net.IP, net.IP, error) {
	family, err := getFamily(ipNet)

	if err != nil {
		return nil, nil, err
	}

	if family == FamilyV4 {
		startIp := ipNet.IP.Mask(ipNet.Mask)

		invertedMask := make(net.IP, len(startIp.To4()))

		for i := range invertedMask {
			invertedMask[i] = ^ipNet.Mask[i]
		}

		endIp := make(net.IP, len(startIp.To4()))
		startIpTo4 := startIp.To4()

		for i := range endIp {
			endIp[i] = startIpTo4[i] | invertedMask[i]
		}

		return startIp, endIp, nil
	}

	if family == FamilyV6 {
		startIp := ipNet.IP.Mask(ipNet.Mask)

		invertedMask := make(net.IP, len(startIp.To16()))

		for i := range invertedMask {
			invertedMask[i] = ^ipNet.Mask[i]
		}

		endIp := make(net.IP, len(startIp.To16()))
		startIpTo16 := startIp.To16()

		for i := range endIp {
			endIp[i] = startIpTo16[i] | invertedMask[i]
		}

		return startIp, endIp, nil
	}

	return nil, nil, fmt.Errorf("not implemented")
}

func setCorrectReplacements(replacements *map[string]string, family Family) {
	if family == FamilyV4 {
		(*replacements)["ipDstAddr"] = "ip.DstAddr"
		(*replacements)["ipSrcAddr"] = "ip.SrcAddr"
	} else {
		(*replacements)["ipDstAddr"] = "ipv6.DstAddr"
		(*replacements)["ipSrcAddr"] = "ipv6.SrcAddr"
	}
}

func buildWinDivertFilter(filter Filter) (string, error) {
	var sb strings.Builder

	replaceMap := map[string]string{
		"tcpDstPort": "tcp.DstPort",
		"udpDstPort": "udp.DstPort",
	}

	portTemplate := "(( {{.tcpDstPort}} >= %d and {{.tcpDstPort}} <= %d ) or ( {{.udpDstPort}} >= %d and {{.udpDstPort}} <= %d ))"
	portTemplateExclude := "(( {{.tcpDstPort}} < %d or {{.tcpDstPort}} > %d ) or ( {{.udpDstPort}} < %d or {{.udpDstPort}} > %d ))"

	sb.WriteString("(tcp or udp) and outbound and ")

	if len(filter.Include) > 0 {
		sb.WriteString("(")
		for i, ran := range filter.Include {
			family, err := getFamily(ran.Net)

			if err != nil {
				return "", err
			}

			setCorrectReplacements(&replaceMap, family)
			portFilter := fmt.Sprintf(portTemplate, ran.PortRange.From, ran.PortRange.To, ran.PortRange.From, ran.PortRange.To)
			startIp, endIp, err := getStartEndIP(ran.Net)

			if err != nil {
				return "", err
			}

			config := fmt.Sprintf("( {{.ipDstAddr}} >= %s and {{.ipDstAddr}} <= %s and %s)", startIp.String(), endIp.String(), portFilter)

			tmpl, err := template.New("filter").Parse(config)

			if err != nil {
				return "", err
			}

			tmpl.Execute(&sb, replaceMap)

			if i < len(filter.Include)-1 {
				sb.WriteString(" or ")
			}
		}
		sb.WriteString(")")
	}

	if len(filter.Include) > 0 && len(filter.Exclude) > 0 {
		sb.WriteString(" and ")
	}

	if len(filter.Exclude) > 0 {
		sb.WriteString("(")
		for i, ran := range filter.Exclude {
			family, err := getFamily(ran.Net)

			if err != nil {
				return "", err
			}

			setCorrectReplacements(&replaceMap, family)
			portFilter := fmt.Sprintf(portTemplateExclude, ran.PortRange.From, ran.PortRange.To, ran.PortRange.From, ran.PortRange.To)
			startIp, endIp, err := getStartEndIP(ran.Net)

			if err != nil {
				return "", err
			}

			config := fmt.Sprintf("(( {{.ipDstAddr}} >= %s and {{.ipDstAddr}} <= %s )? %s: true)",
				startIp.String(), endIp.String(), portFilter)

			tmpl, err := template.New("filter").Parse(config)

			if err != nil {
				return "", err
			}

			tmpl.Execute(&sb, replaceMap)

			if i < len(filter.Exclude)-1 {
				sb.WriteString(" and ")
			}
		}
		sb.WriteString(")")
	}

	return sb.String(), nil
}
