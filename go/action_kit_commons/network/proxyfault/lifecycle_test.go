// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package proxyfault

import (
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func waitForExit(t *testing.T, b *processBase) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		if ex, _ := b.Exited(); ex {
			return
		}
		select {
		case <-deadline:
			t.Fatal("process did not exit in time")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func returnsWithin(t *testing.T, d time.Duration, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { fn(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatal("call blocked longer than expected")
	}
}

// waitExited must never block when the process was never started — otherwise a
// Stop() during cleanup after a failed Start() would hang forever.
func Test_waitExited_returns_when_never_started(t *testing.T) {
	b := newProcessBase()
	ex, err := b.Exited()
	require.False(t, ex)
	require.NoError(t, err)
	returnsWithin(t, time.Second, func() { b.waitExited() })
}

// Exited() must keep reporting the exit across repeated calls and after
// waitExited() (the Stop path) — the previous channel-drain design lost the
// value, so a second Stop blocked and Exited() went back to reporting running.
func Test_exited_stable_across_repeated_stop(t *testing.T) {
	b := newProcessBase()
	require.NoError(t, b.startAndMonitor(exec.Command("sh", "-c", "exit 0"), "test"))
	waitForExit(t, &b)

	ex, err := b.Exited()
	require.True(t, ex)
	require.NoError(t, err)

	// Two consecutive Stops (waitExited) must both return promptly.
	returnsWithin(t, 2*time.Second, func() { b.waitExited() })
	returnsWithin(t, 2*time.Second, func() { b.waitExited() })

	// And Exited() must still report the process as exited.
	ex, err = b.Exited()
	require.True(t, ex, "Exited() must still report exited after Stop")
	require.NoError(t, err)
}

// A non-zero exit is surfaced (and remains readable after Stop).
func Test_exited_reports_error(t *testing.T) {
	b := newProcessBase()
	require.NoError(t, b.startAndMonitor(exec.Command("sh", "-c", "exit 7"), "test"))
	waitForExit(t, &b)
	b.waitExited()
	ex, err := b.Exited()
	require.True(t, ex)
	require.Error(t, err)
}
