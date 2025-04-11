// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH
//go:build !windows

package memfill

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/runc"
	"os/exec"
	"strconv"
	"time"
)

type MemFill struct {
	cmd   *exec.Cmd
	state *runc.BackgroundState
	args  []string
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
	BinaryPath string
	Size       int
	Mode       Mode
	Unit       Unit
	Duration   time.Duration
}

func (o Opts) processArgs() []string {
	args := []string{o.BinaryPath, fmt.Sprintf("%d%s", o.Size, o.Unit), string(o.Mode), fmt.Sprintf("%.0f", o.Duration.Seconds())}
	if len(args[0]) == 0 {
		args[0] = "memfill"
	}
	return args
}

func New(targetProcess runc.LinuxProcessInfo, opts Opts) (*MemFill, error) {
	args := append([]string{
		"nsenter", "-t", "1", "-C", "--",
		//when util-linux package >= 2.39 is broadly available we could also the cgroup change using nsenter,
		"cgexec", "-g", fmt.Sprintf("memory:%s", targetProcess.CGroupPath),
		"nsenter", "-t", strconv.Itoa(targetProcess.Pid), "-p", "-F", "--",
	},
		opts.processArgs()...,
	)

	cmd := runc.RootCommandContext(context.Background(), args[0], args[1:]...)

	return &MemFill{cmd: cmd, args: opts.processArgs()}, nil
}

func (mf *MemFill) Exited() (bool, error) {
	return mf.state.Exited()
}

func (mf *MemFill) Start() error {
	log.Info().
		Strs("args", mf.args).
		Msg("Starting memfill")

	if state, err := runc.RunCommandInBackground(mf.cmd, log.With().Str("id", "memfill").Logger()); err != nil {
		return fmt.Errorf("failed to start: %w", err)
	} else {
		mf.state = state
	}

	return nil
}

func (mf *MemFill) Stop() error {
	log.Info().
		Msg("stopping memfill")

	//as the process is running with a different user, we also need to do so, for sending signals
	ctx := context.Background()
	if err := runc.RootCommandContext(ctx, "kill", "-s", "SIGINT", strconv.Itoa(mf.cmd.Process.Pid)).Run(); err != nil {
		log.Warn().Err(err).Msg("failed to send SIGINT to memfill")
	}

	timerStart := time.AfterFunc(10*time.Second, func() {
		if err := runc.RootCommandContext(ctx, "kill", "-s", "SIGTERM", strconv.Itoa(mf.cmd.Process.Pid)).Run(); err != nil {
			log.Warn().Err(err).Msg("failed to send SIGTERM to memfill")
		}
	})

	mf.state.Wait()
	timerStart.Stop()
	return nil
}

func (mf *MemFill) Args() []string {
	return mf.args
}
