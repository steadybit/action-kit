// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package diskfill

import (
	_ "embed" // required by the go:embed directive for holderSource
	"io"
	"math"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
)

// UnlinkedFileMessage tells users where to find the filled space, as the fill
// file has no name during the attack.
const UnlinkedFileMessage = "The disk-fill file is deleted right after it is opened and is held open until the attack ends, so the filled space is freed even if the extension is terminated. It shows up in df, but not in ls or du; lsof +L1 lists it."

// fillFd is where the fill command finds the file held by holderScript.
const fillFd = "/proc/self/fd/3"

//go:embed holder.sh
var holderSource string

// holderScript is passed on the command line, so it is kept free of comments.
var holderScript = stripComments(holderSource)

// stripComments drops blank and comment lines; comments after code are kept.
func stripComments(script string) string {
	var b strings.Builder
	for line := range strings.Lines(script) {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			b.WriteString(line)
		}
	}
	return b.String()
}

// holderArgs wraps the fill command in holderScript, which creates the fill
// file in dir and holds it for o.Duration or, without one, until Stop.
func (o Opts) holderArgs(fillArgs []string, dir string) []string {
	if isNoop(fillArgs) {
		return fillArgs
	}
	seconds := int64(math.Ceil(o.Duration.Seconds()))
	if seconds <= 0 {
		seconds = math.MaxInt32
	}
	return append([]string{"sh", "-c", holderScript, "diskfill", filepath.Join(dir, "disk-fill"), strconv.FormatInt(seconds, 10)}, fillArgs...)
}

// startWithLifeline starts cmd with the lifeline as its stdin; closing the
// returned writer ends the holder.
func startWithLifeline(cmd *exec.Cmd, logger zerolog.Logger) (*utils.BackgroundState, io.Closer, error) {
	lifeline, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	state, err := utils.RunCommandInBackground(cmd, logger)
	if err != nil {
		_ = lifeline.Close()
		return nil, nil, err
	}
	return state, lifeline, nil
}
