// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build linux

package diskfill

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type holder struct {
	cmd      *exec.Cmd
	dir      string
	lifeline io.Closer
	done     chan struct{}
	err      error
}

func startHolder(t *testing.T, duration time.Duration, fillArgs ...string) *holder {
	t.Helper()
	h := &holder{dir: t.TempDir(), done: make(chan struct{})}
	args := holderArgs(filepath.Join(h.dir, "disk-fill"), duration, fillArgs)
	h.cmd = exec.Command(args[0], args[1:]...)
	h.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	lifeline, err := h.cmd.StdinPipe()
	require.NoError(t, err)
	h.lifeline = lifeline
	require.NoError(t, h.cmd.Start())
	go func() {
		h.err = h.cmd.Wait()
		close(h.done)
	}()
	t.Cleanup(func() {
		_ = syscall.Kill(-h.cmd.Process.Pid, syscall.SIGKILL)
		<-h.done
	})
	return h
}

// heldSize also fails while fd 3 is not yet the unlinked fill file, e.g.
// before the exec, when the forked Go child still has its own fd 3.
func (h *holder) heldSize() (int64, error) {
	fd := fmt.Sprintf("/proc/%d/fd/3", h.cmd.Process.Pid)
	if target, err := os.Readlink(fd); err != nil {
		return 0, err
	} else if target != filepath.Join(h.dir, "disk-fill")+" (deleted)" {
		return 0, fmt.Errorf("fd 3 is %s", target)
	}
	fi, err := os.Stat(fd)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func (h *holder) requireHolding(t *testing.T) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		_, err := h.heldSize()
		assert.NoError(c, err)
	}, time.Second, 10*time.Millisecond)
}

func (h *holder) requireExit(t *testing.T, within time.Duration) error {
	t.Helper()
	select {
	case <-h.done:
		return h.err
	case <-time.After(within):
		require.FailNow(t, "holder did not exit", "within %s", within)
		return nil
	}
}

// requireNoLeftovers checks that no process of the holder's process group
// (its lifeline subshell, the timer and the fill) outlives the holder: the
// runtime considers the sidecar stopped as soon as the holder exits.
func (h *holder) requireNoLeftovers(t *testing.T) {
	t.Helper()
	assert.ErrorIs(t, syscall.Kill(-h.cmd.Process.Pid, 0), syscall.ESRCH)
}

func Test_holder_keeps_the_space_only_until_the_deadline(t *testing.T) {
	h := startHolder(t, 2*time.Second, fallocateArgs(1024, fillFd)...)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		size, err := h.heldSize()
		require.NoError(c, err)
		assert.Equal(c, int64(1024*1024), size)
	}, time.Second, 10*time.Millisecond)

	entries, err := os.ReadDir(h.dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "the fill file must be unlinked while it is held")

	assert.NoError(t, h.requireExit(t, 5*time.Second))
	h.requireNoLeftovers(t)
}

func Test_holder_stops_a_running_fill_on_signal(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			h := startHolder(t, time.Minute, "sleep", "60")
			h.requireHolding(t)

			require.NoError(t, h.cmd.Process.Signal(sig))

			assert.NoError(t, h.requireExit(t, 2*time.Second))
			h.requireNoLeftovers(t)
		})
	}
}

func Test_holder_stops_when_the_lifeline_closes(t *testing.T) {
	h := startHolder(t, time.Minute, "sleep", "60")
	h.requireHolding(t)

	require.NoError(t, h.lifeline.Close())

	assert.NoError(t, h.requireExit(t, 2*time.Second))
	h.requireNoLeftovers(t)
}

func Test_holder_returns_the_exit_code_of_a_failed_fill(t *testing.T) {
	h := startHolder(t, time.Minute, "sh", "-c", "exit 3")

	err := h.requireExit(t, 2*time.Second)

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 3, exitErr.ExitCode())
	h.requireNoLeftovers(t)
}

func Test_holderArgs_holds_until_stop_without_duration(t *testing.T) {
	args := holderArgs("/tmp/disk-fill", 0, []string{"true"})
	assert.Contains(t, args, "2147483647")
}

func Test_processArgs_runs_noop_without_holder(t *testing.T) {
	assert.Equal(t, []string{"echo", "noop"}, Opts{}.processArgs([]string{"echo", "noop"}, "/tmp"))
}
