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

var testTarget = ociruntime.LinuxProcessInfo{
	Pid:        4242,
	CGroupPath: "/kubepods/besteffort/pod123/container456",
}

func TestProcessArgs(t *testing.T) {
	t.Setenv("STEADYBIT_EXTENSION_MEMFILL_PATH", "/usr/bin/memfill")

	t.Run("minimal usage args, no optional flags", func(t *testing.T) {
		args := Opts{Size: 100, Mode: ModeUsage, Unit: UnitPercent, Duration: 120 * time.Second}.processArgs(testTarget)
		assert.Equal(t, []string{
			"/usr/bin/memfill", "100%", "usage", "120",
			"--target-cgroup-path", "/kubepods/besteffort/pod123/container456",
			"--target-pid", "4242",
		}, args)
	})

	t.Run("reserve, adaptive and oom_score_adj are appended in order", func(t *testing.T) {
		score := 500
		args := Opts{
			Size: 100, Mode: ModeUsage, Unit: UnitPercent, Duration: 90 * time.Second,
			Reserve: "512MiB", Adaptive: true, OomScoreAdj: &score,
		}.processArgs(testTarget)
		assert.Equal(t, []string{
			"/usr/bin/memfill", "100%", "usage", "90",
			"--reserve", "512MiB", "--adaptive", "--oom-score-adj", "500",
			"--target-cgroup-path", "/kubepods/besteffort/pod123/container456",
			"--target-pid", "4242",
		}, args)
	})

	t.Run("negative oom_score_adj is rendered", func(t *testing.T) {
		score := -998
		args := Opts{Size: 1024, Mode: ModeAbsolute, Unit: UnitMegabyte, Duration: 10 * time.Second, OomScoreAdj: &score}.processArgs(testTarget)
		assert.Equal(t, []string{
			"/usr/bin/memfill", "1024MiB", "absolute", "10", "--oom-score-adj", "-998",
			"--target-cgroup-path", "/kubepods/besteffort/pod123/container456",
			"--target-pid", "4242",
		}, args)
	})

	t.Run("optional flags are omitted when unset", func(t *testing.T) {
		args := Opts{Size: 50, Mode: ModeUsage, Unit: UnitPercent, Duration: 5 * time.Second, IgnoreCgroup: true}.processArgs(testTarget)
		// Exact match: --ignore-cgroup present, and none of --reserve/--adaptive/--oom-score-adj.
		assert.Equal(t, []string{
			"/usr/bin/memfill", "50%", "usage", "5", "--ignore-cgroup",
			"--target-cgroup-path", "/kubepods/besteffort/pod123/container456",
			"--target-pid", "4242",
		}, args)
	})

	t.Run("target flags are omitted for a zero-value target", func(t *testing.T) {
		args := Opts{Size: 50, Mode: ModeUsage, Unit: UnitPercent, Duration: 5 * time.Second}.processArgs(ociruntime.LinuxProcessInfo{})
		assert.Equal(t, []string{"/usr/bin/memfill", "50%", "usage", "5"}, args)
	})
}

func TestNewMemfillProcessArgs(t *testing.T) {
	t.Setenv("STEADYBIT_EXTENSION_MEMFILL_PATH", "/usr/bin/memfill")

	mf, err := NewMemfillProcess(testTarget, Opts{
		Size: 100, Mode: ModeUsage, Unit: UnitPercent, Duration: 120 * time.Second,
	})
	assert.NoError(t, err)

	cmd := mf.(*memfillRunc).cmd
	// Only the host cgroup namespace is entered externally; the cgroup join and
	// PID-namespace entry are memfill's own --target-* flags now.
	assert.Equal(t, []string{
		"nsenter", "-t", "1", "-C", "--",
		"/usr/bin/memfill", "100%", "usage", "120",
		"--target-cgroup-path", "/kubepods/besteffort/pod123/container456",
		"--target-pid", "4242",
	}, cmd.Args)
	assert.NotContains(t, cmd.Args, "cgexec")
}
