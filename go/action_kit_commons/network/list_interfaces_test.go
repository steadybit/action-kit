// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package network

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

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
