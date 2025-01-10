package runc

import (
	"context"
	"errors"
	"fmt"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func Test_ListNamedNetworkNamespace(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ListNamespaces tests only run on Linux")
		return
	}
	e := exec.Command("lsns").Run()
	if errors.Is(e, exec.ErrNotFound) {
		t.Skip("lsns not available or permitted, skipped")
		return
	}
	e = exec.Command("ip").Run()
	if errors.Is(e, exec.ErrNotFound) {
		return
	}

	networkNamespaceName := "namespace-test"
	sout, err := RootCommandContext(context.Background(), "ip", "netns", "add", networkNamespaceName).CombinedOutput()
	assert.NoErrorf(t, err, "Output: %q", sout)
	defer func() {
		_ = RootCommandContext(context.Background(), "ip", "netns", "delete", networkNamespaceName).Run()
	}()

	// Verify named network namespace is discovered in lookup by starting a new process in that namespace.
	process := RootCommandContext(context.Background(), "ip", "netns", "exec", networkNamespaceName, "sleep", "60")
	err = process.Start()
	assert.NoError(t, err)
	pid := process.Process.Pid
	fmt.Printf("Started process in network namespace %q with pid %d\n", networkNamespaceName, pid)

	executeListNamespaces = executeListNamespaceLsns
	lsns, e := ListNamespaces(context.Background(), pid)
	assert.NoError(t, e, "Could not list namespaces via lsns")
	lsnsNet := FilterNamespaces(lsns, specs.NetworkNamespace)

	executeListNamespaces = executeListNamespacesFilesystem
	fs, e := ListNamespaces(context.Background(), pid)
	assert.NoError(t, e, "Could not list namespaces via the filesystem")
	fsNet := FilterNamespaces(fs, specs.NetworkNamespace)

	assert.Equal(t, lsns, fs)

	err = process.Process.Kill()
	assert.NoError(t, err)
	process.Wait()
	fmt.Printf("Stopped process in network namespace %q with pid %d\n", networkNamespaceName, pid)

	RefreshNamespaces(context.Background(), lsnsNet, specs.NetworkNamespace)
	assert.True(t, strings.HasPrefix(lsnsNet[0].Path, "/var/run/netns/"))

	RefreshNamespaces(context.Background(), fsNet, specs.NetworkNamespace)
	assert.True(t, strings.HasPrefix(fsNet[0].Path, "/var/run/netns/"))
}
