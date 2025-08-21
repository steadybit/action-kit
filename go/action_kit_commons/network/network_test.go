// Copyright 2025 steadybit GmbH. All rights reserved.

package network

import (
	"context"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	blackholeOpts = BlackholeOpts{
		Filter: Filter{
			Include: []NetWithPortRange{
				mustParseNetWithPortRange("0.0.0.0/0", "*"),
			},
		},
	}
)

func Test_generateAndRunCommands_should_serialize(t *testing.T) {
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
			defer Revert(context.Background(), runner, &blackholeOpts)
		}()
	}
	wg.Wait()
}

func newMockedRunc() *MockedRunc {
	bundle := MockBundle{id: "1", path: "/1"}
	bundle.On("EditSpec", mock.Anything, mock.Anything).Return(nil)
	bundle.On("Remove", mock.Anything, mock.Anything).Return(nil)
	bundle.On("CopyFileFromProcess", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	bundle.On("MountFromProcess", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	runcMock := &MockedRunc{}
	runcMock.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(&bundle, nil)
	runcMock.On("Run", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	runcMock.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return runcMock
}

type tooManyTcOpts struct {
}

func (m tooManyTcOpts) IpCommands(family Family, mode Mode) ([]string, error) {
	return nil, nil
}

func (m tooManyTcOpts) TcCommands(mode Mode) ([]string, error) {
	return make([]string, 9999), nil
}

func (m tooManyTcOpts) String() string {
	return "Too Many TC Commands"
}

type MockedRunc struct {
	mock.Mock
}

func (m *MockedRunc) State(ctx context.Context, id string) (*ociruntime.ContainerState, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*ociruntime.ContainerState), args.Error(1)
}

func (m *MockedRunc) Create(ctx context.Context, image, id string) (ociruntime.ContainerBundle, error) {
	args := m.Called(ctx, image, id)
	return args.Get(0).(ociruntime.ContainerBundle), args.Error(1)
}

func (m *MockedRunc) Run(ctx context.Context, container ociruntime.ContainerBundle, ioOpts ociruntime.IoOpts) error {
	args := m.Called(ctx, container, ioOpts)
	return args.Error(0)
}

func (m *MockedRunc) Delete(ctx context.Context, id string, force bool) error {
	args := m.Called(ctx, id, force)
	return args.Error(0)
}

func (m *MockedRunc) RunCommand(_ context.Context, _ ociruntime.ContainerBundle) (*exec.Cmd, error) {
	panic("implement me")
}

func (m *MockedRunc) Kill(_ context.Context, _ string, _ syscall.Signal) error {
	panic("implement me")
}

type MockBundle struct {
	mock.Mock
	path string
	id   string
}

func (m *MockBundle) EditSpec(editors ...ociruntime.SpecEditor) error {
	args := m.Called(editors)
	return args.Error(0)
}

func (m *MockBundle) MountFromProcess(ctx context.Context, fromPid int, fromPath, mountpoint string) error {
	args := m.Called(ctx, fromPid, fromPath, mountpoint)
	return args.Error(0)
}

func (m *MockBundle) CopyFileFromProcess(ctx context.Context, pid int, fromPath, toPath string) error {
	args := m.Called(ctx, pid, fromPath, toPath)
	return args.Error(0)
}

func (m *MockBundle) Path() string {
	return m.path
}

func (m *MockBundle) ContainerId() string {
	return m.id
}

func (m *MockBundle) Remove() error {
	args := m.Called()
	return args.Error(0)
}

func TestCondenseNetWithPortRange(t *testing.T) {
	tests := []struct {
		name  string
		nwps  []NetWithPortRange
		limit int
		want  []NetWithPortRange
	}{
		{
			name: "must not condense when limit is higher than the number of elements",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("192.168.2.2/32", "80"),
			},
			limit: 3,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("192.168.2.2/32", "80"),
			},
		},
		{
			name: "must not condense ipv6 with ipv4",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("fe80::784c:f9ff:fe48:a552/128", "80"),
			},
			limit: 1,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("fe80::784c:f9ff:fe48:a552/128", "80"),
			},
		},
		{
			name: "must not condense different port ranges",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("192.168.2.1/32", "90"),
			},
			limit: 1,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("192.168.2.1/32", "90"),
			},
		},
		{
			name: "should condense greatest common prefix",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.4/32", "80-81"), //should be condensed with next
				mustParseNetWithPortRange("192.168.2.5/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"), //should not be condensed, different ports
				mustParseNetWithPortRange("192.168.2.10/32", "80-81"),
			},
			limit: 4,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.4/31", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"),
				mustParseNetWithPortRange("192.168.2.10/32", "80-81"),
			},
		},
		{
			name: "should condense greatest common prefix further",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.4/32", "80-81"), //should be condensed with next
				mustParseNetWithPortRange("192.168.2.5/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"), //should not be condensed, different ports
				mustParseNetWithPortRange("192.168.2.10/32", "80-81"),
			},
			limit: 3,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.0/29", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"),
				mustParseNetWithPortRange("192.168.2.10/32", "80-81"),
			},
		},
		{
			name: "should condense greatest common prefix further",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.4/32", "80-81"), //should be condensed with next
				mustParseNetWithPortRange("192.168.2.5/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"), //should not be condensed, different ports
				mustParseNetWithPortRange("192.168.2.10/32", "80-81"),
			},
			limit: 2,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.0/28", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"),
			},
		},
		{
			name: "should condense",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.0.0/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.4/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.8/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.12/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.16/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.20/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.24/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.28/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.32/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.36/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.40/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.44/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.48/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.52/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.56/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.60/30", "8086-8088"),
			},
			limit: 5,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.0.0/28", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.16/28", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.32/28", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.48/29", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.56/29", "8086-8088"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CondenseNetWithPortRange(tt.nwps, tt.limit)
			assert.Equalf(t, toString(tt.want), toString(result), "CondenseNetWithPortRange(%v, %v)", tt.nwps, tt.limit)
		})
	}
}

func toString(s []NetWithPortRange) string {
	slices.SortFunc(s, NetWithPortRange.Compare)
	var sb strings.Builder
	for _, portRange := range s {
		sb.WriteString(portRange.String())
		sb.WriteString("\n")
	}
	return sb.String()
}
