// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package network

import (
	"reflect"
	"testing"
)

func Test_getMask(t *testing.T) {
	tests := []struct {
		name      string
		portRange PortRange
		want      []string
	}{
		{
			name:      "any port",
			portRange: PortRangeAny,
			want:      []string{"0 0x0000"},
		},
		{
			name:      "single port",
			portRange: PortRange{From: 80, To: 80},
			want:      []string{"80 0xffff"},
		},
		{
			name:      "range",
			portRange: PortRange{From: 1000, To: 1999},
			want:      []string{"1000 0xfff8", "1008 0xfff0", "1024 0xfe00", "1536 0xff00", "1792 0xff80", "1920 0xffc0", "1984 0xfff0"},
		},
		{
			name:      "small range",
			portRange: PortRange{From: 8080, To: 8081},
			want:      []string{"8080 0xfffe"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getMask(tt.portRange); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getMask() = %v, want %v", got, tt.want)
			}
		})
	}
}
