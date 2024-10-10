/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

package network

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestParsePortRange(t *testing.T) {
	tests := []struct {
		raw     string
		want    PortRange
		wantErr assert.ErrorAssertionFunc
	}{
		{
			raw:     "",
			wantErr: assert.Error,
		},
		{
			raw:     "0",
			wantErr: assert.Error,
		},
		{
			raw:     "1",
			want:    PortRange{From: 1, To: 1},
			wantErr: assert.NoError,
		},
		{
			raw:     "65534",
			want:    PortRange{From: 65534, To: 65534},
			wantErr: assert.NoError,
		},
		{
			raw:     "65535",
			want:    PortRange{},
			wantErr: assert.Error,
		},
		{
			raw:     "0-65534",
			want:    PortRange{},
			wantErr: assert.Error,
		},
		{
			raw:     "1-65534",
			want:    PortRange{1, 65534},
			wantErr: assert.NoError,
		},
		{
			raw:     "*",
			want:    PortRange{1, 65534},
			wantErr: assert.NoError,
		},
		{
			raw:     "1-65535",
			want:    PortRange{},
			wantErr: assert.Error,
		},
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

func TestParseCIDR(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    *net.IPNet
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "Error on empty",
			raw:     "",
			wantErr: assert.Error,
		},
		{
			name:    "Error on hostname",
			raw:     "steadybit.com",
			wantErr: assert.Error,
		},
		{
			name:    "Parse IPV4 address as CIDR",
			raw:     "192.168.2.1",
			want:    mustParse("192.168.2.1/32"),
			wantErr: assert.NoError,
		},
		{
			name:    "Parse IPV4 address with brackets as CIDR",
			raw:     "[192.168.2.1]",
			want:    mustParse("192.168.2.1/32"),
			wantErr: assert.NoError,
		},
		{
			name:    "Parse IPV4 CIDR",
			raw:     "192.168.2.1/16",
			want:    mustParse("192.168.0.0/16"),
			wantErr: assert.NoError,
		},
		{
			name:    "Parse IPV4 CIDR",
			raw:     "10.244.0.3/32",
			want:    mustParse("10.244.0.3/32"),
			wantErr: assert.NoError,
		},
		{
			name:    "Parse IPV6 address as CIDR",
			raw:     "2001:db8::8a2e:370:7334",
			want:    mustParse("2001:db8::8a2e:370:7334/128"),
			wantErr: assert.NoError,
		},
		{
			name:    "Parse IPV6 address with brackets as CIDR",
			raw:     "[2001:db8::8a2e:370:7334]",
			want:    mustParse("2001:db8::8a2e:370:7334/128"),
			wantErr: assert.NoError,
		},
		{
			name:    "Parse IPV6 CIDR",
			raw:     "2001:db8::8a2e:370:7334/112",
			want:    mustParse("2001:db8::8a2e:370:0/112"),
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCIDR(tt.raw)
			if !tt.wantErr(t, err, fmt.Sprintf("ParseCIDR(%v)", tt.raw)) {
				return
			}
			assert.Equalf(t, tt.want, got, "ParseCIDR(%v)", tt.raw)
		})
	}
}

func mustParse(want string) *net.IPNet {
	_, cidr, err := net.ParseCIDR(want)
	if err != nil {
		panic(fmt.Sprintf("could not parse %s: %v", want, err))
	}
	return cidr
}

func TestParseCIDRs(t *testing.T) {
	raw := []string{
		"",
		"steadybit.com",
		"192.168.2.1",
		"[192.168.2.1]",
		"192.168.2.1/16",
		"10.244.0.3/32",
		"2001:db8::8a2e:370:7334",
		"[2001:db8::8a2e:370:7334]",
		"2001:db8::8a2e:370:7334/112",
	}
	wantedCidrs := []net.IPNet{
		*mustParse("192.168.2.1/32"),
		*mustParse("192.168.2.1/32"),
		*mustParse("192.168.0.0/16"),
		*mustParse("10.244.0.3/32"),
		*mustParse("2001:db8::8a2e:370:7334/128"),
		*mustParse("2001:db8::8a2e:370:7334/128"),
		*mustParse("2001:db8::8a2e:370:0/112"),
	}
	wantedUnresolved := []string{"steadybit.com"}

	cidrs, unresolved := ParseCIDRs(raw)

	assert.ElementsMatchf(t, wantedCidrs, cidrs, "ParseCIDRs(%v)", raw)
	assert.ElementsMatchf(t, wantedUnresolved, unresolved, "ParseCIDRs(%v)", raw)
}

func TestPortRange_Overlap(t *testing.T) {
	tests := []struct {
		name  string
		this  PortRange
		other PortRange
		want  bool
	}{
		{
			name:  "Overlap with same range",
			this:  PortRangeAny,
			other: PortRangeAny,
			want:  true,
		},
		{
			name:  "Overlap full within range",
			this:  PortRange{From: 79, To: 81},
			other: PortRange{From: 80, To: 80},
			want:  true,
		},
		{
			name:  "Overlap within lower end",
			this:  PortRange{From: 79, To: 81},
			other: PortRange{From: 79, To: 79},
			want:  true,
		},
		{
			name:  "Overlap within upper end",
			this:  PortRange{From: 79, To: 81},
			other: PortRange{From: 81, To: 81},
			want:  true,
		},
		{
			name:  "No Overlap - below",
			this:  PortRange{From: 79, To: 81},
			other: PortRange{From: 78, To: 78},
			want:  false,
		},
		{
			name:  "No Overlap - above",
			this:  PortRange{From: 79, To: 81},
			other: PortRange{From: 82, To: 82},
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, tt.this.Overlap(tt.other), "Overlap(%v, %v)", tt.this, tt.other)
			assert.Equalf(t, tt.want, tt.other.Overlap(tt.this), "inverse Overlap(%v, %v)", tt.other, tt.this)
		})
	}
}

func TestNetWithPortRange_Overlap(t *testing.T) {
	tests := []struct {
		name  string
		this  NetWithPortRange
		other NetWithPortRange
		want  bool
	}{
		{
			name:  "Overlap with same everything",
			this:  mustParseNetWithPortRange("192.168.2.1/32", "80"),
			other: mustParseNetWithPortRange("192.168.2.1/32", "80"),
			want:  true,
		},
		{
			name:  "Overlapping CIDR and Port Range",
			this:  mustParseNetWithPortRange("192.168.2.1/24", "*"),
			other: mustParseNetWithPortRange("192.168.2.3/32", "80"),
			want:  true,
		},
		{
			name:  "No Overlapping Port Range",
			this:  mustParseNetWithPortRange("192.168.2.1/24", "1-79"),
			other: mustParseNetWithPortRange("192.168.2.3/32", "80"),
			want:  false,
		},
		{
			name:  "No Overlapping CIDR",
			this:  mustParseNetWithPortRange("192.168.2.1/24", "*"),
			other: mustParseNetWithPortRange("192.168.3.1/32", "80"),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, tt.this.Overlap(tt.other), "Overlap(%v, %v)", tt.this, tt.other)
			assert.Equalf(t, tt.want, tt.other.Overlap(tt.this), "inverse Overlap(%v, %v)", tt.other, tt.this)
		})
	}
}

func TestPortRange_Contains(t *testing.T) {
	tests := []struct {
		name string
		this PortRange
		port uint16
		want bool
	}{
		{
			name: "Contains",
			this: PortRange{From: 80, To: 80},
			port: 80,
			want: true,
		},
		{
			name: "Contains range",
			this: PortRange{From: 79, To: 81},
			port: 80,
			want: true,
		},
		{
			name: "No Contains - below",
			this: PortRange{From: 80, To: 80},
			port: 79,
		},
		{
			name: "No Contains - above",
			this: PortRange{From: 80, To: 80},
			port: 81,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, tt.this.Contains(tt.port), "Contains(%v, %v)", tt.this, tt.port)
		})
	}
}

func Test_deduplicateNetWithPortRange(t *testing.T) {
	tests := []struct {
		name string
		arg  []NetWithPortRange
		want []NetWithPortRange
	}{
		{name: "Empty"},
		{
			name: "Simple Duplicate",
			arg: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
			},
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
			},
		},
		{
			name: "Already covered by port range",
			arg: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("192.168.2.1/32", "80-8999"),
			},
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80-8999"),
			},
		},
		{
			name: "Already covered by cidr",
			arg: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("192.168.2.0/24", "80"),
			},
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.0/24", "80"),
			},
		},
		{
			name: "Cannot be deduped",
			arg: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
				mustParseNetWithPortRange("192.168.2.0/24", "8080"),
			},
			want: []NetWithPortRange{
				mustParseNetWithPortRange("192.168.2.0/24", "8080"),
				mustParseNetWithPortRange("192.168.2.1/32", "80"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, deduplicateNetWithPortRange(tt.arg), "deduplicateNetWithPortRange(%v)", tt.arg)
		})
	}
}
