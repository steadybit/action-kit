/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package networkutils

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestHostnameResolver_Resolve(t *testing.T) {
	githubIPs, _ := net.LookupIP("github.com")
	for i, p := range githubIPs {
		githubIPs[i] = p.To16()
	}

	tests := []struct {
		ipOrHostnames []string
		want          []net.IP
		wantErr       assert.ErrorAssertionFunc
	}{
		{ipOrHostnames: []string{"", ""}, want: nil, wantErr: assert.NoError},
		{ipOrHostnames: []string{"127.0.0.1", "github.com"}, want: append([]net.IP{net.ParseIP("127.0.0.1")}, githubIPs...), wantErr: assert.NoError},
		{ipOrHostnames: []string{"not-existing.local"}, want: nil, wantErr: assert.Error},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("Resolve(%+v)", tt.ipOrHostnames), func(t *testing.T) {
			h := &HostnameResolver{}
			got, err := h.Resolve(context.Background(), tt.ipOrHostnames...)
			if !tt.wantErr(t, err, fmt.Sprintf("Resolve(ctx, %v)", tt.ipOrHostnames)) {
				return
			}
			assert.Equalf(t, tt.want, got, "Resolve(ctx, %v)", tt.ipOrHostnames)
		})
	}
}
