// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build windows

package network

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type PackageLossOpts struct {
	Filter
	Loss             uint
	Duration         time.Duration
	filterFile       string
	InterfaceIndexes []int
}

func (o *PackageLossOpts) FwCommands(_ Family, _ Mode) ([]string, error) {
	return nil, nil
}

func (o *PackageLossOpts) QoSCommands(mode Mode) ([]string, error) {
	return nil, nil
}

func (o *PackageLossOpts) WinDivertCommands(mode Mode) ([]string, error) {
	var cmds []string

	if mode == ModeAdd {
		filterFile, err := buildWinDivertFilterFileWithInterfaces(o.Filter, o.InterfaceIndexes)
		if err != nil {
			return nil, err
		}
		o.filterFile = filterFile
		cmds = append(cmds, fmt.Sprintf("wdna.exe --file=%q --mode=drop --duration=%d --percentage=%d", filterFile, int(o.Duration.Seconds()), o.Loss))

	} else {
		cmds = append(cmds, "wdna_shutdown")
		cmds = append(cmds, "cmd /c \"sc stop windivert || exit /b 0\"") // don't fail on error
		_ = os.Remove(o.filterFile)
	}

	return cmds, nil
}

func (o *PackageLossOpts) String() string {
	var sb strings.Builder
	sb.WriteString("loosing packages of ")
	sb.WriteString(fmt.Sprintf("%d%%", o.Loss))
	writeStringForFilters(&sb, optimizeFilter(o.Filter))
	return sb.String()
}
