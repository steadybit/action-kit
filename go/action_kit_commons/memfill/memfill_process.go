// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH
//go:build !windows

package memfill

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"os/exec"
	"strconv"
	"time"
)

// joinCgroupScript moves the current shell into the target's memory cgroup and then execs the
// remaining arguments. It replaces the previous `cgexec -g memory:<path>` invocation, which
// depended on libcgroup-tools (the cgroup-tools/libcgroup-tools package). That package does not
// exist for Enterprise Linux 9 and never supported cgroup v2, so the extension could neither be
// installed nor fill memory correctly on modern hosts.
//
// The move must happen before exec: under cgroup v2 a process keeps the memory already charged to
// it when migrated, so anything memfill allocates before joining would be accounted to the wrong
// cgroup. Writing $$ and then `exec`ing keeps the same PID, so memfill starts already inside the
// target cgroup with nothing allocated yet.
//
// The first argument is the target cgroup path (as read from /proc/<pid>/cgroup); the remaining
// arguments are the command to exec. cgroup v1 (memory controller) is preferred over v2 unified to
// match the historical `memory:` semantics on hybrid hosts.
const joinCgroupScript = `cg="$1"; shift
if [ -e "/sys/fs/cgroup/memory${cg}/cgroup.procs" ]; then
  procs="/sys/fs/cgroup/memory${cg}/cgroup.procs"
elif [ -e "/sys/fs/cgroup${cg}/cgroup.procs" ]; then
  procs="/sys/fs/cgroup${cg}/cgroup.procs"
else
  echo "memfill: no cgroup.procs found for ${cg} (looked under cgroup v1 memory and v2 unified)" >&2
  exit 1
fi
printf '%s\n' "$$" > "$procs" || { echo "memfill: failed to join cgroup $procs" >&2; exit 1; }
exec "$@"`

type memfillRunc struct {
	cmd   *exec.Cmd
	state *utils.BackgroundState
	args  []string
}

// memfillCommandArgs builds the argument vector that runs memfill inside the target's memory
// cgroup and PID namespace. It enters the host mount namespace (nsenter -t 1 -C), joins the target
// cgroup via joinCgroupScript, then enters the target PID namespace and execs memfill.
func memfillCommandArgs(targetProcess ociruntime.LinuxProcessInfo, opts Opts) []string {
	args := []string{
		"nsenter", "-t", "1", "-C", "--",
		"sh", "-c", joinCgroupScript, "sh", targetProcess.CGroupPath,
		"nsenter", "-t", strconv.Itoa(targetProcess.Pid), "-p", "-F", "--",
	}
	return append(args, opts.processArgs()...)
}

func NewMemfillProcess(targetProcess ociruntime.LinuxProcessInfo, opts Opts) (Memfill, error) {
	args := memfillCommandArgs(targetProcess, opts)

	cmd := utils.RootCommandContext(context.Background(), args[0], args[1:]...)

	return &memfillRunc{cmd: cmd, args: opts.processArgs()}, nil
}

func (mf *memfillRunc) Exited() (bool, error) {
	if mf.state == nil {
		// Start was never called (or failed): there is no running process.
		return true, nil
	}
	return mf.state.Exited()
}

func (mf *memfillRunc) Start() error {
	log.Info().
		Strs("args", mf.args).
		Msg("Starting memfill")

	if state, err := utils.RunCommandInBackground(mf.cmd, log.With().Str("id", "memfill").Logger()); err != nil {
		return fmt.Errorf("failed to start: %w", err)
	} else {
		mf.state = state
	}

	return nil
}

func (mf *memfillRunc) Stop() error {
	log.Info().
		Msg("stopping memfill")

	if mf.cmd.Process == nil || mf.state == nil {
		// never started (or start failed) — nothing to stop
		return nil
	}
	//as the process is running with a different user, we also need to do so, for sending signals
	ctx := context.Background()
	// Capture the pid up front so the timer goroutine doesn't read mf.cmd.Process
	// concurrently with the exec waiter.
	pid := strconv.Itoa(mf.cmd.Process.Pid)
	if err := utils.RootCommandContext(ctx, "kill", "-s", "SIGINT", pid).Run(); err != nil {
		log.Warn().Err(err).Msg("failed to send SIGINT to memfill")
	}

	timer := time.AfterFunc(10*time.Second, func() {
		if err := utils.RootCommandContext(ctx, "kill", "-s", "SIGTERM", pid).Run(); err != nil {
			log.Warn().Err(err).Msg("failed to send SIGTERM to memfill")
		}
	})

	mf.state.Wait()
	timer.Stop()
	return nil
}

func (mf *memfillRunc) Args() []string {
	return mf.args
}
