// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package memfill

import (
	"testing"
	"time"

	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/stretchr/testify/assert"
)

func TestMemfillCommandArgs(t *testing.T) {
	t.Setenv("STEADYBIT_EXTENSION_MEMFILL_PATH", "/usr/bin/memfill")

	target := ociruntime.LinuxProcessInfo{Pid: 4242, CGroupPath: "/system.slice/target.scope"}
	args := memfillCommandArgs(target, Opts{Size: 80, Mode: ModeUsage, Unit: UnitPercent, Duration: 30 * time.Second, IgnoreCgroup: true})

	// Enters the host mount namespace, joins the target cgroup via the sh wrapper, then enters the
	// target PID namespace and execs memfill. No cgexec anywhere.
	assert.Equal(t, []string{
		"nsenter", "-t", "1", "-C", "--",
		"sh", "-c", joinCgroupScript, "sh", "/system.slice/target.scope",
		"nsenter", "-t", "4242", "-p", "-F", "--",
		"/usr/bin/memfill", "80%", "usage", "30", "--ignore-cgroup",
	}, args)

	for _, arg := range args {
		assert.NotContains(t, arg, "cgexec", "cgexec must no longer be used")
	}

	// The cgroup path is passed as a positional argument (not interpolated into the script),
	// so it cannot break out of the shell.
	assert.Contains(t, joinCgroupScript, "cgroup.procs")
	assert.Contains(t, joinCgroupScript, "exec \"$@\"")
}

func TestProcessArgs(t *testing.T) {
	t.Setenv("STEADYBIT_EXTENSION_MEMFILL_PATH", "/usr/bin/memfill")

	t.Run("minimal usage args, no optional flags", func(t *testing.T) {
		args := Opts{Size: 100, Mode: ModeUsage, Unit: UnitPercent, Duration: 120 * time.Second}.processArgs()
		assert.Equal(t, []string{"/usr/bin/memfill", "100%", "usage", "120"}, args)
	})

	t.Run("reserve, adaptive and oom_score_adj are appended in order", func(t *testing.T) {
		score := 500
		args := Opts{
			Size: 100, Mode: ModeUsage, Unit: UnitPercent, Duration: 90 * time.Second,
			Reserve: "512MiB", Adaptive: true, OomScoreAdj: &score,
		}.processArgs()
		assert.Equal(t, []string{
			"/usr/bin/memfill", "100%", "usage", "90",
			"--reserve", "512MiB", "--adaptive", "--oom-score-adj", "500",
		}, args)
	})

	t.Run("negative oom_score_adj is rendered", func(t *testing.T) {
		score := -998
		args := Opts{Size: 1024, Mode: ModeAbsolute, Unit: UnitMegabyte, Duration: 10 * time.Second, OomScoreAdj: &score}.processArgs()
		assert.Equal(t, []string{"/usr/bin/memfill", "1024MiB", "absolute", "10", "--oom-score-adj", "-998"}, args)
	})

	t.Run("optional flags are omitted when unset", func(t *testing.T) {
		args := Opts{Size: 50, Mode: ModeUsage, Unit: UnitPercent, Duration: 5 * time.Second, IgnoreCgroup: true}.processArgs()
		// Exact match: --ignore-cgroup present, and none of --reserve/--adaptive/--oom-score-adj.
		assert.Equal(t, []string{"/usr/bin/memfill", "50%", "usage", "5", "--ignore-cgroup"}, args)
	})
}
