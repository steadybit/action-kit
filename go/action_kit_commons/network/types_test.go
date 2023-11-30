/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package network

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParsePortRange(t *testing.T) {
	tests := []struct {
		raw     string
		want    PortRange
		wantErr assert.ErrorAssertionFunc
	}{
		{raw: "", want: PortRange{}, wantErr: assert.Error},
		{raw: "0", want: PortRange{}, wantErr: assert.Error},
		{raw: "1", want: PortRange{From: 1, To: 1}, wantErr: assert.NoError},
		{raw: "65534", want: PortRange{From: 65534, To: 65534}, wantErr: assert.NoError},
		{raw: "65535", want: PortRange{}, wantErr: assert.Error},
		{raw: "0-65534", want: PortRange{}, wantErr: assert.Error},
		{raw: "1-65534", want: PortRange{1, 65534}, wantErr: assert.NoError},
		{raw: "1-65535", want: PortRange{}, wantErr: assert.Error},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("parse \"%s\"", tt.raw), func(t *testing.T) {
			got, err := ParsePortRange(tt.raw)
			if !tt.wantErr(t, err, fmt.Sprintf("ParsePortRange(%v)", tt.raw)) {
				return
			}
			assert.Equalf(t, tt.want, got, "ParsePortRange(%v)", tt.raw)
		})
	}
}
