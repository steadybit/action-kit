// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package runc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

type BackgroundState struct {
	cond   *sync.Cond
	exited bool
	err    error
}

func RunBundleInBackground(ctx context.Context, runc Runc, bundle ContainerBundle) (*BackgroundState, error) {
	cmd, err := runc.RunCommand(ctx, bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to run %s: %w", bundle.ContainerId(), err)
	}

	logger := log.With().Str("id", bundle.ContainerId()).Logger()

	return RunCommandInBackground(cmd, logger)
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

func FilterNamespaces(ns []LinuxNamespace, types ...specs.LinuxNamespaceType) []LinuxNamespace {
	result := make([]LinuxNamespace, 0, len(types))
	for _, n := range ns {
		for _, t := range types {
			if n.Type == t {
				result = append(result, n)
			}
		}
	}
	return result
}

type LinuxNamespace struct {
	Type  specs.LinuxNamespaceType
	Path  string
	Inode uint64
	Pid   int
}

type LinuxProcessInfo struct {
	Pid        int
	Namespaces []LinuxNamespace
	CGroupPath string
}

func ReadLinuxProcessInfo(ctx context.Context, pid int) (LinuxProcessInfo, error) {
	ns, nsErr := ListNamespaces(ctx, pid)
	cgroup, cgroupErr := readCgroupPath(ctx, pid)
	return LinuxProcessInfo{
		Pid:        pid,
		Namespaces: ns,
		CGroupPath: cgroup,
	}, errors.Join(nsErr, cgroupErr)
}

func ListNamespaces(ctx context.Context, pid int, typ ...string) ([]LinuxNamespace, error) {
	args := []string{"--output=ns,type,path,pid", "--noheadings", "--notruncate"}

	if pid > 0 {
		args = append(args, "--task", strconv.Itoa(pid))
	}

	for _, t := range typ {
		args = append(args, "--type", t)
	}

	sout, err := executeLsns(ctx, args...)
	if err != nil {
		return nil, err
	}

	var namespaces []LinuxNamespace
	for _, line := range strings.Split(strings.TrimSpace(sout.String()), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 4 {
			continue
		}
		inode, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			log.Warn().Err(err).Msgf("failed to parse inode %s. omitting inode namespace information", fields[0])
		}
		pid, _ := strconv.Atoi(fields[3])
		ns := LinuxNamespace{
			Inode: inode,
			Type:  toRuncNamespaceType(fields[1]),
			Path:  fields[2],
			Pid:   pid,
		}
		namespaces = append(namespaces, ns)
	}
	return namespaces, nil
}

var executeLsns = executeLsnsImpl

func executeLsnsImpl(ctx context.Context, args ...string) (*bytes.Buffer, error) {
	var lastErr error
	var sout bytes.Buffer
	//due to https://github.com/util-linux/util-linux/issues/2799 we just retry
	for attempts := 0; attempts < 3; attempts++ {
		cmd := RootCommandContext(ctx, "lsns", args...)
		cmd.Stdout = &sout
		if err := cmd.Run(); err == nil {
			break
		} else {
			lastErr = err
			sout.Reset()
			var exiterr *exec.ExitError
			if errors.As(err, &exiterr) && exiterr.ExitCode() != 1 {
				break
			}
		}
	}
	return &sout, lastErr
}

func toRuncNamespaceType(t string) specs.LinuxNamespaceType {
	switch t {
	case "net":
		return specs.NetworkNamespace
	case "mnt":
		return specs.MountNamespace
	default:
		return specs.LinuxNamespaceType(t)
	}
}

func NamespacesExists(ctx context.Context, namespaces []LinuxNamespace, nsType ...specs.LinuxNamespaceType) error {
	filtered := namespaces
	if len(nsType) > 0 {
		filtered = FilterNamespaces(namespaces, nsType...)
	}

	for _, ns := range filtered {
		if ns.Path == "" {
			continue
		}

		RefreshNamespace(ctx, &ns)

		if _, err := os.Lstat(ns.Path); err != nil && os.IsNotExist(err) {
			return fmt.Errorf("namespace %s doesn't exist", ns.Path)
		}
	}

	return nil
}

func RefreshNamespaces(ctx context.Context, namespaces []LinuxNamespace, nsType ...specs.LinuxNamespaceType) {
	for i := range namespaces {
		if len(nsType) == 0 || slices.Contains(nsType, namespaces[i].Type) {
			RefreshNamespace(ctx, &namespaces[i])
		}
	}
}

func RefreshNamespace(ctx context.Context, ns *LinuxNamespace) {
	if ns == nil || ns.Inode == 0 {
		return
	}

	if _, err := os.Lstat(ns.Path); err == nil {
		return
	}

	log.Trace().Str("type", string(ns.Type)).
		Str("path", ns.Path).
		Uint64("inode", ns.Inode).
		Msg("refreshing namespace")

	if out, err := executeLsns(ctx, strconv.FormatUint(ns.Inode, 10), "--output=path", "--noheadings", "--notruncate"); err != nil {
		log.Warn().Str("type", string(ns.Type)).
			Err(err).
			Str("path", ns.Path).
			Uint64("inode", ns.Inode).
			Msg("failed refreshing namespace")
	} else {
		for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
			fields := strings.Fields(line)
			if len(fields) != 1 {
				continue
			}

			ns.Path = fields[0]
			log.Trace().Str("type", string(ns.Type)).
				Str("path", ns.Path).
				Uint64("inode", ns.Inode).
				Msg("refreshed namespace")
			return
		}
	}
}

func readCgroupPath(ctx context.Context, pid int) (string, error) {
	var out bytes.Buffer
	cmd := RootCommandContext(ctx, "nsenter", "-t", "1", "-C", "--", "cat", filepath.Join("/proc", strconv.Itoa(pid), "cgroup"))
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, out.String())
	}

	minHid := 9999
	cgroup := ""
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) != 3 {
			continue
		}
		hid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		if hid < minHid {
			minHid = hid
			cgroup = fields[2]
		}
	}
	if cgroup == "" {
		return "", fmt.Errorf("failed to read cgroup for pid %d\n%s", pid, out.String())
	}
	return cgroup, nil
}
