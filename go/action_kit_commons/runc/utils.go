// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH
//go:build !windows

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
	"golang.org/x/sync/errgroup"
	"io"
	"iter"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
var netNsOutputCleanup = regexp.MustCompile(`\s*\(id: \d+\)$`)

var executeListNamespaces = executeListNamespacesFilesystem
var findNamespaceInProcesses = findNamespaceInProcessesImpl
var errorNsNotFound = errors.New("namespace not found")

func init() {
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

func ReadLinuxProcessInfo(ctx context.Context, pid int, nsTypes ...specs.LinuxNamespaceType) (LinuxProcessInfo, error) {
	info := LinuxProcessInfo{Pid: pid}
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		ns, err := listNamespaces(gctx, pid, nsTypes...)
		if err == nil {
			info.Namespaces = ns
		}
		return err
	})

	g.Go(func() error {
		cgroup, err := readCgroupPath(gctx, pid)
		if err == nil {
			info.CGroupPath = cgroup
		}
		return err
	})

	return info, g.Wait()
}

func listNamespaces(ctx context.Context, pid int, types ...specs.LinuxNamespaceType) ([]LinuxNamespace, error) {
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
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode := exitError.ExitCode()
			if exitCode == 1 { // file does not exist
				return nil, os.ErrNotExist
			}
		}
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

// allLinuxNamespaceTypes contains known namespace types.
var allLinuxNamespaceTypes = []specs.LinuxNamespaceType{
	specs.PIDNamespace,
	specs.NetworkNamespace,
	specs.MountNamespace,
	specs.IPCNamespace,
	specs.UTSNamespace,
	specs.UserNamespace,
	specs.CgroupNamespace,
	specs.TimeNamespace,
}

// executeListNamespacesFilesystem reads namespace information from the filesystem without
// requiring an external dependency.
func executeListNamespacesFilesystem(ctx context.Context, pid int, nsTypes ...specs.LinuxNamespaceType) ([]LinuxNamespace, error) {
	if len(nsTypes) == 0 {
		nsTypes = allLinuxNamespaceTypes
	}
	var nsPaths = make([]string, len(nsTypes))
	for i, nsType := range nsTypes {
		nsPaths[i] = fmt.Sprintf("/proc/%d/ns/%s", pid, fromRuncNamespaceType(nsType))
	}

	// Use readlink and not stat as the returned links contain the inode and namespace type.
	links, err := executeReadlinkInProc(ctx, nsPaths...)
	if err != nil {
		// Don't return an error as the given pid could already be gone.
		log.Debug().Err(err).Int("pid", pid).Msg("failed to read links for namespaces of pid")
		return nil, nil
	}

	var namespaces []LinuxNamespace
	for _, link := range links {
		nsType, inode := parseInodeFromString(link)
		if inode == 0 {
			continue
		}

		// Find namespace started by a lower pid to point to a potentially more stable path.
		// Also this is needed for host network namespace check to work correctly.
		path, err := findNamespaceInProcesses(ctx, inode, nsType)
		if err != nil {
			// No better namespace found, build up path manually.
			// nsPaths cannot be used, as it may contain missing types and, hence, no result
			// in the readlink response.
			log.Warn().
				Err(err).
				Str("type", string(nsType)).
				Int("pid", pid).
				Uint64("inode", inode).
				Msg("failed refreshing namespace")
			path = fmt.Sprintf("/proc/%d/ns/%s", pid, nsType)
		}
		namespaces = append(namespaces, LinuxNamespace{
			Inode: inode,
			Type:  nsType,
			Path:  path,
		})
	}
	return namespaces, nil
}

func executeReadlinkUsingExec(ctx context.Context, nsPaths ...string) ([]string, error) {
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

func executeReadlinkUsingSyscall(_ context.Context, nsPaths ...string) ([]string, error) {
	links := make([]string, 0, len(nsPaths))
	for _, path := range nsPaths {
		link, err := os.Readlink(path)
		if err == nil {
			links = append(links, link)
		} else {
			// Ignore error, as the given path may not exist.
			log.Trace().Err(err).Str("path", path).Msg("failed to read link")
		}
	}
	return links, nil
}

func parseInodeFromString(link string) (specs.LinuxNamespaceType, uint64) {
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
	return toRuncNamespaceType(parts[0]), inode
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

		refreshNamespace(ctx, &ns)

		if _, err := executeReadInodes(ctx, ns.Path); err != nil && errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("namespace %s doesn't exist: %w", ns.Path, err)
		}
	}
	return nil
}

func RefreshNamespaces(ctx context.Context, namespaces []LinuxNamespace, nsType ...specs.LinuxNamespaceType) {
	for i := range namespaces {
		if len(nsType) == 0 || slices.Contains(nsType, namespaces[i].Type) {
			refreshNamespace(ctx, &namespaces[i])
		}
	}
}

func HasNamedNetworkNamespace(namespaces ...LinuxNamespace) bool {
	return slices.ContainsFunc(namespaces, func(ns LinuxNamespace) bool {
		return ns.Type == specs.NetworkNamespace && strings.HasPrefix(ns.Path, netNsDir)
	})
}

func refreshNamespace(ctx context.Context, ns *LinuxNamespace) {
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
	} else {
		log.Trace().
			Str("type", string(ns.Type)).
			Str("path", ns.Path).
			Uint64("inode", ns.Inode).
			Err(err).
			Msg("namespace path does not exist, refreshing")
	}

	nsPath, err := findNamespaceInProcesses(ctx, ns.Inode, ns.Type)
	if errors.Is(err, errorNsNotFound) && ns.Type == specs.NetworkNamespace {
		nsPath, err = lookupNamedNetworkNamespace(ctx, ns.Inode)
	}

	if err == nil {
		ns.Path = nsPath
		log.Trace().
			Str("type", string(ns.Type)).
			Str("path", ns.Path).
			Uint64("inode", ns.Inode).
			Msg("refreshed namespace")
	} else {
		log.Warn().
			Err(err).
			Str("type", string(ns.Type)).
			Str("path", ns.Path).
			Uint64("inode", ns.Inode).
			Msg("failed refreshing namespace")
	}
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
			netNsName := netNsOutputCleanup.ReplaceAllString(line, "")
			path := fmt.Sprintf("%s/%s", netNsDir, strings.TrimSpace(netNsName))
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

type refreshNamespaceCandidatesKey struct{}

func WithNamespaceCandidates(ctx context.Context, pids ...int) context.Context {
	return context.WithValue(ctx, refreshNamespaceCandidatesKey{}, pids)
}

// findNamespaceInProcessesImpl looks up the path to the namespace file, referencing
// the given inode and type, in /proc/*/ns with the lowest pid.
func findNamespaceInProcessesImpl(ctx context.Context, inode uint64, nsType specs.LinuxNamespaceType) (string, error) {
	//we need to check for the host pid first, for the host network detection to work correctly
	pids := []int{1}
	if c, ok := ctx.Value(refreshNamespaceCandidatesKey{}).([]int); ok && len(pids) > 0 {
		pids = append(pids, c...)
	}

	for pid, err := range concat2(toSeq2(pids...), allProcesses()) {
		if err != nil {
			return "", err
		}
		if nsPath := searchNamespacePathInProcess(ctx, inode, nsType, pid); nsPath != "" {
			return nsPath, nil
		}
	}

	return "", errorNsNotFound
}

func toSeq2[E any](elements ...E) iter.Seq2[E, error] {
	return func(yield func(E, error) bool) {
		for _, el := range elements {
			if !yield(el, nil) {
				return
			}
		}
	}
}

func concat2[K, V any](seqs ...iter.Seq2[K, V]) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, seq := range seqs {
			for k, v := range seq {
				if !yield(k, v) {
					return
				}
			}
		}
	}
}

func allProcesses() iter.Seq2[int, error] {
	return func(yield func(int, error) bool) {
		f, err := os.Open("/proc")
		if err != nil {
			yield(-1, fmt.Errorf("failed to open /proc: %w", err))
			return
		}
		defer func() { _ = f.Close() }()

		for {
			names, err := f.Readdirnames(25)
			if err == io.EOF {
				return
			} else if err != nil {
				yield(-1, fmt.Errorf("failed to list /proc: %w", err))
				return
			}

			for _, name := range names {
				pid, err := strconv.Atoi(name)
				if err == nil {
					if !yield(pid, nil) {
						return
					}
				}
			}
		}
	}
}

func searchNamespacePathInProcess(ctx context.Context, inode uint64, nsType specs.LinuxNamespaceType, pid int) string {
	nsPath := filepath.Join("/proc", strconv.Itoa(pid), "ns", fromRuncNamespaceType(nsType))
	links, err := executeReadlinkInProc(ctx, nsPath)
	if err != nil {
		return ""
	}
	for _, link := range links {
		if _, i := parseInodeFromString(link); i == inode {
			return nsPath
		}
	}
	return ""
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
