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
	"sort"
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

var netNsDir = "/var/run/netns"
var executeListNamespaces = executeListNamespaceLsns
var executeRefreshNamespace = executeRefreshNamespaceLsns
var errorNsNotFound = errors.New("namespace not found")

func init() {
	if os.Getenv("STEADYBIT_EXTENSION_ENABLE_INTERNAL_NAMESPACE_RESOLUTION") != "" {
		log.Info().Msgf("Enabling file based namespace handling")
		executeListNamespaces = executeListNamespacesFilesystem
		executeRefreshNamespace = executeRefreshNamespaceFilesystem
	}
	if os.Getenv("STEADYBIT_EXTENSION_NETNS_DIR") != "" {
		netNsDir = os.Getenv("STEADYBIT_EXTENSION_NETNS_DIR")
	}
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

func ListNamespaces(ctx context.Context, pid int, types ...string) ([]LinuxNamespace, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("invalid pid %d", pid)
	}

	namespaces, err := executeListNamespaces(ctx, pid, types...)
	if err != nil {
		return nil, err
	}

	sort.Slice(namespaces, func(i, j int) bool {
		return namespaces[i].Inode < namespaces[j].Inode
	})

	log.Debug().Msgf("Listed namespaces for pid %d and types %v: %+v", pid, types, namespaces)

	return namespaces, nil
}

func executeReadInodes(ctx context.Context, paths ...string) ([]uint64, error) {
	var sout, serr bytes.Buffer
	args := []string{"-t", "1", "-m", "-n", "--", "stat", "-L", "-c", "%i"}
	args = append(args, paths...)
	cmd := RootCommandContext(ctx, "nsenter", args...)
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	err := cmd.Run()

	log.Trace().
		Str("out", sout.String()).
		Str("err", serr.String()).
		Msgf("Executed stat command: %v", cmd.Args)

	if err != nil {
		log.Trace().Err(err).Msgf("failed to read inode(s) of %s", paths)
		return nil, err
	}

	var inodes []uint64
	lines := strings.Split(sout.String(), "\n")
	for _, line := range lines {
		if line != "" {
			inode, err := strconv.ParseUint(strings.TrimSpace(line), 10, 64)
			if err != nil {
				log.Trace().Err(err).Msgf("failed to parse inode %s", line)
				continue
			}
			inodes = append(inodes, inode)
		}
	}
	return inodes, nil
}

func executeListNamespaceLsns(ctx context.Context, pid int, types ...string) ([]LinuxNamespace, error) {
	args := []string{"--task", strconv.Itoa(pid), "--output=ns,type,path", "--noheadings", "--notruncate"}
	for _, t := range types {
		args = append(args, "--type", t)
	}

	sout, err := executeLsns(ctx, args...)
	if err != nil {
		return nil, err
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

func executeLsns(ctx context.Context, args ...string) (*bytes.Buffer, error) {
	var lastErr error
	var sout, serr bytes.Buffer
	//due to https://github.com/util-linux/util-linux/issues/2799 we just retry
	for attempts := 0; attempts < 5; attempts++ {
		cmd := RootCommandContext(ctx, "lsns", args...)
		cmd.Stdout = &sout
		cmd.Stderr = &serr
		if err := cmd.Run(); err == nil {
			log.Trace().
				Str("out", sout.String()).
				Str("err", serr.String()).
				Msgf("Executed lsns command: %v", cmd.Args)
			break
		} else {
			lastErr = fmt.Errorf("error executing lsns: out: %s; err:%s; cause: %w", sout.String(), serr.String(), err)
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

func fromRuncNamespaceType(namespaceType specs.LinuxNamespaceType) string {
	switch namespaceType {
	case specs.NetworkNamespace:
		return "net"
	case specs.MountNamespace:
		return "mnt"
	default:
		return string(namespaceType)
	}
}

// nsTypes contains known namespace type names.
// It does not fully match golang constants defined by type LinuxNamespaceType.
var nsTypes = []string{
	"mnt", "uts", "ipc", "pid", "net", "cgroup", "user", "time",
}

// executeListNamespacesFilesystem reads namespace information from the filesystem without
// requiring an external dependency.
func executeListNamespacesFilesystem(ctx context.Context, pid int, types ...string) ([]LinuxNamespace, error) {
	if len(types) == 0 {
		types = nsTypes
	}
	var nsPaths []string
	for _, nsType := range types {
		nsPaths = append(nsPaths, fmt.Sprintf("/proc/%d/ns/%s", pid, nsType))
	}
	// Use readlink and not stat as the returned links contain the inode and namespace type.
	links, err := executeReadlink(ctx, nsPaths...)
	if err != nil {
		// Don't return an error as the given pid could already be gone.
		return nil, nil
	}
	var namespaces []LinuxNamespace
	for _, link := range links {
		nsType, inode := parseInodeFromString(link)
		if inode == 0 {
			continue
		}
		// Find namespace started by a lower pid to point to a potentially more stable path.
		path, err := executeRefreshNamespaceFilesystem(ctx, inode, nsType)
		if err != nil {
			// No better namespace found, build up path manually.
			// nsPaths can not be used, as it may contain missing types and, hence, no result
			// in the readlink response.
			log.Warn().
				Err(err).
				Str("type", nsType).
				Int("pid", pid).
				Uint64("inode", inode).
				Msg("failed refreshing namespace")
			path = fmt.Sprintf("/proc/%d/ns/%s", pid, nsType)
		}
		ns := LinuxNamespace{
			Inode: inode,
			Type:  toRuncNamespaceType(nsType),
			Path:  path,
		}
		namespaces = append(namespaces, ns)
	}
	return namespaces, nil
}

func executeReadlink(ctx context.Context, nsPaths ...string) ([]string, error) {
	var sout, serr bytes.Buffer
	cmd := RootCommandContext(ctx, "readlink", nsPaths...)
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	err := cmd.Run()
	if err != nil {
		// If one of the given paths does not exist, readlink exits with code 1
		// but still returns the available paths. Only log the error and proceed.
		log.Trace().Err(err).
			Str("out", sout.String()).
			Str("err", serr.String()).
			Msgf("Executed readlink")
	}
	lines := strings.Split(sout.String(), "\n")
	var links []string
	for _, line := range lines {
		if line != "" {
			links = append(links, line)
		}
	}
	return links, nil
}

func parseInodeFromString(link string) (string, uint64) {
	parts := strings.Split(link, ":")
	if len(parts) != 2 {
		log.Trace().Msgf("unexpected link format %s", link)
		return "", 0
	}
	inodePart := strings.TrimRight(strings.TrimLeft(parts[1], "["), "]")
	inode, err := strconv.ParseUint(inodePart, 10, 64)
	if err != nil {
		log.Trace().Err(err).Msgf("unexpected inod format %s", inodePart)
		return "", 0
	}
	return parts[0], inode
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

		if _, err := executeReadInodes(ctx, ns.Path); err != nil && os.IsNotExist(err) {
			return fmt.Errorf("namespace %s doesn't exist: %w", ns.Path, err)
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

func HasNamedNetworkNamespace(namespaces ...LinuxNamespace) bool {
	return slices.ContainsFunc(namespaces, func(ns LinuxNamespace) bool {
		return ns.Type == specs.NetworkNamespace && strings.HasPrefix(ns.Path, netNsDir)
	})
}

func RefreshNamespace(ctx context.Context, ns *LinuxNamespace) {
	if ns == nil || ns.Inode == 0 {
		return
	}

	if HasNamedNetworkNamespace(*ns) {
		log.Trace().
			Str("type", string(ns.Type)).
			Str("path", ns.Path).
			Uint64("inode", ns.Inode).
			Msg("named network namespace, no need to refresh")
		return
	}

	if _, err := os.Lstat(ns.Path); err == nil {
		log.Trace().
			Str("type", string(ns.Type)).
			Str("path", ns.Path).
			Uint64("inode", ns.Inode).
			Msg("namespace path still existing, no need to refresh")
		return
	}

	log.Trace().
		Str("type", string(ns.Type)).
		Str("path", ns.Path).
		Uint64("inode", ns.Inode).
		Msg("refreshing namespace")

	nsPath, err := executeRefreshNamespace(ctx, ns.Inode, fromRuncNamespaceType(ns.Type))
	if errors.Is(err, errorNsNotFound) && ns.Type == specs.NetworkNamespace {
		log.Info().
			Str("type", string(ns.Type)).
			Str("path", ns.Path).
			Uint64("inode", ns.Inode).
			Msg("refreshing namespace using named network namespace lookup")
		nsPath, err = lookupNamedNetworkNamespace(ctx, ns.Inode)
		log.Info().
			Err(err).
			Str("type", string(ns.Type)).
			Str("path", nsPath).
			Uint64("inode", ns.Inode).
			Msg("refreshed namespace using named network namespace lookup")
	}
	if err != nil {
		log.Warn().
			Err(err).
			Str("type", string(ns.Type)).
			Str("path", ns.Path).
			Uint64("inode", ns.Inode).
			Msg("failed refreshing namespace")
		return
	}

	ns.Path = nsPath

	log.Trace().
		Str("type", string(ns.Type)).
		Str("path", ns.Path).
		Uint64("inode", ns.Inode).
		Msg("refreshed namespace")
}

func lookupNamedNetworkNamespace(ctx context.Context, targetInode uint64) (string, error) {
	var sout, serr bytes.Buffer
	cmd := RootCommandContext(ctx, "nsenter", "-t", "1", "-m", "-n", "--", "ip", "netns")
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	err := cmd.Run()

	log.Trace().
		Str("out", sout.String()).
		Str("err", serr.String()).
		Msgf("Executed ip command: %v", cmd.Args)
	if err != nil {
		return "", err
	}

	lines := strings.Split(sout.String(), "\n")
	for _, line := range lines {
		if line != "" {
			path := fmt.Sprintf("%s/%s", netNsDir, strings.TrimSpace(line))
			inodes, err := executeReadInodes(ctx, path)
			if err != nil {
				// Ignore error, as named network namespace could have been removed.
				continue
			}
			for _, inode := range inodes {
				if inode == targetInode {
					return path, nil
				}
			}
		}
	}

	return "", errorNsNotFound
}

// executeRefreshNamespaceLsns uses "lsns" to look up the namespace file path of the given inode and type.
func executeRefreshNamespaceLsns(ctx context.Context, inode uint64, ns string) (string, error) {
	out, err := executeLsns(ctx, strconv.FormatUint(inode, 10), "--type", ns, "--output=path", "--noheadings", "--notruncate")
	if err != nil {
		return "", errorNsNotFound
	}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 1 {
			continue
		}
		return fields[0], nil
	}
	return "", errorNsNotFound
}

// executeRefreshNamespaceFilesystem looks up the path to the namespace file, referencing
// the given inode and type, in /proc/*/ns with the lowest pid.
func executeRefreshNamespaceFilesystem(ctx context.Context, inode uint64, ns string) (string, error) {
	nsPaths, err := filepath.Glob(fmt.Sprintf("/proc/[0-9]*/ns/%s", ns))
	if err != nil {
		return "", fmt.Errorf("failed to read ns glob: %w", err)
	}
	// Sort paths as procfs does not guarantee any order.
	sort.Slice(nsPaths, func(i, j int) bool {
		return nsPaths[i] < nsPaths[j]
	})
	for _, nsPath := range nsPaths {
		links, err := executeReadlink(ctx, nsPath)
		if err != nil {
			continue
		}
		for _, link := range links {
			if _, i := parseInodeFromString(link); i == inode {
				return nsPath, nil
			}
		}
	}
	return "", errorNsNotFound
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
