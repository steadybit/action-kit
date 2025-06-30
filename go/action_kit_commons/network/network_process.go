// Copyright 2025 steadybit GmbH. All rights reserved.

package network

import (
	"bytes"
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
)

func NewProcessRunner() CommandRunner {
	return &processRunner{}
}

type processRunner struct {
}

func (p processRunner) run(ctx context.Context, args []string, cmds []string) (string, error) {
	log.Info().Strs("cmds", cmds).Strs("args", args[1:]).Str("path", args[0]).Msg("running commands")

	var outb, errb bytes.Buffer
	cmd := utils.RootCommandContext(ctx, args[0], args[1:]...)
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	cmd.Stdin = ToReader(cmds)
	err := cmd.Run()

	if err != nil {
		if parsed := ParseBatchError(args, bytes.NewReader(errb.Bytes())); parsed != nil {
			return "", parsed
		}
		return "", fmt.Errorf("failed: %w, output: %s, error: %s", err, outb.String(), errb.String())
	}
	return outb.String(), err
}

func (p processRunner) id() string {
	return "host"
}
