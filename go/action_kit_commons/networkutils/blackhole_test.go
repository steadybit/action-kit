/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package networkutils

import (
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
	"testing/iotest"
)

func TestBlackholeOpts_IpCommands(t *testing.T) {
	tests := []struct {
		name      string
		opts      BlackholeOpts
		wantAddV4 []byte
		wantDelV4 []byte
		wantAddV6 []byte
		wantDelV6 []byte
		wantErr   bool
	}{
		{
			name: "blackhole",
			opts: BlackholeOpts{
				Filter: Filter{
					Include: []NetWithPortRange{
						{Net: net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)}, PortRange: PortRangeAny},
						{Net: net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, 128)}, PortRange: PortRangeAny},
					},
					Exclude: []NetWithPortRange{
						{Net: net.IPNet{IP: net.ParseIP("192.168.2.1"), Mask: net.CIDRMask(32, 32)}, PortRange: PortRange{From: 80, To: 80}},
						{Net: net.IPNet{IP: net.ParseIP("ff02::114"), Mask: net.CIDRMask(128, 128)}, PortRange: PortRange{From: 8000, To: 8999}},
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAddV4, err := tt.opts.IpCommands(FamilyV4, ModeAdd)
			if (err != nil) != tt.wantErr {
				t.Errorf("TcCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NoError(t, iotest.TestReader(gotAddV4, tt.wantAddV4))

			gotDelV4, err := tt.opts.IpCommands(FamilyV4, ModeDelete)
			if (err != nil) != tt.wantErr {
				t.Errorf("TcCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NoError(t, iotest.TestReader(gotDelV4, tt.wantDelV4))

			gotAddV6, err := tt.opts.IpCommands(FamilyV6, ModeAdd)
			if (err != nil) != tt.wantErr {
				t.Errorf("TcCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NoError(t, iotest.TestReader(gotAddV6, tt.wantAddV6))

			gotDelV6, err := tt.opts.IpCommands(FamilyV6, ModeDelete)
			if (err != nil) != tt.wantErr {
				t.Errorf("TcCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NoError(t, iotest.TestReader(gotDelV6, tt.wantDelV6))
		})
	}
}
