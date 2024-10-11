/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package network

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"testing/iotest"
	"time"
)

func TestDelayOpts_TcCommands(t *testing.T) {
	tests := []struct {
		name         string
		opts         DelayOpts
		ipv6Disabled bool
		wantAdd      []byte
		wantDel      []byte
		wantErr      bool
	}{
		{
			name: "delay too many filters",
			opts: DelayOpts{
				Delay:      100 * time.Millisecond,
				Jitter:     10 * time.Millisecond,
				Interfaces: []string{"eth0"},
				Filter: Filter{
					Include: []NetWithPortRange{
						mustParseNetWithPortRange("0.0.0.0/0", "*"),
					},
					Exclude: generateNWPs(2000),
				},
			},
			wantErr: true,
		},
		{
			name: "delay",
			opts: DelayOpts{
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
					}},
				Delay:      100 * time.Millisecond,
				Jitter:     10 * time.Millisecond,
				Interfaces: []string{"eth0"},
			},
			wantAdd: []byte(`qdisc add dev eth0 root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
qdisc add dev eth0 parent 1:3 handle 30: netem delay 100ms 10ms
filter add dev eth0 protocol ip parent 1: prio 1 u32 match ip src 192.168.2.1/32 match ip sport 80 0xffff flowid 1:1
filter add dev eth0 protocol ip parent 1: prio 2 u32 match ip dst 192.168.2.1/32 match ip dport 80 0xffff flowid 1:1
filter add dev eth0 protocol ipv6 parent 1: prio 3 u32 match ip6 src ff02::114/128 match ip6 sport 8000 0xffc0 flowid 1:1
filter add dev eth0 protocol ipv6 parent 1: prio 4 u32 match ip6 dst ff02::114/128 match ip6 dport 8000 0xffc0 flowid 1:1
filter add dev eth0 protocol ipv6 parent 1: prio 5 u32 match ip6 src ff02::114/128 match ip6 sport 8064 0xff80 flowid 1:1
filter add dev eth0 protocol ipv6 parent 1: prio 6 u32 match ip6 dst ff02::114/128 match ip6 dport 8064 0xff80 flowid 1:1
filter add dev eth0 protocol ipv6 parent 1: prio 7 u32 match ip6 src ff02::114/128 match ip6 sport 8192 0xfe00 flowid 1:1
filter add dev eth0 protocol ipv6 parent 1: prio 8 u32 match ip6 dst ff02::114/128 match ip6 dport 8192 0xfe00 flowid 1:1
filter add dev eth0 protocol ipv6 parent 1: prio 9 u32 match ip6 src ff02::114/128 match ip6 sport 8704 0xff00 flowid 1:1
filter add dev eth0 protocol ipv6 parent 1: prio 10 u32 match ip6 dst ff02::114/128 match ip6 dport 8704 0xff00 flowid 1:1
filter add dev eth0 protocol ipv6 parent 1: prio 11 u32 match ip6 src ff02::114/128 match ip6 sport 8960 0xffe0 flowid 1:1
filter add dev eth0 protocol ipv6 parent 1: prio 12 u32 match ip6 dst ff02::114/128 match ip6 dport 8960 0xffe0 flowid 1:1
filter add dev eth0 protocol ipv6 parent 1: prio 13 u32 match ip6 src ff02::114/128 match ip6 sport 8992 0xfff8 flowid 1:1
filter add dev eth0 protocol ipv6 parent 1: prio 14 u32 match ip6 dst ff02::114/128 match ip6 dport 8992 0xfff8 flowid 1:1
filter add dev eth0 protocol ip parent 1: prio 15 u32 match ip src 0.0.0.0/0 match ip sport 0 0x0000 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 16 u32 match ip dst 0.0.0.0/0 match ip dport 0 0x0000 flowid 1:3
filter add dev eth0 protocol ipv6 parent 1: prio 17 u32 match ip6 src ::/0 match ip6 sport 0 0x0000 flowid 1:3
filter add dev eth0 protocol ipv6 parent 1: prio 18 u32 match ip6 dst ::/0 match ip6 dport 0 0x0000 flowid 1:3
`),
			wantDel: []byte(`filter del dev eth0 protocol ipv6 parent 1: prio 18 u32 match ip6 dst ::/0 match ip6 dport 0 0x0000 flowid 1:3
filter del dev eth0 protocol ipv6 parent 1: prio 17 u32 match ip6 src ::/0 match ip6 sport 0 0x0000 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 16 u32 match ip dst 0.0.0.0/0 match ip dport 0 0x0000 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 15 u32 match ip src 0.0.0.0/0 match ip sport 0 0x0000 flowid 1:3
filter del dev eth0 protocol ipv6 parent 1: prio 14 u32 match ip6 dst ff02::114/128 match ip6 dport 8992 0xfff8 flowid 1:1
filter del dev eth0 protocol ipv6 parent 1: prio 13 u32 match ip6 src ff02::114/128 match ip6 sport 8992 0xfff8 flowid 1:1
filter del dev eth0 protocol ipv6 parent 1: prio 12 u32 match ip6 dst ff02::114/128 match ip6 dport 8960 0xffe0 flowid 1:1
filter del dev eth0 protocol ipv6 parent 1: prio 11 u32 match ip6 src ff02::114/128 match ip6 sport 8960 0xffe0 flowid 1:1
filter del dev eth0 protocol ipv6 parent 1: prio 10 u32 match ip6 dst ff02::114/128 match ip6 dport 8704 0xff00 flowid 1:1
filter del dev eth0 protocol ipv6 parent 1: prio 9 u32 match ip6 src ff02::114/128 match ip6 sport 8704 0xff00 flowid 1:1
filter del dev eth0 protocol ipv6 parent 1: prio 8 u32 match ip6 dst ff02::114/128 match ip6 dport 8192 0xfe00 flowid 1:1
filter del dev eth0 protocol ipv6 parent 1: prio 7 u32 match ip6 src ff02::114/128 match ip6 sport 8192 0xfe00 flowid 1:1
filter del dev eth0 protocol ipv6 parent 1: prio 6 u32 match ip6 dst ff02::114/128 match ip6 dport 8064 0xff80 flowid 1:1
filter del dev eth0 protocol ipv6 parent 1: prio 5 u32 match ip6 src ff02::114/128 match ip6 sport 8064 0xff80 flowid 1:1
filter del dev eth0 protocol ipv6 parent 1: prio 4 u32 match ip6 dst ff02::114/128 match ip6 dport 8000 0xffc0 flowid 1:1
filter del dev eth0 protocol ipv6 parent 1: prio 3 u32 match ip6 src ff02::114/128 match ip6 sport 8000 0xffc0 flowid 1:1
filter del dev eth0 protocol ip parent 1: prio 2 u32 match ip dst 192.168.2.1/32 match ip dport 80 0xffff flowid 1:1
filter del dev eth0 protocol ip parent 1: prio 1 u32 match ip src 192.168.2.1/32 match ip sport 80 0xffff flowid 1:1
qdisc del dev eth0 parent 1:3 handle 30: netem delay 100ms 10ms
qdisc del dev eth0 root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
`),
			wantErr: false,
		},
		{
			name: "delay with filtered excludes",
			opts: DelayOpts{
				Filter: Filter{
					Include: []NetWithPortRange{
						mustParseNetWithPortRange("192.168.2.1/24", "80"),
					},
					Exclude: []NetWithPortRange{
						mustParseNetWithPortRange("192.168.2.1/32", "80"),
						mustParseNetWithPortRange("192.168.2.1/32", "8080"), //should be filtered, wrong port range
						mustParseNetWithPortRange("ff02::114/128", "*"),     // should be filtered CIDR not overlapping
					},
				},
				Delay:      100 * time.Millisecond,
				Jitter:     10 * time.Millisecond,
				Interfaces: []string{"eth0"},
			},
			wantAdd: []byte(`qdisc add dev eth0 root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
qdisc add dev eth0 parent 1:3 handle 30: netem delay 100ms 10ms
filter add dev eth0 protocol ip parent 1: prio 1 u32 match ip src 192.168.2.1/32 match ip sport 80 0xffff flowid 1:1
filter add dev eth0 protocol ip parent 1: prio 2 u32 match ip dst 192.168.2.1/32 match ip dport 80 0xffff flowid 1:1
filter add dev eth0 protocol ip parent 1: prio 3 u32 match ip src 192.168.2.0/24 match ip sport 80 0xffff flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 4 u32 match ip dst 192.168.2.0/24 match ip dport 80 0xffff flowid 1:3
`),
			wantDel: []byte(`filter del dev eth0 protocol ip parent 1: prio 4 u32 match ip dst 192.168.2.0/24 match ip dport 80 0xffff flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 3 u32 match ip src 192.168.2.0/24 match ip sport 80 0xffff flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 2 u32 match ip dst 192.168.2.1/32 match ip dport 80 0xffff flowid 1:1
filter del dev eth0 protocol ip parent 1: prio 1 u32 match ip src 192.168.2.1/32 match ip sport 80 0xffff flowid 1:1
qdisc del dev eth0 parent 1:3 handle 30: netem delay 100ms 10ms
qdisc del dev eth0 root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
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

			gotAdd, err := tt.opts.TcCommands(ModeAdd)
			if tt.wantErr {
				if (err != nil) != tt.wantErr {
					t.Errorf("TcCommands() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			} else {
				assert.NoError(t, iotest.TestReader(ToReader(gotAdd), tt.wantAdd))
			}

			gotDel, err := tt.opts.TcCommands(ModeDelete)
			if tt.wantErr {

				if (err != nil) != tt.wantErr {
					t.Errorf("TcCommands() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			} else {
				assert.NoError(t, iotest.TestReader(ToReader(gotDel), tt.wantDel))
			}
		})
	}
}
