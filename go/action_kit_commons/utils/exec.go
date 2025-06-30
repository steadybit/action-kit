// Copyright 2025 steadybit GmbH. All rights reserved.

package utils

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"io"
	"os/exec"
	"sync"
	"syscall"
)

func RootCommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: 0,
			Gid: 0,
		},
	}
	return cmd
}

type BackgroundState struct {
	cond   *sync.Cond
	exited bool
	err    error
}

func (r *BackgroundState) Exited() (bool, error) {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()
	return r.exited, r.err
}

func (r *BackgroundState) Wait() {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()
	for !r.exited {
		r.cond.Wait()
	}
}

func RunCommandInBackground(cmd *exec.Cmd, logger zerolog.Logger) (*BackgroundState, error) {
	var errb bytes.Buffer
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = io.MultiWriter(&errb, pw)

	go func() {
		defer func() { _ = pr.Close() }()
		bufReader := bufio.NewReader(pr)

		for {
			if line, err := bufReader.ReadString('\n'); err == nil {
				logger.Debug().Msg(line)
			} else {
				break
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start: %w", err)
	}

	result := &BackgroundState{
		cond:   &sync.Cond{L: &sync.Mutex{}},
		exited: false,
		err:    nil,
	}

	go func(r *BackgroundState) {
		defer func() { _ = pw.Close() }()
		err := cmd.Wait()
		logger.Trace().Int("exitCode", cmd.ProcessState.ExitCode()).Msg("exited.")

		r.cond.L.Lock()
		defer r.cond.L.Unlock()

		r.exited = true
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitErr.Stderr = errb.Bytes()
			r.err = exitErr
		} else {
			r.err = err
		}

		r.cond.Broadcast()
	}(result)

	return result, nil
}
