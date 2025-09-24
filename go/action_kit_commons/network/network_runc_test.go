// Copyright 2025 steadybit GmbH. All rights reserved.
//go:build !windows

package network

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/stretchr/testify/mock"
)

func Test_generateAndRunCommands_using_runc_should_serialize(t *testing.T) {
	blackholeOpts := BlackholeOpts{
		Filter: Filter{
			Include: []NetWithPortRange{
				mustParseNetWithPortRange("0.0.0.0/0", "*"),
			},
		},
	}

	runcMock := newMockedRunc()
	var concurrent int64
	runcMock.On("Run", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Unset()
	runcMock.On("Run", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		counter := atomic.AddInt64(&concurrent, 1)
		defer func() { atomic.AddInt64(&concurrent, -1) }()
		if counter > 1 {
			t.Errorf("concurrent run detected")
		}
		time.Sleep(10 * time.Millisecond)
	}).Return(nil)

	wg := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sidecar := SidecarOpts{
				TargetProcess: ociruntime.LinuxProcessInfo{},
				IdSuffix:      "test",
			}

			runner := NewRuncRunner(runcMock, sidecar)

			_ = Apply(context.Background(), runner, &blackholeOpts)
			defer func(ctx context.Context, runner CommandRunner, opts Opts) {
				_ = Revert(ctx, runner, opts)
			}(context.Background(), runner, &blackholeOpts)
		}()
	}
	wg.Wait()
}
