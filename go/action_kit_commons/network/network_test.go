// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package network

import (
	"context"
	"github.com/steadybit/action-kit/go/action_kit_commons/runc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
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
				TargetProcess: runc.LinuxProcessInfo{},
				IdSuffix:      "test",
			}

			_ = Apply(context.Background(), runcMock, sidecar, &blackholeOpts)
			defer Revert(context.Background(), runcMock, sidecar, &blackholeOpts)
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

func Test_generateAndRunCommands_should_error_when_too_many(t *testing.T) {
	runcMock := newMockedRunc()
	sidecar := SidecarOpts{
		TargetProcess: runc.LinuxProcessInfo{},
		IdSuffix:      "test",
	}

	err := Apply(context.Background(), runcMock, sidecar, &tooManyTcOpts{})
	defer Revert(context.Background(), runcMock, sidecar, &blackholeOpts)

	var target *ErrTooManyTcCommands
	assert.ErrorAs(t, err, &target)
}

type MockedRunc struct {
	mock.Mock
}

func (m *MockedRunc) State(ctx context.Context, id string) (*runc.ContainerState, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*runc.ContainerState), args.Error(1)
}

func (m *MockedRunc) Create(ctx context.Context, image, id string) (runc.ContainerBundle, error) {
	args := m.Called(ctx, image, id)
	return args.Get(0).(runc.ContainerBundle), args.Error(1)
}

func (m *MockedRunc) Run(ctx context.Context, container runc.ContainerBundle, ioOpts runc.IoOpts) error {
	args := m.Called(ctx, container, ioOpts)
	return args.Error(0)
}

func (m *MockedRunc) Delete(ctx context.Context, id string, force bool) error {
	args := m.Called(ctx, id, force)
	return args.Error(0)
}

func (m *MockedRunc) RunCommand(_ context.Context, _ runc.ContainerBundle) (*exec.Cmd, error) {
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

func (m *MockBundle) EditSpec(editors ...runc.SpecEditor) error {
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
