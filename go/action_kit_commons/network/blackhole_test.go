/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package network

import (
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
	"testing/iotest"
)

func TestBlackholeOpts_IpCommands(t *testing.T) {
	tests := []struct {
		name         string
		opts         BlackholeOpts
		ipv6Disabled bool
		wantAddV4    []byte
		wantDelV4    []byte
		wantAddV6    []byte
		wantDelV6    []byte
		wantErr      bool
	}{
		{
			name: "blackhole",
			opts: BlackholeOpts{
				Filter: Filter{
					Include: []NetWithPortRange{
						mustParseNetWithPortRange("0.0.0.0/0", "*"),
						mustParseNetWithPortRange("0.0.0.0/0", "*"), //should filter that duplicate
						mustParseNetWithPortRange("::0/0", "*"),
					},
					Exclude: []NetWithPortRange{
						mustParseNetWithPortRange("192.168.2.1/32", "80"),
						mustParseNetWithPortRange("192.168.2.1/32", "80"), //should filter that duplicate
						mustParseNetWithPortRange("ff02::114/128", "8000-8999"),
					},
				},
			},
			wantAddV4: []byte(`rule add blackhole to 0.0.0.0/0 dport 1-65534
rule add blackhole from 0.0.0.0/0 sport 1-65534
rule add to 192.168.2.1/32 dport 80 table main
rule add from 192.168.2.1/32 sport 80 table main
`),
			wantDelV4: []byte(`rule del from 192.168.2.1/32 sport 80 table main
rule del to 192.168.2.1/32 dport 80 table main
rule del blackhole from 0.0.0.0/0 sport 1-65534
rule del blackhole to 0.0.0.0/0 dport 1-65534
`),
			wantAddV6: []byte(`rule add blackhole to ::/0 dport 1-65534
rule add blackhole from ::/0 sport 1-65534
rule add to ff02::114/128 dport 8000-8999 table main
rule add from ff02::114/128 sport 8000-8999 table main
`),
			wantDelV6: []byte(`rule del from ff02::114/128 sport 8000-8999 table main
rule del to ff02::114/128 dport 8000-8999 table main
rule del blackhole from ::/0 sport 1-65534
rule del blackhole to ::/0 dport 1-65534
`),
			wantErr: false,
		},
		{
			name: "blackhole udp port 123 only",
			opts: BlackholeOpts{
				IpProto: IpProtoUdp,
				Filter: Filter{
					Include: NewNetWithPortRanges(NetAny, PortRange{From: 123, To: 123}),
					Exclude: []NetWithPortRange{
						{Net: net.IPNet{IP: net.ParseIP("192.168.2.1"), Mask: net.CIDRMask(32, 32)}, PortRange: PortRange{From: 80, To: 80}},
					},
				},
			},
			wantAddV4: []byte(`rule add blackhole to 0.0.0.0/0 ipproto udp dport 123
rule add blackhole from 0.0.0.0/0 ipproto udp sport 123
`),
			wantDelV4: []byte(`rule del blackhole from 0.0.0.0/0 ipproto udp sport 123
rule del blackhole to 0.0.0.0/0 ipproto udp dport 123
`),
			wantAddV6: []byte(`rule add blackhole to ::/0 ipproto udp dport 123
rule add blackhole from ::/0 ipproto udp sport 123
`),
			wantDelV6: []byte(`rule del blackhole from ::/0 ipproto udp sport 123
rule del blackhole to ::/0 ipproto udp dport 123
`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ipv6Supported = func() bool {
				return !tt.ipv6Disabled
			}
			defer func() {
				ipv6Supported = defaultIpv6Supported
			}()

			gotAddV4, err := tt.opts.IpCommands(FamilyV4, ModeAdd)
			if (err != nil) != tt.wantErr {
				t.Errorf("TcCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NoError(t, iotest.TestReader(ToReader(gotAddV4), tt.wantAddV4))

			gotDelV4, err := tt.opts.IpCommands(FamilyV4, ModeDelete)
			if (err != nil) != tt.wantErr {
				t.Errorf("TcCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NoError(t, iotest.TestReader(ToReader(gotDelV4), tt.wantDelV4))

			gotAddV6, err := tt.opts.IpCommands(FamilyV6, ModeAdd)
			if (err != nil) != tt.wantErr {
				t.Errorf("TcCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NoError(t, iotest.TestReader(ToReader(gotAddV6), tt.wantAddV6))

			gotDelV6, err := tt.opts.IpCommands(FamilyV6, ModeDelete)
			if (err != nil) != tt.wantErr {
				t.Errorf("TcCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NoError(t, iotest.TestReader(ToReader(gotDelV6), tt.wantDelV6))
		})
	}
}
