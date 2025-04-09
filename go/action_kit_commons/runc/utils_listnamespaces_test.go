package runc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"testing"
	"time"
)

func Test_ListNamespaces_stress(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ListNamespaces tests only run on Linux")
		return
	}

	t.Run("compare implementations", func(t *testing.T) {
		pid := os.Getpid()

		executeListNamespaces = executeListNamespacesFilesystem
		fs, e := ListNamespaces(context.Background(), pid)
		assert.NoError(t, e, "Could not list namespaces via the filesystem")

		assert.NotEmpty(t, fs)
	})

	t.Run("stress", func(t *testing.T) {
		t.Run("filesystem", func(t *testing.T) {
			executeListNamespaces = executeListNamespacesFilesystem
			executeRefreshNamespace = executeRefreshNamespaceFilesystem
			runStressTest(t)
		})
	})
}

func runStressTest(t *testing.T) {
	pid := os.Getpid()

	timeout := 5 * time.Second
	concurrentFileSystem := 5
	concurrentProcesses := 10

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	wg := sync.WaitGroup{}
	for i := 0; i < concurrentFileSystem; i++ {
		wg.Add(1)
		go func(ctx context.Context) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					_, e := ListNamespaces(context.Background(), pid)
					assert.NoError(t, e)
					if e != nil {
						cancel()
					}
				}
			}
		}(ctx)
	}

	for i := 0; i < concurrentProcesses; i++ {
		wg.Add(1)
		go func(ctx context.Context) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					exec.Command("ls", "-l")
				}
			}
		}(ctx)
	}

	wg.Wait()
}
