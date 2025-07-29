// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package network

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
	"net"
	"os/exec"
	"testing"
)

func TestHostnameResolver_Resolve(t *testing.T) {
	steadybitIPs, _ := net.LookupIP("steadybit.com")
	for i, p := range steadybitIPs {
		steadybitIPs[i] = p.To16()
	}

	tests := []struct {
		hostnames []string
		want      []net.IP
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			hostnames: []string{""},
			want:      []net.IP{},
			wantErr:   assert.Error,
		},
		{
			hostnames: []string{" "},
			wantErr:   assert.Error,
		},
		{
			hostnames: []string{"not-existing.local"},
			wantErr:   assert.Error,
		},
		{
			hostnames: []string{"127.0.0.1"},
			wantErr:   assert.Error,
		},
		{
			hostnames: []string{"steadybit.com"},
			want:      steadybitIPs,
			wantErr:   assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Resolve(%+v)", tt.hostnames), func(t *testing.T) {
			got, err := Resolve(context.Background(), tt.hostnames...)
			if !tt.wantErr(t, err, fmt.Sprintf("Resolve(ctx, %v)", tt.hostnames)) {
				return
			}
			assert.ElementsMatchf(t, tt.want, got, "Resolve(ctx, %v)", tt.hostnames)
		})
	}
}

func TestHostnameResolver_ResolveHostnames_errorhandling(t *testing.T) {
	tests := []struct {
		output  []byte
		err     error
		want    []net.IP
		wantErr assert.ErrorAssertionFunc
	}{
		{
			output: []byte(""),
			want:   []net.IP{},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "could not resolve hostnames: 'steadybit.com'", i...)
			},
		},
		{
			output: []byte(`STEADybit.com.          IN      A       141.193.213.11
STEADybit.com.          IN      A       141.193.213.10`),
			want:    []net.IP{net.ParseIP("141.193.213.11"), net.ParseIP("141.193.213.10")},
			wantErr: assert.NoError,
		},
		{
			output: []byte(`STEADybit.com.          IN      A       141.193.213.11
STEADybit.com.          IN      A       141.193.213.10`),
			want:    []net.IP{net.ParseIP("141.193.213.11"), net.ParseIP("141.193.213.10")},
			wantErr: assert.NoError,
		},
		{
			output: []byte(`;; communications error to 127.1.1.1#53: connection refused
;; communications error to 127.1.1.1#53: connection refused
;; communications error to 127.1.1.1#53: connection refused
;; no servers could be reached`),
			err:  &exec.ExitError{},
			want: []net.IP{},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, `could not resolve hostnames: <nil>
communications error to 127.1.1.1#53: connection refused
communications error to 127.1.1.1#53: connection refused
communications error to 127.1.1.1#53: connection refused
no servers could be reached`, i...)
			},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Resolve(%+v)", i), func(t *testing.T) {
			resolver := &HostnameResolver{Dig: &mockDigRunner{
				tt.output,
				tt.err,
			}}

			got, err := resolver.Resolve(context.Background(), "steadybit.com")
			if !tt.wantErr(t, err) {
				return
			}
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

type mockDigRunner struct {
	output []byte
	err    error
}

func (m mockDigRunner) Run(ctx context.Context, arg []string, stdin io.Reader) ([]byte, error) {
	return m.output, m.err
}
