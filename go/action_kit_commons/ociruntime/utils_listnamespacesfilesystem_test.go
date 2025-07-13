// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH
//go:build !windows

package ociruntime

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"runtime"
	"strings"
	"testing"
)

func Test_ListNamespacesFilesystem(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("listNamespaces tests only run on Linux")
		return
	}
	t.Run("own pid", func(t *testing.T) {
		ns, err := executeListNamespacesFilesystem(context.Background(), os.Getpid())
		require.NoError(t, err)
		require.Len(t, ns, 8)
		require.True(t, strings.HasPrefix(ns[0].Path, "/proc/1/ns/"))
	})

	t.Run("only specific namespaces", func(t *testing.T) {
		ns, err := executeListNamespacesFilesystem(context.Background(), os.Getpid(), "pid", "cgroup")
		require.NoError(t, err)
		require.Len(t, ns, 2)
	})

	t.Run("unknown pid", func(t *testing.T) {
		pid, err := findUnusedPID()
		if err != nil {
			t.Skip("Could not find an unused PID")
		}
		ns, err := executeListNamespacesFilesystem(context.Background(), pid)
		require.NoError(t, err)
		require.Len(t, ns, 0)
	})
}

func findUnusedPID() (int, error) {
	for pid := 1; pid < 32768; pid++ {
		_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
		if os.IsNotExist(err) {
			return pid, nil
		}
	}
	return 0, fmt.Errorf("no unused PID found")
}
