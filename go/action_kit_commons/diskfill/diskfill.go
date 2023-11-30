// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package diskfill

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/runc"
	"path/filepath"
	"syscall"
	"time"
)

type DiskFill struct {
	bundle runc.ContainerBundle
	runc   runc.Runc

	state *runc.BackgroundState
	args  []string
}

const maxBlockSize = 1024  //Megabytes (1GB)
const defaultBlockSize = 5 //Megabytes (5MB)
const mountpointInContainer = "/disk-fill-temp"
const fileInContainer = "/disk-fill-temp/disk-fill"

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
}

func New(ctx context.Context, r runc.Runc, sidecar SidecarOpts, opts Opts) (*DiskFill, error) {
	kbytesToWrite, err := calculateKBytesToWrite(ctx, r, sidecar, opts)
	if err != nil {
		return nil, err
	}
	if kbytesToWrite <= 0 {
		return nil, errors.New("invalid size to write")
	}

	var processArgs []string
	if opts.Method == AtOnce {
		processArgs = fallocateArgs(kbytesToWrite)
	} else if opts.Method == OverTime {
		blockSizeInKB := calculateBlockSizeKBytes(opts, kbytesToWrite)
		processArgs = ddArgs(kbytesToWrite, blockSizeInKB)
	}

	bundle, err := createBundle(ctx, r, sidecar, opts, processArgs...)
	if err != nil {
		log.Error().Err(err).Msg("failed to create start bundle")
		return nil, err
	}

	return &DiskFill{
		bundle: bundle,
		runc:   r,
		args:   processArgs,
	}, nil
}

func (df *DiskFill) Exited() (bool, error) {
	return df.state.Exited()
}

func (df *DiskFill) Start() error {
	log.Info().
		Str("containerId", df.bundle.ContainerId()).
		Strs("args", df.args).
		Msg("Starting diskfill")

	if state, err := runc.RunBundleInBackground(context.Background(), df.runc, df.bundle); err != nil {
		return fmt.Errorf("failed to start diskfill: %w", err)
	} else {
		df.state = state
	}
	return nil
}

func (df *DiskFill) Stop() error {
	log.Info().
		Str("containerId", df.bundle.ContainerId()).
		Msg("stopping diskfill")
	ctx := context.Background()

	//stop writer
	if err := df.runc.Kill(ctx, df.bundle.ContainerId(), syscall.SIGINT); err != nil {
		log.Warn().Str("id", df.bundle.ContainerId()).Err(err).Msg("failed to send SIGINT to container")
	}

	timerStart := time.AfterFunc(10*time.Second, func() {
		if err := df.runc.Kill(ctx, df.bundle.ContainerId(), syscall.SIGTERM); err != nil {
			log.Warn().Str("id", df.bundle.ContainerId()).Err(err).Msg("failed to send SIGTERM to container")
		}
	})

	df.state.Wait()
	timerStart.Stop()

	// remove file
	var deleteFileErr error
	fileToRemove := filepath.Join(df.bundle.Path(), "rootfs", fileInContainer)
	if out, err := runc.RootCommandContext(ctx, "rm", fileToRemove).CombinedOutput(); err != nil {
		log.Error().Err(err).Msgf("failed to remove file %s", out)
		deleteFileErr = errors.New(fmt.Sprintf("failed to remove file %s! You have to remove it manually now! %s", fileToRemove, out))
	} else {
		log.Info().Msgf("removed file %s: %s", fileToRemove, out)
	}

	if err := df.runc.Delete(ctx, df.bundle.ContainerId(), false); err != nil {
		log.Warn().Str("id", df.bundle.ContainerId()).Err(err).Msg("failed to delete container")
	}

	if err := df.bundle.Remove(); err != nil {
		log.Warn().Str("id", df.bundle.ContainerId()).Err(err).Msg("failed to remove bundle")
	}
	return deleteFileErr
}

func (df *DiskFill) Args() []string {
	return df.args
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

func calculateKBytesToWrite(ctx context.Context, r runc.Runc, sidecar SidecarOpts, opts Opts) (int64, error) {
	if opts.Mode == MBToFill {
		return int64(opts.Size) * 1024, nil
	}

	if opts.Mode == Percentage || opts.Mode == MBLeft {
		diskSpace, err := readDiskUsage(ctx, r, sidecar, opts)
		if err != nil {
			log.Error().Err(err).Msg("failed to resolve disk space")
			return 0, err
		}
		if opts.Mode == Percentage {
			desiredUsage := diskSpace.Capacity * int64(opts.Size) / 100
			if diskSpace.Used >= desiredUsage {
				return 0, fmt.Errorf("disk is already filled up to %f%%", float64(diskSpace.Used)/float64(diskSpace.Capacity)*100)
			}
			bytesToWriteNeeded := desiredUsage - diskSpace.Used
			return bytesToWriteNeeded, nil
		} else { // MB_LEFT
			return diskSpace.Available - (int64(opts.Size) * 1024), nil
		}
	}

	log.Error().Msgf("Invalid size unit %s", opts.Mode)
	return 0, fmt.Errorf("invalid size unit %s", opts.Mode)
}

func ddArgs(writeKBytes int64, blockSize int) []string {
	args := []string{
		"dd",
		"if=/dev/zero",
		fmt.Sprintf("of=%s", fileInContainer),
		fmt.Sprintf("bs=%dK", blockSize),
		fmt.Sprintf("count=%d", writeKBytes/int64(blockSize)),
	}
	if log.Trace().Enabled() {
		args = append(args, "status=progress")
	}
	return args
}

func fallocateArgs(writeKBytes int64) []string {
	return []string{
		"fallocate",
		"-l",
		fmt.Sprintf("%dKiB", writeKBytes),
		fileInContainer,
	}
}
