//go:build windows
// +build windows

// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package network

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type DelayOpts struct {
	Filter
	Delay            time.Duration
	Duration         time.Duration
	Jitter           bool
	FilterFile       string
	InterfaceIndexes []int
}

func (o *DelayOpts) FwCommands(_ Family, _ Mode) ([]string, error) {
	return nil, nil
}

func (o *DelayOpts) QoSCommands(mode Mode) ([]string, error) {
	return nil, nil
}

func (o *DelayOpts) WinDivertCommands(mode Mode) ([]string, error) {
	var cmds []string

	if mode == ModeAdd {
		jitter := ""
		if o.Jitter {
			jitter = "--jitter"
		}

		filterFile, err := buildWinDivertFilterFileWithInterfaces(o.Filter, o.InterfaceIndexes)
		if err != nil {
			return nil, err
		}
		o.FilterFile = filterFile

		cmds = append(cmds, fmt.Sprintf("wdna.exe --file=%q --mode=delay --duration=%d --time=%d %s", filterFile, int(o.Duration.Seconds()), o.Delay.Milliseconds(), jitter))

	} else {
		cmds = append(cmds, "wdna_shutdown")
		cmds = append(cmds, "cmd /c \"sc stop windivert || exit /b 0\"") // don't fail on error
		_ = os.Remove(o.FilterFile)
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
