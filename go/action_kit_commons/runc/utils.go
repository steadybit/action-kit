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
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/trace"
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

	var outb bytes.Buffer
	pr, pw := io.Pipe()
	writer := io.MultiWriter(&outb, pw)
	cmd.Stdout = writer
	cmd.Stderr = writer

	go func() {
		defer func() { _ = pr.Close() }()
		bufReader := bufio.NewReader(pr)

		for {
			if line, err := bufReader.ReadString('\n'); err == nil {
				log.Debug().Str("id", bundle.ContainerId()).Msg(line)
			} else {
				break
			}
		}
	}()

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start %s: %w", bundle.ContainerId(), err)
	}

	result := &BackgroundState{
		cond:   &sync.Cond{L: &sync.Mutex{}},
		exited: false,
		err:    nil,
	}

	go func(r *BackgroundState) {
		defer func() { _ = pw.Close() }()
		err := cmd.Wait()
		log.Trace().Str("id", bundle.ContainerId()).Int("exitCode", cmd.ProcessState.ExitCode()).Msgf("%s exited", bundle.ContainerId())

		r.cond.L.Lock()
		defer r.cond.L.Unlock()

		r.exited = true
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitErr.Stderr = outb.Bytes()
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
}

type LinuxProcessInfo struct {
	Pid        int
	Namespaces []LinuxNamespace
	CGroupPath string
}

func ReadLinuxProcessInfo(ctx context.Context, pid int) (LinuxProcessInfo, error) {
	ns, nsErr := readNamespaces(ctx, pid)
	cgroup, cgroupErr := readCgroupPath(ctx, pid)
	return LinuxProcessInfo{
		Pid:        pid,
		Namespaces: ns,
		CGroupPath: cgroup,
	}, errors.Join(nsErr, cgroupErr)
}

func readNamespaces(ctx context.Context, pid int) ([]LinuxNamespace, error) {
	defer trace.StartRegion(ctx, "runc.readNamespaces").End()

	var sout bytes.Buffer
	var serr bytes.Buffer
	cmd := RootCommandContext(ctx, "lsns", "--task", strconv.Itoa(pid), "--output=ns,type,path", "--noheadings")
	cmd.Stdout = &sout
	cmd.Stderr = &serr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("lsns --task %d %w: %s", pid, err, serr.String())
	}

	var namespaces []LinuxNamespace
	for _, line := range strings.Split(strings.TrimSpace(sout.String()), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		inode, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			log.Warn().Err(err).Msgf("failed to parse inode %s. omitting inode namespace information", fields[0])
		}
		ns := LinuxNamespace{
			Inode: inode,
			Type:  toRuncNamespaceType(fields[1]),
			Path:  fields[2],
		}
		namespaces = append(namespaces, ns)
	}
	return namespaces, nil
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

var listNamespaceUsingInode = listNamespaceUsingInodeImpl

func NamespacesExists(ctx context.Context, namespaces []LinuxNamespace, nsType ...specs.LinuxNamespaceType) error {
	defer trace.StartRegion(ctx, "runc.NamespacesExists").End()

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
	defer trace.StartRegion(ctx, "runc.refreshNamespacesUsingInode").End()

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

	out, err := listNamespaceUsingInode(ctx, ns.Inode)

	if err != nil {
		log.Warn().Str("type", string(ns.Type)).
			Err(err).
			Str("path", ns.Path).
			Uint64("inode", ns.Inode).
			Msg("failed refreshing namespace")
	}

	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
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

func listNamespaceUsingInodeImpl(ctx context.Context, inode uint64) (string, error) {
	var sout, serr bytes.Buffer
	cmd := RootCommandContext(ctx, "lsns", strconv.FormatUint(inode, 10), "--output=path", "--noheadings")
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	if err := cmd.Run(); err != nil {
		return sout.String(), fmt.Errorf("lsns %w: %s", err, serr.String())
	}
	return sout.String(), nil
}

func readCgroupPath(ctx context.Context, pid int) (string, error) {
	defer trace.StartRegion(ctx, "runc.readCgroupPath").End()
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
