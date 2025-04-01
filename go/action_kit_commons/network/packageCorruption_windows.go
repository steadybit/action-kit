// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH
//go:build windows
// +build windows

package network

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type CorruptPackagesOpts struct {
	Filter
	Corruption uint
	Duration   time.Duration
	filterFile string
}

func (o *CorruptPackagesOpts) FwCommands(_ Family, _ Mode) ([]string, error) {
	return nil, nil
}

func (o *CorruptPackagesOpts) QoSCommands(_ Mode) ([]string, error) {
	return nil, nil
}

func (o *CorruptPackagesOpts) WinDivertCommands(mode Mode) ([]string, error) {
	var cmds []string

	if mode == ModeAdd {
		filterFile, err := buildWinDivertFilterFile(o.Filter)
		if err != nil {
			return nil, err
		}
		o.filterFile = filterFile
		cmds = append(cmds, fmt.Sprintf("wdna.exe --file=%q --mode=corrupt --duration=%d --percentage=%d", filterFile, int(o.Duration.Seconds()), o.Corruption))

	} else {
		cmds = append(cmds, "wdna_shutdown")
		cmds = append(cmds, "cmd /c \"sc stop windivert || exit /b 0\"") // don't fail on error
		_ = os.Remove(o.filterFile)
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
