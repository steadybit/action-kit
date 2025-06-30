// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package stress

import (
	"context"
	"fmt"
	"github.com/moby/sys/capability"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"os/exec"
	"strconv"
	"time"
)

type stressProcess struct {
	state *utils.BackgroundState
	cmd   *exec.Cmd
}

func NewStressProcess(opts Opts) (Stress, error) {
	path := utils.LocateExecutable("stress-ng", "STEADYBIT_EXTENSION_STRESSNG_PATH")

	if ok, _ := capability.GetBound(capability.CAP_SYS_RESOURCE); !ok {
		log.Warn().Msg("CAP_SYS_RESOURCE not available. oom_score_adj will fail.")
	}

	return &stressProcess{
		cmd: utils.RootCommandContext(context.Background(), path, opts.Args()...),
	}, nil
}

func (s *stressProcess) Exited() (bool, error) {
	return s.state.Exited()
}

func (s *stressProcess) Start() error {
	log.Info().
		Strs("args", s.cmd.Args).
		Str("path", s.cmd.Path).
		Msg("Starting stress-ng")

	if state, err := utils.RunCommandInBackground(s.cmd, log.Logger); err != nil {
		return fmt.Errorf("failed to start stress-ng: %w", err)
	} else {
		s.state = state
	}
	return nil
}

func (s *stressProcess) Stop() {
	//as the process is running with a different user, we also need to do so, for sending signals
	ctx := context.Background()
	if err := utils.RootCommandContext(ctx, "kill", "-s", "SIGINT", strconv.Itoa(s.cmd.Process.Pid)).Run(); err != nil {
		log.Warn().Err(err).Msg("failed to send SIGINT to memfill")
	}

	timer := time.AfterFunc(10*time.Second, func() {
		if err := utils.RootCommandContext(ctx, "kill", "-s", "SIGTERM", strconv.Itoa(s.cmd.Process.Pid)).Run(); err != nil {
			log.Warn().Err(err).Msg("failed to send SIGTERM to memfill")
		}
	})

	s.state.Wait()
	timer.Stop()
}
