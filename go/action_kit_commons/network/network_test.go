// Copyright 2025 steadybit GmbH. All rights reserved.
//go:build !windows

package network

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCondenseNetWithPortRange(t *testing.T) {
	tests := []struct {
		name  string
		nwps  []NetWithPortRange
		limit int
		want  []NetWithPortRange
	}{
		{
			name: "must not condense when limit is higher than the number of elements",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("192.168.2.2/32", "80"),
			},
			limit: 3,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("192.168.2.2/32", "80"),
			},
		},
		{
			name: "must not condense ipv6 with ipv4",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("fe80::784c:f9ff:fe48:a552/128", "80"),
			},
			limit: 1,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("fe80::784c:f9ff:fe48:a552/128", "80"),
			},
		},
		{
			name: "must not condense different port ranges",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("192.168.2.1/32", "90"),
			},
			limit: 1,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("192.168.2.1/32", "90"),
			},
		},
		{
			name: "should condense greatest common prefix",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.4/32", "80-81"), //should be condensed with next
				mustParseNetWithPortRange("192.168.2.5/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"), //should not be condensed, different ports
				mustParseNetWithPortRange("192.168.2.10/32", "80-81"),
			},
			limit: 4,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.4/31", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"),
				mustParseNetWithPortRange("192.168.2.10/32", "80-81"),
			},
		},
		{
			name: "should condense greatest common prefix further",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.4/32", "80-81"), //should be condensed with next
				mustParseNetWithPortRange("192.168.2.5/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"), //should not be condensed, different ports
				mustParseNetWithPortRange("192.168.2.10/32", "80-81"),
			},
			limit: 3,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.0/29", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"),
				mustParseNetWithPortRange("192.168.2.10/32", "80-81"),
			},
		},
		{
			name: "should condense greatest common prefix further",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.4/32", "80-81"), //should be condensed with next
				mustParseNetWithPortRange("192.168.2.5/32", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"), //should not be condensed, different ports
				mustParseNetWithPortRange("192.168.2.10/32", "80-81"),
			},
			limit: 2,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.0/28", "80-81"),
				mustParseNetWithPortRange("192.168.2.6/32", "80-8080"),
			},
		},
		{
			name: "should condense",
			nwps: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.0.0/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.4/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.8/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.12/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.16/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.20/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.24/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.28/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.32/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.36/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.40/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.44/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.48/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.52/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.56/30", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.60/30", "8086-8088"),
			},
			limit: 5,
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.0.0/28", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.16/28", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.32/28", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.48/29", "8086-8088"),
				mustParseNetWithPortRange("192.168.0.56/29", "8086-8088"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CondenseNetWithPortRange(tt.nwps, tt.limit)
			assert.Equalf(t, toString(tt.want), toString(result), "CondenseNetWithPortRange(%v, %v)", tt.nwps, tt.limit)
		})
	}
}

func toString(s []NetWithPortRange) string {
	slices.SortFunc(s, NetWithPortRange.Compare)
	var sb strings.Builder
	for _, portRange := range s {
		sb.WriteString(portRange.String())
		sb.WriteString("\n")
	}
	return sb.String()
}
