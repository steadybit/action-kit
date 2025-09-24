// Copyright 2025 steadybit GmbH. All rights reserved.
//go:build !windows

package network

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"github.com/stretchr/testify/assert"
)

func Test_run_command_using_ip_netns_in_named_network_namespace(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("listNamespaces tests only run on Linux")
		return
	}
	err := exec.Command("ip").Run()
	if errors.Is(err, exec.ErrNotFound) {
		t.Skip("ip command not found, skipping test")
		return
	}

	// Create named network namespace
	networkNamespaceName := "namespace-test"
	sout, err := utils.RootCommandContext(context.Background(), "ip", "netns", "add", networkNamespaceName).CombinedOutput()
	assert.NoErrorf(t, err, "Output: %q", sout)
	t.Cleanup(func() {
		_ = utils.RootCommandContext(context.Background(), "ip", "netns", "delete", networkNamespaceName).Run()
	})

	// Manually look up inode of named network namespace
	sout, err = utils.RootCommandContext(context.Background(), "ip", "netns", "exec", networkNamespaceName, "stat", "-L", "-c", "%i", fmt.Sprintf("/var/run/netns/%s", networkNamespaceName)).CombinedOutput()
	assert.NoErrorf(t, err, "Output: %q", sout)
	inode, err := strconv.Atoi(strings.TrimSpace(string(sout)))
	assert.NoError(t, err)

	// Simulate stopped process with named network namespace
	runner := NewRuncRunner(newMockedRunc(), SidecarOpts{
		TargetProcess: ociruntime.LinuxProcessInfo{
			Pid: 99999,
			Namespaces: []ociruntime.LinuxNamespace{
				{
					Type:  specs.NetworkNamespace,
					Path:  "/proc/99999/ns/net",
					Inode: uint64(inode),
				},
			},
		},
	})

	// Execute any command in named network namespace context
	_, err = runner.run(t.Context(), []string{"ls"}, []string{})
	assert.NoError(t, err)
}
