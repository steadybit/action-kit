// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package diskfill

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type diskfillProcess struct {
	state        *utils.BackgroundState
	cmd          *exec.Cmd
	pathToRemove string
}

func NewDiskfillProcess(ctx context.Context, opts Opts) (Diskfill, error) {
	processArgs, err := opts.Args("", func(path string) (*DiskUsage, error) {
		return readDiskUsageProcess(ctx, path)
	})
	if err != nil {
		return nil, err
	}

	return &diskfillProcess{
		cmd:          utils.RootCommandContext(context.Background(), processArgs[0], processArgs[1:]...),
		pathToRemove: filepath.Join(opts.TempPath, "disk-fill"),
	}, nil
}

func (df *diskfillProcess) Exited() (bool, error) {
	return df.state.Exited()
}

func (df *diskfillProcess) Start() error {
	log.Info().
		Strs("args", df.cmd.Args).
		Str("path", df.cmd.Path).
		Msg("Starting diskfill")

	if state, err := utils.RunCommandInBackground(df.cmd, log.Logger); err != nil {
		return fmt.Errorf("failed to start diskfill: %w", err)
	} else {
		df.state = state
	}
	return nil
}

func (df *diskfillProcess) Stop() error {
	log.Info().
		Msg("stopping diskfill")

	//as the process is running with a different user, we also need to do so, for sending signals
	ctx := context.Background()
	if err := utils.RootCommandContext(ctx, "kill", "-s", "SIGINT", strconv.Itoa(df.cmd.Process.Pid)).Run(); err != nil {
		log.Warn().Err(err).Msg("failed to send SIGINT to diskfill")
	}

	timer := time.AfterFunc(10*time.Second, func() {
		if err := utils.RootCommandContext(ctx, "kill", "-s", "SIGTERM", strconv.Itoa(df.cmd.Process.Pid)).Run(); err != nil {
			log.Warn().Err(err).Msg("failed to send SIGTERM to diskfill")
		}
	})

	df.state.Wait()
	timer.Stop()

	var deleteFileErr error
	if !df.Noop() {
		if out, err := utils.RootCommandContext(ctx, "rm", df.pathToRemove).CombinedOutput(); err != nil && !strings.Contains(string(out), "No such file or directory") {
			log.Error().Err(err).Msgf("failed to remove file %s", out)
			deleteFileErr = fmt.Errorf("failed to remove file %s! You have to remove it manually now! %s", df.pathToRemove, out)
		} else {
			log.Debug().Msgf("removed file %s: %s", df.pathToRemove, out)
		}
	}
	return deleteFileErr
}

func (df *diskfillProcess) Args() []string {
	return append([]string{}, df.cmd.Args...)
}

func (df *diskfillProcess) Noop() bool {
	return df.cmd.Args[0] == "echo" && df.cmd.Args[1] == "noop"
}
