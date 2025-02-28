// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH
//go:build windows
// +build windows

package network

import (
	"fmt"
	"strings"
	"time"
)

type PackageLossOpts struct {
	Filter
	Loss       uint
	Duration   time.Duration
	Interfaces []string
}

func (o *PackageLossOpts) FwCommands(_ Family, _ Mode) ([]string, error) {
	return nil, nil
}

func (o *PackageLossOpts) QoSCommands(mode Mode) ([]string, error) {
	return nil, nil
}

func (o *PackageLossOpts) WinDivertCommands(mode Mode) ([]string, error) {
	var cmds []string

	specifiedFilter, err := buildWinDivertFilter(o.Filter)

	if err != nil {
		return []string{}, err
	}

	if mode == ModeAdd {
		cmds = append(cmds, fmt.Sprintf("wdna.exe --filter=\"%s\" --mode=drop --duration=%d --percentage=%d", specifiedFilter, o.Duration, 10))

	} else {
		cmds = append(cmds, "taskkill /f /t /im wdna.exe")
	}

	return cmds, nil
}

func (o *PackageLossOpts) String() string {
	var sb strings.Builder
	sb.WriteString("loosing packages of ")
	sb.WriteString(fmt.Sprintf("%d%%", o.Loss))
	sb.WriteString("(interfaces: ")
	sb.WriteString(strings.Join(o.Interfaces, ", "))
	sb.WriteString(")")
	writeStringForFilters(&sb, optimizeFilter(o.Filter))
	return sb.String()
}
