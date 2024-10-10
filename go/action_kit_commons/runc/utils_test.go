package runc

import (
	"bytes"
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
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
	executeLsns = fakeExecuteLsns
	defer func() { executeLsns = executeLsnsImpl }()

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
			name: "resolve using lsns on non-existent path",
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
			name: "resolv using lsns on non-existent path fails",
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

func fakeExecuteLsns(_ context.Context, args ...string) (*bytes.Buffer, error) {
	if args[0] == strconv.FormatUint(presentInode, 10) {
		return bytes.NewBufferString(resolvedPath), nil
	}
	return nil, fmt.Errorf("no such inode")
}
