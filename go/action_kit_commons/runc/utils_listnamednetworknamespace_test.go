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
	cmd := RootCommandContext(context.Background(), "ip", "netns", "add", networkNamespaceName)
	err := cmd.Run()
	sout, _ := cmd.Output()
	assert.NoErrorf(t, err, "Output: %q", sout)

	// Verify named network namespace is discovered in lookup by starting a new process in that namespace.
	control := make(chan struct{})
	var process *exec.Cmd
	go func() {
		process = RootCommandContext(context.Background(), "ip", "netns", "exec", networkNamespaceName, "sh", "-c", "while : ; do sleep 1 ; done")
		err = process.Start()
		assert.NoError(t, err)
		fmt.Printf("Started process in network namespace %q with pid %d\n", networkNamespaceName, cmd.Process.Pid)
		control <- struct{}{}
		<-control
		defer process.Process.Kill()
	}()

	<-control
	pid := process.Process.Pid
	executeListNamespaces = executeListNamespaceLsns
	lsns, e := ListNamespaces(context.Background(), pid)
	assert.NoError(t, e, "Could not list namespaces via lsns")
	lsnsNet := FilterNamespaces(lsns, specs.NetworkNamespace)
	assert.True(t, strings.HasPrefix(lsnsNet[0].Path, "/var/run/netns/"))

	executeListNamespaces = executeListNamespacesFilesystem
	fs, e := ListNamespaces(context.Background(), pid)
	assert.NoError(t, e, "Could not list namespaces via the filesystem")
	fsNet := FilterNamespaces(fs, specs.NetworkNamespace)
	assert.True(t, strings.HasPrefix(fsNet[0].Path, "/var/run/netns/"))

	assert.Equal(t, lsns, fs)

	control <- struct{}{}

	cmd = RootCommandContext(context.Background(), "ip", "netns", "delete", networkNamespaceName)
	err = cmd.Run()
	sout, _ = cmd.Output()
	assert.NoErrorf(t, err, "Output: %q", sout)
}
