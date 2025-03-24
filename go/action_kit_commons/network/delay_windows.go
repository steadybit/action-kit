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
	Delay    time.Duration
	Duration time.Duration
	Jitter   bool
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

		filterFile, err := o.createFilterFile()
		if err != nil {
			return nil, err
		}
		_ = filterFile.Close()
		cmds = append(cmds, fmt.Sprintf("wdna.exe --file=%q --mode=delay --duration=%d --time=%d %s", filterFile.Name(), int(o.Duration.Seconds()), o.Delay.Milliseconds(), jitter))

	} else {
		cmds = append(cmds, "wdna_shutdown")
		cmds = append(cmds, "cmd /c \"sc stop windivert || exit /b 0\"") // don't fail on error
	}

	return cmds, nil
}

func (o *DelayOpts) createFilterFile() (*os.File, error) {
	specifiedFilter, err := buildWinDivertFilter(o.Filter)
	if err != nil {
		return nil, err
	}
	tempFile, err := os.CreateTemp("", "wdna-filter-*.txt")
	if err != nil {
		return nil, err
	}
	_, err = tempFile.Write([]byte(specifiedFilter))
	if err != nil {
		return nil, err
	}
	return tempFile, nil
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
