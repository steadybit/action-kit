// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package netfault

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasIstioRedirect(t *testing.T) {
	nftablesEmpty := "*nat\n:PREROUTING ACCEPT [0:0]\n:INPUT ACCEPT [0:0]\n:OUTPUT ACCEPT [0:0]\n:POSTROUTING ACCEPT [0:0]\nCOMMIT\n"
	legacyWithIstio := "*nat\n:PREROUTING ACCEPT [0:0]\n:ISTIO_REDIRECT - [0:0]\n:ISTIO_OUTPUT - [0:0]\nCOMMIT\n"
	nftablesWithIstio := "*nat\n:PREROUTING ACCEPT [0:0]\n:ISTIO_REDIRECT - [0:0]\nCOMMIT\n"

	tests := []struct {
		name    string
		runner  CommandRunner
		want    bool
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "no istio in either backend",
			runner: &mockRunnerMulti{responses: map[string]mockResponse{
				"iptables-save":        {stdout: nftablesEmpty},
				"iptables-legacy-save": {stdout: nftablesEmpty},
			}},
			want:    false,
			wantErr: assert.NoError,
		},
		{
			name: "istio in nftables backend",
			runner: &mockRunnerMulti{responses: map[string]mockResponse{
				"iptables-save": {stdout: nftablesWithIstio},
			}},
			want:    true,
			wantErr: assert.NoError,
		},
		{
			name: "istio in legacy backend",
			runner: &mockRunnerMulti{responses: map[string]mockResponse{
				"iptables-save":        {stdout: nftablesEmpty},
				"iptables-legacy-save": {stdout: legacyWithIstio},
			}},
			want:    true,
			wantErr: assert.NoError,
		},
		{
			name: "legacy-save not available",
			runner: &mockRunnerMulti{responses: map[string]mockResponse{
				"iptables-save":        {stdout: nftablesEmpty},
				"iptables-legacy-save": {err: fmt.Errorf("exec: iptables-legacy-save: not found")},
			}},
			want:    false,
			wantErr: assert.NoError,
		},
		{
			name: "empty output and legacy-save not available",
			runner: &mockRunnerMulti{responses: map[string]mockResponse{
				"iptables-save":        {stdout: ""},
				"iptables-legacy-save": {err: fmt.Errorf("exec: iptables-legacy-save: not found")},
			}},
			want:    false,
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HasIstioRedirect(context.Background(), tt.runner)
			if !tt.wantErr(t, err, "HasIstioRedirect()") {
				return
			}
			assert.Equalf(t, tt.want, got, "HasIstioRedirect()")
		})
	}
}

func TestHasCiliumIpRoutes(t *testing.T) {
	tests := []struct {
		name    string
		stdout  string
		want    bool
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "no routes",
			stdout:  "[]",
			want:    false,
			wantErr: assert.NoError,
		},
		{
			name:    "cilium",
			stdout:  "[{\"dst\":\"default\",\"gateway\":\"192.168.58.1\",\"dev\":\"eth0\",\"flags\":[]},{\"dst\":\"10.0.0.0/24\",\"gateway\":\"10.0.0.24\",\"dev\":\"cilium_host\",\"protocol\":\"kernel\",\"prefsrc\":\"10.0.0.24\",\"flags\":[]},{\"dst\":\"10.0.0.24\",\"dev\":\"cilium_host\",\"protocol\":\"kernel\",\"scope\":\"link\",\"flags\":[]},{\"dst\":\"10.244.0.0/16\",\"dev\":\"bridge\",\"protocol\":\"kernel\",\"scope\":\"link\",\"prefsrc\":\"10.244.0.1\",\"flags\":[]},{\"dst\":\"172.17.0.0/16\",\"dev\":\"docker0\",\"protocol\":\"kernel\",\"scope\":\"link\",\"prefsrc\":\"172.17.0.1\",\"flags\":[\"linkdown\"]},{\"dst\":\"192.168.58.0/24\",\"dev\":\"eth0\",\"protocol\":\"kernel\",\"scope\":\"link\",\"prefsrc\":\"192.168.58.2\",\"flags\":[]}]",
			want:    true,
			wantErr: assert.NoError,
		},
		{
			name:    "no cilium",
			stdout:  "[{\"dst\":\"default\",\"gateway\":\"192.168.58.1\",\"dev\":\"eth0\",\"flags\":[]},{\"dst\":\"10.244.0.0/16\",\"dev\":\"bridge\",\"protocol\":\"kernel\",\"scope\":\"link\",\"prefsrc\":\"10.244.0.1\",\"flags\":[]},{\"dst\":\"172.17.0.0/16\",\"dev\":\"docker0\",\"protocol\":\"kernel\",\"scope\":\"link\",\"prefsrc\":\"172.17.0.1\",\"flags\":[\"linkdown\"]},{\"dst\":\"192.168.58.0/24\",\"dev\":\"eth0\",\"protocol\":\"kernel\",\"scope\":\"link\",\"prefsrc\":\"192.168.58.2\",\"flags\":[]}]",
			want:    false,
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HasCiliumIpRoutes(context.Background(), &mockRunner{stdout: tt.stdout})
			if !tt.wantErr(t, err, "HasCiliumIpRoutes()") {
				return
			}
			assert.Equalf(t, tt.want, got, "HasCiliumIpRoutes()")
		})
	}
}

type mockRunner struct {
	stdout string
	err    error
}

func (m mockRunner) run(_ context.Context, _ []string, _ []string) (string, error) {
	return m.stdout, m.err
}

func (m mockRunner) id() string {
	return "mock"
}

type mockResponse struct {
	stdout string
	err    error
}

type mockRunnerMulti struct {
	responses map[string]mockResponse
}

func (m *mockRunnerMulti) run(_ context.Context, processArgs []string, _ []string) (string, error) {
	if len(processArgs) > 0 {
		if resp, ok := m.responses[processArgs[0]]; ok {
			return resp.stdout, resp.err
		}
	}
	return "", fmt.Errorf("unexpected command: %v", processArgs)
}

func (m *mockRunnerMulti) id() string {
	return "mock-multi"
}
