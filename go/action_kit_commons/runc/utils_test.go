package runc

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

const (
	presentInode     = uint64(1)
	nonExistentInode = uint64(9999)
	nonExistentPath  = "/doesnt-exist"
	resolvedPath     = "/resolved"
)

func Test_RefreshNamespacesUsingInode(t *testing.T) {
	executeRefreshNamespace = fakeExecuteRefresh
	defer func() { executeRefreshNamespace = executeRefreshNamespaceFilesystem }()

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

func fakeExecuteRefresh(_ context.Context, inode uint64, _ string) (string, error) {
	if inode == presentInode {
		return resolvedPath, nil
	}
	return "", fmt.Errorf("refresh error")
}
