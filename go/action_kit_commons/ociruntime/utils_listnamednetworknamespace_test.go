// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH
//go:build !windows

package ociruntime

import (
	"context"
	"errors"
	"fmt"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"github.com/stretchr/testify/assert"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func Test_ListNamedNetworkNamespace(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("listNamespaces tests only run on Linux")
		return
	}
	e := exec.Command("ip").Run()
	if errors.Is(e, exec.ErrNotFound) {
		return
	}

	networkNamespaceName := "namespace-test"
	sout, err := utils.RootCommandContext(context.Background(), "ip", "netns", "add", networkNamespaceName).CombinedOutput()
	assert.NoErrorf(t, err, "Output: %q", sout)
	defer func() {
		_ = utils.RootCommandContext(context.Background(), "ip", "netns", "delete", networkNamespaceName).Run()
	}()

	// Verify named network namespace is discovered in lookup by starting a new process in that namespace.
	process := utils.RootCommandContext(context.Background(), "ip", "netns", "exec", networkNamespaceName, "sleep", "60")
	err = process.Start()
	assert.NoError(t, err)
	pid := process.Process.Pid
	fmt.Printf("Started process in network namespace %q with pid %d\n", networkNamespaceName, pid)

	executeListNamespaces = executeListNamespacesFilesystem
	fs, e := listNamespaces(context.Background(), pid)
	assert.NoError(t, e, "Could not list namespaces via the filesystem")
	fsNet := FilterNamespaces(fs, specs.NetworkNamespace)

	assert.NotEmpty(t, fsNet)

	err = process.Process.Kill()
	assert.NoError(t, err)
	process.Wait()
	fmt.Printf("Stopped process in network namespace %q with pid %d\n", networkNamespaceName, pid)

	RefreshNamespaces(context.Background(), fsNet, specs.NetworkNamespace)
	assert.True(t, strings.HasPrefix(fsNet[0].Path, "/var/run/netns/"))
}
