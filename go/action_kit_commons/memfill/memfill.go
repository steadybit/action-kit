// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH
//go:build !windows

package memfill

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"time"
)

type Memfill interface {
	Exited() (bool, error)
	Start() error
	Stop() error
	Args() []string
}

type Mode string
type Unit string

const (
	ModeUsage    Mode = "usage"
	ModeAbsolute Mode = "absolute"
	UnitPercent  Unit = "%"
	UnitMegabyte Unit = "MiB"
)

type Opts struct {
	Size     int
	Mode     Mode
	Unit     Unit
	Duration time.Duration
}

func (o Opts) processArgs() []string {
	path := utils.LocateExecutable("memfill", "STEADYBIT_EXTENSION_MEMFILL_PATH")
	args := []string{path, fmt.Sprintf("%d%s", o.Size, o.Unit), string(o.Mode), fmt.Sprintf("%.0f", o.Duration.Seconds())}
	return args
}
