// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package diskfill

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
)

type diskfillProcess struct {
	state    *utils.BackgroundState
	cmd      *exec.Cmd
	lifeline io.Closer
	fillArgs []string
}

func NewDiskfillProcess(ctx context.Context, opts Opts) (Diskfill, error) {
	fillArgs, err := opts.Args(func(path string) (*DiskUsage, error) {
		return readDiskUsageProcess(ctx, path)
	})
	if err != nil {
		return nil, err
	}

	processArgs := opts.processArgs(fillArgs, opts.TempPath)
	return &diskfillProcess{
		cmd:      utils.RootCommandContext(context.Background(), processArgs[0], processArgs[1:]...),
		fillArgs: fillArgs,
	}, nil
}

func (df *diskfillProcess) Exited() (bool, error) {
	if df.state == nil {
		// Start was never called (or failed): there is no running process.
		return true, nil
	}
	return df.state.Exited()
}

func (df *diskfillProcess) Start() error {
	log.Info().
		Strs("args", df.cmd.Args).
		Str("path", df.cmd.Path).
		Msg("Starting diskfill")

	state, lifeline, err := startWithLifeline(df.cmd, log.Logger)
	if err != nil {
		return fmt.Errorf("failed to start diskfill: %w", err)
	}
	df.state = state
	df.lifeline = lifeline
	return nil
}

func (df *diskfillProcess) Stop() error {
	log.Info().
		Msg("stopping diskfill")

	if df.cmd.Process == nil || df.state == nil {
		// never started (or start failed) — nothing to stop
		return nil
	}
	// Capture the pid up front so the timer goroutine doesn't read df.cmd.Process
	// concurrently with the exec waiter.
	pid := strconv.Itoa(df.cmd.Process.Pid)
	_ = df.lifeline.Close()

	timer := time.AfterFunc(10*time.Second, func() {
		//as the process is running with a different user, we also need to do so, for sending signals
		if err := utils.RootCommandContext(context.Background(), "kill", "-s", "SIGTERM", pid).Run(); err != nil {
			log.Warn().Err(err).Msg("failed to send SIGTERM to diskfill")
		}
	})

	df.state.Wait()
	timer.Stop()
	return nil
}

func (df *diskfillProcess) Args() []string {
	return append([]string{}, df.fillArgs...)
}

func (df *diskfillProcess) Noop() bool {
	return isNoop(df.fillArgs)
}

func CheckPathWritableProcess(ctx context.Context, targetPath string) error {
	if out, err := utils.RootCommandContext(ctx, "test", "-d", targetPath).CombinedOutput(); err != nil {
		return fmt.Errorf("target path %q does not exist: %w: %s", targetPath, err, string(out))
	}

	checkFile := filepath.Join(targetPath, ".steadybit-diskfill-check")
	if out, err := utils.RootCommandContext(ctx, "touch", checkFile).CombinedOutput(); err != nil {
		return fmt.Errorf("target path %q is not writable: %w: %s", targetPath, err, string(out))
	}

	_ = utils.RootCommandContext(ctx, "rm", "-f", checkFile).Run()
	return nil
}
