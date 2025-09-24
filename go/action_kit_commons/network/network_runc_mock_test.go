// Copyright 2025 steadybit GmbH. All rights reserved.
//go:build !windows

package network

import (
	"context"
	"os/exec"
	"syscall"

	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/stretchr/testify/mock"
)

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
