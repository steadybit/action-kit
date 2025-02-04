// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH
//go:build windows
// +build windows

package network

import (
	"fmt"
	"regexp"
	"strings"
)

type LimitBandwidthOpts struct {
	Filter
	Bandwidth  string
	Interfaces []string
}

func (o *LimitBandwidthOpts) FwCommands(_ Family, _ Mode) ([]string, error) {
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
		cmds = append(cmds, fmt.Sprintf("New-NetQosPolicy -Name STEADYBIT_QOS_%s -Default -PolicyStore ActiveStore -ThrottleRateActionBitsPerSecond %s", o.Bandwidth, o.Bandwidth))
	} else {
		cmds = append(cmds, fmt.Sprintf("Remove-NetQosPolicy -Name STEADYBIT_QOS_%s -PolicyStore ActiveStore", o.Bandwidth))
	}

	return cmds, nil
}

func (o *LimitBandwidthOpts) String() string {
	var sb strings.Builder
	sb.WriteString("limit bandwidth to ")
	sb.WriteString(o.Bandwidth)
	sb.WriteString(" (interfaces: ")
	sb.WriteString(strings.Join(o.Interfaces, ", "))
	sb.WriteString(")")
	writeStringForFilters(&sb, optimizeFilter(o.Filter))
	return sb.String()
}
