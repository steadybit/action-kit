// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH
//go:build windows

package network

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

type LimitBandwidthOpts struct {
	Bandwidth   string
	IncludeCidr *net.IPNet
	Port        string
}

func (o *LimitBandwidthOpts) FwCommands(_ Family, _ Mode) ([]string, error) {
	return nil, nil
}

func (o *LimitBandwidthOpts) WinDivertCommands(_ Mode) ([]string, error) {
	return nil, nil
}

func (o *LimitBandwidthOpts) QoSCommands(mode Mode) ([]string, error) {
	var cmds []string

	expression, err := regexp.Compile("^[0-7]$")
	if err != nil {
		return nil, err
	}
	if expression.MatchString(o.Bandwidth) {
		return nil, fmt.Errorf("windows qos policy does not support rate settings below 8bit/s. (%s)", o.Bandwidth)
	}

	if mode == ModeAdd {
		var additionalParameters string
		if o.IncludeCidr != nil {
			additionalParameters = fmt.Sprintf("-IPDstPrefixMatchCondition '%s' ", o.IncludeCidr.String())
		}
		if o.Port != "" {
			additionalParameters = fmt.Sprintf("%s-IPDstPortMatchCondition %s", additionalParameters, o.Port)
		}
		cmds = append(cmds, fmt.Sprintf("New-NetQosPolicy -Name STEADYBIT_QOS_%s -Precedence 255 -PolicyStore ActiveStore -Confirm:`$false -ThrottleRateActionBitsPerSecond %s %s", o.Bandwidth, o.Bandwidth, additionalParameters))
	} else {
		cmds = append(cmds, fmt.Sprintf("Remove-NetQosPolicy -Name STEADYBIT_QOS_%s -PolicyStore ActiveStore -Confirm:`$false", o.Bandwidth))
	}

	return cmds, nil
}

func (o *LimitBandwidthOpts) String() string {
	var sb strings.Builder
	sb.WriteString("limit bandwidth to ")
	sb.WriteString(o.Bandwidth)
	if o.IncludeCidr != nil || o.Port != "" {
		sb.WriteString(" for ")
		if o.IncludeCidr != nil {
			sb.WriteString(o.IncludeCidr.String())
		}
		if o.Port != "" {
			sb.WriteString(":")
			sb.WriteString(o.Port)
		}
	}
	return sb.String()
}
