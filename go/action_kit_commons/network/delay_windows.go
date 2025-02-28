//go:build windows
// +build windows

// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package network

import (
	"fmt"
	"strings"
	"time"
)

type DelayOpts struct {
	Filter
	Delay      time.Duration
	Duration   time.Duration
	Jitter     bool
	Interfaces []string
}

func (o *DelayOpts) FwCommands(_ Family, _ Mode) ([]string, error) {
	return nil, nil
}

func (o *DelayOpts) QoSCommands(mode Mode) ([]string, error) {
	return nil, nil
}

func (o *DelayOpts) WinDivertCommands(mode Mode) ([]string, error) {
	var cmds []string

	jitter := ""
	if o.Jitter {
		jitter = "--jitter"
	}

	specifiedFilter, err := buildWinDivertFilter(o.Filter)

	if err != nil {
		return []string{}, err
	}

	if mode == ModeAdd {
		cmds = append(cmds, fmt.Sprintf("wdna.exe --filter=\"%s\" --mode=delay --duration=%d --time=%d %s", specifiedFilter, int(o.Duration.Seconds()), o.Delay.Milliseconds(), jitter))
	} else {
		cmds = append(cmds, "taskkill /f /t /im wdna.exe")
		cmds = append(cmds, "sc stop windivert")
		cmds = append(cmds, "sc delete windivert")
	}

	return cmds, nil
}

func (o *DelayOpts) String() string {
	var sb strings.Builder
	sb.WriteString("delaying traffic by ")
	sb.WriteString(o.Delay.String())
	sb.WriteString(" (jitter: ")
	if o.Jitter {
		sb.WriteString("yes")
	} else {
		sb.WriteString("no")
	}
	sb.WriteString(")")
	return sb.String()
}
