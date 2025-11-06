// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package ociruntime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

const (
	presentInode     = uint64(1)
	nonExistentInode = uint64(9999)
	nonExistentPath  = "/doesnt-exist"
	resolvedPath     = "/resolved"
)

func Test_RefreshNamespaces(t *testing.T) {
	findNamespaceInProcesses = fakeFindNamespaceInProcesses
	defer func() { findNamespaceInProcesses = findNamespaceInProcessesImpl }()

	tests := []struct {
		name     string
		ns       []LinuxNamespace
		wantedNs []LinuxNamespace
	}{
		{
			name: "do nothing on nil",
		},
		{
			name: "do nothing on missing inode",
			ns: []LinuxNamespace{{
				Inode: 0,
			}},
			wantedNs: []LinuxNamespace{{
				Inode: 0,
			}},
		},
		{
			name: "do nothing on present path",
			ns: []LinuxNamespace{{
				Path:  filepath.Join("proc", strconv.Itoa(os.Getpid()), "ns", "net"),
				Inode: nonExistentInode,
			}},
			wantedNs: []LinuxNamespace{{
				Path:  filepath.Join("proc", strconv.Itoa(os.Getpid()), "ns", "net"),
				Inode: nonExistentInode,
			}},
		},
		{
			name: "resolve using filesystem on non-existent path",
			ns: []LinuxNamespace{{
				Path:  nonExistentPath,
				Inode: presentInode,
			}},
			wantedNs: []LinuxNamespace{{
				Path:  resolvedPath,
				Inode: presentInode,
			}},
		},
		{
			name: "resolve using filesystem on non-existent path fails",
			ns: []LinuxNamespace{{
				Path:  nonExistentPath,
				Inode: nonExistentInode,
			}},
			wantedNs: []LinuxNamespace{{
				Path:  nonExistentPath,
				Inode: nonExistentInode,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RefreshNamespaces(context.Background(), tt.ns)
			assert.Equal(t, tt.wantedNs, tt.ns)
		})
	}
}

func Test_NamespaceExists(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("NamespaceExists tests only run on Linux")
		return
	}
	pid := os.Getpid()

	tests := []struct {
		name          string
		ns            []LinuxNamespace
		expectedError error
	}{
		{
			name: "existing namespace",
			ns: []LinuxNamespace{{
				Path: fmt.Sprintf("/proc/%d/ns/net", pid),
			}},
			expectedError: nil,
		}, {
			name: "missing namespace",
			ns: []LinuxNamespace{{
				Path: "/proc/65432/ns/net",
			}},
			expectedError: os.ErrNotExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NamespacesExists(context.Background(), tt.ns)
			if tt.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tt.expectedError)
			}
		})
	}
}

func fakeFindNamespaceInProcesses(_ context.Context, inode uint64, _ specs.LinuxNamespaceType) (string, error) {
	if inode == presentInode {
		return resolvedPath, nil
	}
	return "", fmt.Errorf("refresh error")
}

func Test_parseProcCgroupFile(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "valid cgroup v1 output",
			content: `11:freezer:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
10:pids:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
9:memory:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
8:blkio:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
7:hugetlb:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
6:devices:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
5:cpuset:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
4:cpu,cpuacct:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
3:perf_event:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
2:net_cls,net_prio:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
1:name=systemd:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
`,
			want: "/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope",
		},
		{
			name:    "valid cgroup v2 output",
			content: `0::/system.slice/docker.service`,
			want:    "/system.slice/docker.service",
		},
		{
			name: "mixed cgroup v1 + v2 output - v1 used",
			content: `11:freezer:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
10:pids:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
9:memory:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
8:blkio:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
7:hugetlb:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
6:devices:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
5:cpuset:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
4:cpu,cpuacct:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
3:perf_event:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
2:net_cls,net_prio:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
1:name=systemd:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
0::/`,
			want: "/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope",
		},
		{
			name: "mixed cgroup v1 + v2 output - v2used",
			content: `11:freezer:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope
10:pids:/
9:memory:/
8:blkio:/
7:hugetlb:/
6:devices:/
5:cpuset:/
4:cpu,cpuacct:/
3:perf_event:/
2:net_cls,net_prio:/
1:name=systemd:/
0::/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope`,
			want: "/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod70565f42_b704_42f2_83fe_1ffb77a67800.slice/cri-containerd-a678b5127d3f1a1bfd029818c628a8cd106d7b873ff64cd6bfaa7022579cbab5.scope",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseProcCgroupFile(tt.content)
			assert.Equalf(t, tt.want, got, "parseProcCgroupFile(%v bytes)", len(tt.content))
		})
	}
}
