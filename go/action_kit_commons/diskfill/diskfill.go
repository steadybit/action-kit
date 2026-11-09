// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package diskfill

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
)

type Diskfill interface {
	Exited() (bool, error)
	Start() error
	Stop() error
	Args() []string
	Noop() bool
}

const maxBlockSize = 1024  //Megabytes (1GB)
const defaultBlockSize = 5 //Megabytes (5MB)

type Mode string
type Method string

const (
	Percentage Mode   = "PERCENTAGE"
	MBToFill   Mode   = "MB_TO_FILL"
	MBLeft     Mode   = "MB_LEFT"
	AtOnce     Method = "AT_ONCE"
	OverTime   Method = "OVER_TIME"
)

type Opts struct {
	BlockSize int  // in megabytes
	Size      int  // in megabytes or percentage
	Mode      Mode // PERCENTAGE or MB_TO_FILL or MB_LEFT
	TempPath  string
	Method    Method // AT_ONCE or OVER_TIME
	// Duration bounds how long the space is held, also when the extension is gone and Stop is never called.
	Duration time.Duration
}

// UnlinkedFileMessage tells users where to find the filled space, as the fill
// file has no name during the attack.
const UnlinkedFileMessage = "The disk-fill file is deleted right after it is opened and is held open until the attack ends, so the filled space is freed even if the extension is terminated. It shows up in df, but not in ls or du; lsof +L1 lists it."

// fillFd is the descriptor under which holderScript keeps the unlinked fill file open.
const fillFd = "/proc/self/fd/3"

// holderScript creates the fill file, unlinks it right away and keeps it open
// on fd 3 while the fill command writes to it via fillFd. The kernel frees the
// space as soon as the last descriptor is closed, so there is nothing to clean
// up (see BusinessMap card 26837).
// The holder ends on SIGINT/SIGTERM, at the deadline, or when its stdin reaches
// EOF. Stdin is a pipe whose write end only the extension holds, so the
// holder also ends when the extension dies, even when it runs in a sidecar
// outside the extension's cgroup: runc copies its stdin to the sidecar and
// closes it when the copy ends or runc dies; crun hands the pipe on as is.
// Background jobs get /dev/null as stdin, hence the copy to fd 4. Everything
// runs in the background so that the traps run as soon as a signal arrives; a
// trap is deferred while a foreground command runs. Only the fill may inherit
// fd 3, or the space stays allocated until that process ends. The holder waits
// for its children before it exits, as the runtime considers the sidecar
// stopped once the holder is gone.
const holderScript = `trap 'kill $fill $timer $lifeline 2>/dev/null; wait' EXIT
trap 'exit 0' INT TERM
file=$1
seconds=$2
shift 2
exec 3>"$file" || exit 1
rm -f "$file" || exit 1
exec 4<&0
{ while read -r _; do :; done; kill -TERM $$; } <&4 3>&- 4<&- &
lifeline=$!
exec 4<&-
sleep "$seconds" 3>&- &
timer=$!
"$@" &
fill=$!
wait $fill || exit
wait $timer
`

func holderArgs(file string, duration time.Duration, fillArgs []string) []string {
	seconds := int64(math.Ceil(duration.Seconds()))
	if seconds <= 0 {
		// Without a duration the space is held until Stop.
		seconds = math.MaxInt32
	}
	return append([]string{"sh", "-c", holderScript, "diskfill", file, strconv.FormatInt(seconds, 10)}, fillArgs...)
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

func isNoop(args []string) bool {
	return len(args) == 2 && args[0] == "echo" && args[1] == "noop"
}

// Args returns the fill command, which writes to the file held open by holderScript.
func (o Opts) Args(readDiskUsageFn func(path string) (*DiskUsage, error)) ([]string, error) {
	kbytesToWrite, noop, err := calculateKBytesToWrite(o, readDiskUsageFn)
	if err != nil {
		return nil, err
	}
	if kbytesToWrite <= 0 && !noop {
		return nil, errors.New("invalid size to write")
	}

	var fillArgs []string
	if noop {
		fillArgs = []string{"echo", "noop"}
	} else if o.Method == AtOnce {
		fillArgs = fallocateArgs(kbytesToWrite, fillFd)
	} else if o.Method == OverTime {
		blockSizeInKB := calculateBlockSizeKBytes(o, kbytesToWrite)
		fillArgs = ddArgs(kbytesToWrite, blockSizeInKB, fillFd)
	}
	return fillArgs, nil
}

// processArgs wraps the fill command in holderScript, which creates the fill file in dir.
func (o Opts) processArgs(fillArgs []string, dir string) []string {
	if isNoop(fillArgs) {
		return fillArgs
	}
	return holderArgs(filepath.Join(dir, "disk-fill"), o.Duration, fillArgs)
}

func levelForErr(err error) zerolog.Level {
	if errors.Is(err, ociruntime.ErrContainerNotFound) {
		return zerolog.DebugLevel
	}
	return zerolog.WarnLevel
}

func calculateBlockSizeKBytes(opts Opts, kbytesToWrites int64) int {
	blockSizeInKB := opts.BlockSize * 1024
	if blockSizeInKB < 1 {
		log.Trace().Msgf("block size %v is smaller than 1", blockSizeInKB)
		blockSizeInKB = defaultBlockSize * 1024
		log.Trace().Msgf("setting block size to %v", blockSizeInKB)
	}

	if blockSizeInKB > (maxBlockSize * 1024) {
		log.Trace().Msgf("block size %v is bigger than max block size %v", blockSizeInKB, maxBlockSize*1024)
		blockSizeInKB = maxBlockSize * 1024
		log.Trace().Msgf("setting block size to %v", blockSizeInKB)
	}

	if int64(blockSizeInKB) > kbytesToWrites {
		log.Trace().Msgf("block size %v is bigger than needed size %v", blockSizeInKB, kbytesToWrites)
		if kbytesToWrites > (1024 * 1024) {
			blockSizeInKB = 1024 * 1024
		} else {
			blockSizeInKB = int(kbytesToWrites)
		}
		log.Trace().Msgf("setting block size to %v", blockSizeInKB)
	}
	return blockSizeInKB
}

func calculateKBytesToWrite(opts Opts, readDiskUsageFn func(path string) (*DiskUsage, error)) (int64, bool, error) {
	if opts.Mode == MBToFill {
		return int64(opts.Size) * 1024, false, nil
	}

	if opts.Mode == Percentage || opts.Mode == MBLeft {
		diskSpace, err := readDiskUsageFn(opts.TempPath)
		if err != nil {
			log.Error().Err(err).Msg("failed to resolve disk space")
			return 0, false, err
		}
		if opts.Mode == Percentage {
			desiredUsage := diskSpace.Capacity * int64(opts.Size) / 100
			if diskSpace.Used >= desiredUsage {
				log.Warn().Msgf("disk is already filled up to %f%%", float64(diskSpace.Used)/float64(diskSpace.Capacity)*100)
				return 0, true, nil
			}
			bytesToWriteNeeded := desiredUsage - diskSpace.Used
			return bytesToWriteNeeded, false, nil
		} else { // MB_LEFT
			bytesToWriteNeeded := diskSpace.Available - (int64(opts.Size) * 1024)
			if bytesToWriteNeeded <= 0 {
				return 0, true, nil
			}
			return bytesToWriteNeeded, false, nil
		}
	}

	log.Error().Msgf("Invalid size unit %s", opts.Mode)
	return 0, false, fmt.Errorf("invalid size unit %s", opts.Mode)
}

func ddArgs(writeKBytes int64, blockSize int, file string) []string {
	ddPath := utils.LocateExecutable("dd", "STEADYBIT_EXTENSION_DD_PATH")
	args := []string{
		ddPath,
		"if=/dev/zero",
		fmt.Sprintf("of=%s", file),
		fmt.Sprintf("bs=%dK", blockSize),
		fmt.Sprintf("count=%d", writeKBytes/int64(blockSize)),
	}
	if log.Trace().Enabled() {
		args = append(args, "status=progress")
	}
	return args
}

func fallocateArgs(writeKBytes int64, file string) []string {
	fallocatePath := utils.LocateExecutable("fallocate", "STEADYBIT_EXTENSION_FALLOCATE_PATH")
	return []string{
		fallocatePath,
		"-l",
		fmt.Sprintf("%dKiB", writeKBytes),
		file,
	}
}

// withCLocale makes the command print untranslated messages and headers, which
// this package parses. With a host installation the extension inherits the
// host's locale.
func withCLocale(cmd *exec.Cmd) *exec.Cmd {
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	return cmd
}

func withCLocaleEnv(spec *specs.Spec) {
	spec.Process.Env = append(slices.DeleteFunc(spec.Process.Env, func(e string) bool {
		return strings.HasPrefix(e, "LC_ALL=")
	}), "LC_ALL=C")
}
