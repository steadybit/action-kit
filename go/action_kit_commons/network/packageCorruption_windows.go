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

type CorruptPackagesOpts struct {
	Filter
	Corruption uint
	Duration   time.Duration
}

func (o *CorruptPackagesOpts) FwCommands(_ Family, _ Mode) ([]string, error) {
	return nil, nil
}

func (o *CorruptPackagesOpts) QoSCommands(_ Mode) ([]string, error) {
	return nil, nil
}

func (o *CorruptPackagesOpts) WinDivertCommands(mode Mode) ([]string, error) {
	var cmds []string

	specifiedFilter, err := buildWinDivertFilter(o.Filter)

	if err != nil {
		return []string{}, err
	}

	if mode == ModeAdd {
		cmds = append(cmds, fmt.Sprintf("wdna.exe --filter=\"%s\" --mode=corrupt --duration=%d --percentage=%d", specifiedFilter, int(o.Duration.Seconds()), o.Corruption))

	} else {
		cmds = append(cmds, "taskkill /f /t /im wdna.exe")
		cmds = append(cmds, "cmd /c sc stop windivert")
	}

	return cmds, nil
}

func (o *CorruptPackagesOpts) String() string {
	var sb strings.Builder
	sb.WriteString("corrupting packages of ")
	sb.WriteString(fmt.Sprintf("%d%%", o.Corruption))
	sb.WriteString(fmt.Sprintf("%s", o.Filter))
	return sb.String()
}
