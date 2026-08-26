// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package nsrunner

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/steadybit/action-kit/go/action_kit_commons/ociruntime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestValidateStdin_RejectsNewlineInjection(t *testing.T) {
	require.NoError(t, validateStdin([]string{"*nat", "-A CHAIN -j RETURN", "COMMIT"}))
	require.Error(t, validateStdin([]string{"-A CHAIN\n-A EVIL"}))
	require.Error(t, validateStdin([]string{"ok", "bad\r"}))
}

func TestToReader_JoinsWithNewlinesAndTrailer(t *testing.T) {
	b, _ := io.ReadAll(toReader([]string{"a", "b", "c"}))
	assert.Equal(t, "a\nb\nc\n", string(b))
}

func TestNextContainerId_UniquePerCall(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := nextContainerId("iptables-restore", "exec-1")
		if !strings.Contains(id, "iptables-restore") {
			t.Fatalf("id %q lost the tool name", id)
		}
		if seen[id] {
			t.Fatalf("duplicate container id generated: %q", id)
		}
		seen[id] = true
	}
}

func TestProcessRunner_EmptyArgv(t *testing.T) {
	_, err := NewProcessRunner().Run(context.Background(), nil, nil)
	require.Error(t, err)
}

func TestProcessRunner_Identity(t *testing.T) {
	r := NewProcessRunner()
	assert.Equal(t, "host", r.ID())
	assert.Equal(t, "/proc/self/ns/net", r.NetNsPath())
}

func TestRuncRunner_Identity(t *testing.T) {
	r := NewRuncRunner(nil, SidecarOpts{TargetProcess: ociruntime.LinuxProcessInfo{
		Namespaces: []ociruntime.LinuxNamespace{{Type: specs.NetworkNamespace, Path: "/proc/42/ns/net", Inode: 4026531993}},
	}})
	assert.Equal(t, "/proc/42/ns/net", r.NetNsPath())
	assert.Equal(t, "4026531993", r.ID())
}

func TestRuncRunner_OrchestratesSidecarAndFeedsStdin(t *testing.T) {
	var createdImage, createdID, capturedStdin string
	runc := &mockRunc{}
	bundle := &mockBundle{id: "1", path: "/1"}
	bundle.On("EditSpec", mock.Anything).Return(nil)
	bundle.On("Remove").Return(nil)
	runc.On("Create", mock.Anything, mock.Anything, mock.Anything).
		Run(func(a mock.Arguments) { createdImage = a.String(1); createdID = a.String(2) }).
		Return(bundle, nil)
	runc.On("Run", mock.Anything, mock.Anything, mock.Anything).
		Run(func(a mock.Arguments) {
			if r := a.Get(2).(ociruntime.IoOpts).Stdin; r != nil {
				b, _ := io.ReadAll(r)
				capturedStdin = string(b)
			}
		}).Return(nil)
	runc.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	r := NewRuncRunner(runc, SidecarOpts{TargetProcess: ociruntime.LinuxProcessInfo{}, Id: "exec-1"})
	_, err := r.Run(context.Background(), []string{"iptables-restore", "-w", "-n"}, []string{"*nat", "-A OUTPUT -j RETURN", "COMMIT"})
	require.NoError(t, err)

	assert.Equal(t, "/", createdImage, "sidecar rootfs should be the extension's own root")
	assert.Contains(t, createdID, "iptables-restore", "container id should carry the tool name")
	assert.Equal(t, "*nat\n-A OUTPUT -j RETURN\nCOMMIT\n", capturedStdin)
	runc.AssertCalled(t, "Delete", mock.Anything, mock.Anything, true)
	bundle.AssertCalled(t, "Remove")
}

// --- mocks ---

type mockRunc struct{ mock.Mock }

func (m *mockRunc) State(ctx context.Context, id string) (*ociruntime.ContainerState, error) {
	a := m.Called(ctx, id)
	return a.Get(0).(*ociruntime.ContainerState), a.Error(1)
}
func (m *mockRunc) Create(ctx context.Context, image, id string) (ociruntime.ContainerBundle, error) {
	a := m.Called(ctx, image, id)
	return a.Get(0).(ociruntime.ContainerBundle), a.Error(1)
}
func (m *mockRunc) Run(ctx context.Context, c ociruntime.ContainerBundle, io ociruntime.IoOpts) error {
	return m.Called(ctx, c, io).Error(0)
}
func (m *mockRunc) Delete(ctx context.Context, id string, force bool) error {
	return m.Called(ctx, id, force).Error(0)
}
func (m *mockRunc) RunCommand(context.Context, ociruntime.ContainerBundle) (*exec.Cmd, error) {
	panic("unused")
}
func (m *mockRunc) Kill(context.Context, string, syscall.Signal) error { panic("unused") }

type mockBundle struct {
	mock.Mock
	id, path string
}

func (m *mockBundle) EditSpec(editors ...ociruntime.SpecEditor) error {
	return m.Called(editors).Error(0)
}
func (m *mockBundle) MountFromProcess(ctx context.Context, pid int, from, to string) error {
	return m.Called(ctx, pid, from, to).Error(0)
}
func (m *mockBundle) CopyFileFromProcess(ctx context.Context, pid int, from, to string) error {
	return m.Called(ctx, pid, from, to).Error(0)
}
func (m *mockBundle) Path() string        { return m.path }
func (m *mockBundle) ContainerId() string { return m.id }
func (m *mockBundle) Remove() error       { return m.Called().Error(0) }
