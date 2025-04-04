// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build windows

package network

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
	"net"
	"os"
	"strings"
	"text/template"
	"time"
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

func buildWinDivertFilter(filter Filter, ifIdxs []int) (string, error) {
	var sb strings.Builder

	replaceMap := map[string]string{
		"tcpDstPort": "tcp.DstPort",
		"udpDstPort": "udp.DstPort",
	}

	portTemplate := "(( {{.tcpDstPort}} >= %d and {{.tcpDstPort}} <= %d ) or ( {{.udpDstPort}} >= %d and {{.udpDstPort}} <= %d ))"
	portTemplateExclude := "(( {{.tcpDstPort}} < %d or {{.tcpDstPort}} > %d ) or ( {{.udpDstPort}} < %d or {{.udpDstPort}} > %d ))"

	sb.WriteString("(tcp or udp) and outbound")

	if len(ifIdxs) > 0 {
		sb.WriteString(" and (")
		ifIdxStatements := make([]string, len(ifIdxs))
		for i, ifIdx := range ifIdxs {
			ifIdxStatements[i] = fmt.Sprintf("ifIdx == %d", ifIdx)
		}
		sb.WriteString(strings.Join(ifIdxStatements, " or "))
		sb.WriteString(")")
	}

	if len(filter.Include) > 0 {
		sb.WriteString(" and (")
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

			err = tmpl.Execute(&sb, replaceMap)
			if err != nil {
				return "", err
			}

			if i < len(filter.Include)-1 {
				sb.WriteString(" or ")
			}
		}
		sb.WriteString(")")
	}

	if len(filter.Exclude) > 0 {
		sb.WriteString(" and (")
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

			err = tmpl.Execute(&sb, replaceMap)
			if err != nil {
				return "", err
			}

			if i < len(filter.Exclude)-1 {
				sb.WriteString(" and ")
			}
		}
		sb.WriteString(")")
	}

	return sb.String(), nil
}

func buildWinDivertFilterFile(filter Filter) (string, error) {
	return buildWinDivertFilterFileWithInterfaces(filter, nil)
}

func buildWinDivertFilterFileWithInterfaces(filter Filter, interfaces []int) (string, error) {
	filterContent, err := buildWinDivertFilter(filter, interfaces)
	if err != nil {
		return "", err
	}

	tempFile, err := os.CreateTemp("", "wdna-filter-*.txt")
	if err != nil {
		return "", err
	}
	defer func(tempFile *os.File) {
		_ = tempFile.Close()
	}(tempFile)

	_, err = tempFile.Write([]byte(filterContent))
	if err != nil {
		return "", err
	}
	return tempFile.Name(), nil
}

func awaitWinDivertServiceStatus(state svc.State, timeout time.Duration) error {
	// wait until the windivert service reports successful startup or an error occurred
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func(m *mgr.Mgr) {
		_ = m.Disconnect()
	}(m)

	end := time.Now().Add(timeout)
	for time.Now().Before(end) {
		s, err := m.OpenService("windivert")
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		// deferred function is only created once
		//goland:noinspection GoDeferInLoop
		defer func(s *mgr.Service) {
			_ = s.Close()
		}(s)

		for time.Now().Before(end) {
			status, err := s.Query()
			if err == nil && status.State == state {
				log.Debug().Int("state", int(status.State)).Msgf("windivert service reached state %d", state)
				return nil
			}
			//goland:noinspection GoDfaErrorMayBeNotNil
			log.Debug().Int("state", int(status.State)).Msgf("windivert service not yet in state %d", state)
			time.Sleep(100 * time.Millisecond)
		}
	}
	return fmt.Errorf("windivert service did not reach state %d in time", state)
}
