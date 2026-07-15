// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH
//go:build !windows

package memfill

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"strconv"
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
	Size         int
	Mode         Mode
	Unit         Unit
	Duration     time.Duration
	IgnoreCgroup bool
	// Reserve is the minimum memory to keep available (e.g. "512MiB"). Empty means no reserve.
	// In usage mode it floors the memory left free; in absolute mode it caps the allocation.
	Reserve string
	// Adaptive frees memory when the host is under memory pressure (Linux PSI). Usage mode only.
	Adaptive bool
	// OomScoreAdj, when set, is the oom_score_adj (-1000..1000) applied to the fill process.
	// Host fills should pass a high value so the OOM killer targets the fill, not the kubelet.
	OomScoreAdj *int
}

func (o Opts) processArgs() []string {
	path := utils.LocateExecutable("memfill", "STEADYBIT_EXTENSION_MEMFILL_PATH")
	args := []string{path, fmt.Sprintf("%d%s", o.Size, o.Unit), string(o.Mode), fmt.Sprintf("%.0f", o.Duration.Seconds())}
	if o.IgnoreCgroup {
		args = append(args, "--ignore-cgroup")
	}
	if o.Reserve != "" {
		args = append(args, "--reserve", o.Reserve)
	}
	if o.Adaptive {
		args = append(args, "--adaptive")
	}
	if o.OomScoreAdj != nil {
		args = append(args, "--oom-score-adj", strconv.Itoa(*o.OomScoreAdj))
	}
	return args
}
