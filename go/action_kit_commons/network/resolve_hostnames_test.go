/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

package network

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
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
