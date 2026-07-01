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

type memfillRunc struct {
	cmd   *exec.Cmd
	state *utils.BackgroundState
	args  []string
}

func NewMemfillProcess(targetProcess ociruntime.LinuxProcessInfo, opts Opts) (Memfill, error) {
	args := append([]string{
		"nsenter", "-t", "1", "-C", "--",
		//when util-linux package >= 2.39 is broadly available we could also the cgroup change using nsenter,
		"cgexec", "-g", fmt.Sprintf("memory:%s", targetProcess.CGroupPath),
		"nsenter", "-t", strconv.Itoa(targetProcess.Pid), "-p", "-F", "--",
	},
		opts.processArgs()...,
	)

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
