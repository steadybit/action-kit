// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH
//go:build !windows

package runc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"os"
	"path/filepath"
	"runtime/trace"
	"strconv"
)

type containerBundle struct {
	id         string
	path       string
	finalizers []func() error
	runc       *defaultRunc
}

func (b *containerBundle) Path() string {
	return b.path
}

func (b *containerBundle) ContainerId() string {
	return b.id
}

func (b *containerBundle) addFinalizer(f func() error) {
	b.finalizers = append(b.finalizers, f)
}

func (b *containerBundle) Remove() error {
	var errs []error
	for i := len(b.finalizers) - 1; i >= 0; i-- {
		errs = append(errs, b.finalizers[i]())
	}
	return errors.Join(errs...)
}
func (b *containerBundle) mountRootfsOverlay(ctx context.Context, image string) error {
	upper := filepath.Join(b.path, "upper")
	err := os.MkdirAll(upper, 0775)
	if err != nil {
		return fmt.Errorf("failed to create directory '%s': %w", upper, err)
	}

	work := filepath.Join(b.path, "work")
	err = os.MkdirAll(work, 0775)
	if err != nil {
		return fmt.Errorf("failed to create directory '%s': %w", work, err)
	}

	rootfs := filepath.Join(b.path, "rootfs")
	err = os.MkdirAll(rootfs, 0775)
	if err != nil {
		return fmt.Errorf("failed to create directory '%s': %w", rootfs, err)
	}

	log.Trace().
		Str("lowerdir", image).
		Str("upper", upper).
		Str("work", work).
		Str("rootfs", rootfs).
		Msg("mounting overlay")
	out, err := utils.RootCommandContext(ctx,
		"mount",
		"-t",
		"overlay",
		"-o",
		fmt.Sprintf("rw,relatime,lowerdir=%s,upperdir=%s,workdir=%s", image, upper, work),
		"overlay",
		rootfs).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	b.addFinalizer(func() error {
		return unmount(context.Background(), rootfs)
	})
	return nil
}

func (b *containerBundle) CopyFileFromProcess(ctx context.Context, pid int, fromPath, toPath string) error {
	defer trace.StartRegion(ctx, "utils.CopyFileFromProcessToBundle").End()
	var out bytes.Buffer
	cmd := utils.RootCommandContext(ctx, "cat", filepath.Join("/proc", strconv.Itoa(pid), "root", fromPath))
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, out.String())
	}

	return os.WriteFile(filepath.Join(b.path, "rootfs", toPath), out.Bytes(), 0644)
}

func (b *containerBundle) MountFromProcess(ctx context.Context, fromPid int, fromPath, toPath string) error {
	defer trace.StartRegion(ctx, "utils.MountFromProcessToBundle").End()

	mountpoint := filepath.Join(b.path, "rootfs", toPath)
	log.Trace().
		Int("fromPid", fromPid).
		Str("fromPath", fromPath).
		Str("mountpoint", mountpoint).
		Msg("mount from process to bundle")

	if err := os.Mkdir(mountpoint, 0755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create mountpoint %s: %w", mountpoint, err)
	}

	var out bytes.Buffer
	cmd := utils.RootCommandContext(ctx, b.runc.cfg.NsmountPath, strconv.Itoa(fromPid), fromPath, strconv.Itoa(os.Getpid()), mountpoint)
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, out.String())
	}
	b.addFinalizer(func() error {
		return unmount(context.Background(), mountpoint)
	})
	return nil
}

func unmount(ctx context.Context, path string) error {
	log.Trace().Str("path", path).Msg("unmounting")
	out, err := utils.RootCommandContext(ctx, "umount", "-v", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}
