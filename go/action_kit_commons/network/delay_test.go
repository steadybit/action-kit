// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package network

import (
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
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
		{
			name: "delay with TCP PSH flag filtering",
			opts: DelayOpts{
				Filter: Filter{
					Include: []NetWithPortRange{
						mustParseNetWithPortRange("192.168.1.0/24", "80"),
						mustParseNetWithPortRange("10.0.0.0/8", "443"),
					},
				},
				Delay:      200 * time.Millisecond,
				Jitter:     20 * time.Millisecond,
				Interfaces: []string{"eth0"},
				TcpPshOnly: true,
			},
			wantAdd: []byte(`qdisc add dev eth0 root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
qdisc add dev eth0 parent 1:3 handle 30: netem delay 200ms 20ms
filter add dev eth0 protocol ip parent 1: prio 1 u32 match ip protocol 6 0xff match ip src 10.0.0.0/8 match u16 443 0xffff at 20 match u8 0x08 0x08 at 33 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 2 u32 match ip protocol 6 0xff match ip dst 10.0.0.0/8 match u16 443 0xffff at 22 match u8 0x08 0x08 at 33 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 3 u32 match ip protocol 17 0xff match ip src 10.0.0.0/8 match u16 443 0xffff at 20 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 4 u32 match ip protocol 17 0xff match ip dst 10.0.0.0/8 match u16 443 0xffff at 22 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 5 u32 match ip protocol 6 0xff match ip src 192.168.1.0/24 match u16 80 0xffff at 20 match u8 0x08 0x08 at 33 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 6 u32 match ip protocol 6 0xff match ip dst 192.168.1.0/24 match u16 80 0xffff at 22 match u8 0x08 0x08 at 33 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 7 u32 match ip protocol 17 0xff match ip src 192.168.1.0/24 match u16 80 0xffff at 20 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 8 u32 match ip protocol 17 0xff match ip dst 192.168.1.0/24 match u16 80 0xffff at 22 flowid 1:3
`),
			wantDel: []byte(`filter del dev eth0 protocol ip parent 1: prio 8 u32 match ip protocol 17 0xff match ip dst 192.168.1.0/24 match u16 80 0xffff at 22 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 7 u32 match ip protocol 17 0xff match ip src 192.168.1.0/24 match u16 80 0xffff at 20 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 6 u32 match ip protocol 6 0xff match ip dst 192.168.1.0/24 match u16 80 0xffff at 22 match u8 0x08 0x08 at 33 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 5 u32 match ip protocol 6 0xff match ip src 192.168.1.0/24 match u16 80 0xffff at 20 match u8 0x08 0x08 at 33 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 4 u32 match ip protocol 17 0xff match ip dst 10.0.0.0/8 match u16 443 0xffff at 22 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 3 u32 match ip protocol 17 0xff match ip src 10.0.0.0/8 match u16 443 0xffff at 20 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 2 u32 match ip protocol 6 0xff match ip dst 10.0.0.0/8 match u16 443 0xffff at 22 match u8 0x08 0x08 at 33 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 1 u32 match ip protocol 6 0xff match ip src 10.0.0.0/8 match u16 443 0xffff at 20 match u8 0x08 0x08 at 33 flowid 1:3
qdisc del dev eth0 parent 1:3 handle 30: netem delay 200ms 20ms
qdisc del dev eth0 root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
`),
			wantErr: false,
		},
		{
			name: "delay with TCP PSH flag filtering and IPv6",
			opts: DelayOpts{
				Filter: Filter{
					Include: []NetWithPortRange{
						mustParseNetWithPortRange("2001:db8::/32", "8080"),
					},
				},
				Delay:      150 * time.Millisecond,
				Jitter:     15 * time.Millisecond,
				Interfaces: []string{"eth0"},
				TcpPshOnly: true,
			},
			wantAdd: []byte(`qdisc add dev eth0 root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
qdisc add dev eth0 parent 1:3 handle 30: netem delay 150ms 15ms
filter add dev eth0 protocol ipv6 parent 1: prio 1 u32 match ip6 nexthdr 6 0xff match ip6 src 2001:db8::/32 match u16 8080 0xffff at 40 match u8 0x08 0x08 at 53 flowid 1:3
filter add dev eth0 protocol ipv6 parent 1: prio 2 u32 match ip6 nexthdr 6 0xff match ip6 dst 2001:db8::/32 match u16 8080 0xffff at 42 match u8 0x08 0x08 at 53 flowid 1:3
filter add dev eth0 protocol ipv6 parent 1: prio 3 u32 match ip6 nexthdr 17 0xff match ip6 src 2001:db8::/32 match u16 8080 0xffff at 40 flowid 1:3
filter add dev eth0 protocol ipv6 parent 1: prio 4 u32 match ip6 nexthdr 17 0xff match ip6 dst 2001:db8::/32 match u16 8080 0xffff at 42 flowid 1:3
`),
			wantDel: []byte(`filter del dev eth0 protocol ipv6 parent 1: prio 4 u32 match ip6 nexthdr 17 0xff match ip6 dst 2001:db8::/32 match u16 8080 0xffff at 42 flowid 1:3
filter del dev eth0 protocol ipv6 parent 1: prio 3 u32 match ip6 nexthdr 17 0xff match ip6 src 2001:db8::/32 match u16 8080 0xffff at 40 flowid 1:3
filter del dev eth0 protocol ipv6 parent 1: prio 2 u32 match ip6 nexthdr 6 0xff match ip6 dst 2001:db8::/32 match u16 8080 0xffff at 42 match u8 0x08 0x08 at 53 flowid 1:3
filter del dev eth0 protocol ipv6 parent 1: prio 1 u32 match ip6 nexthdr 6 0xff match ip6 src 2001:db8::/32 match u16 8080 0xffff at 40 match u8 0x08 0x08 at 53 flowid 1:3
qdisc del dev eth0 parent 1:3 handle 30: netem delay 150ms 15ms
qdisc del dev eth0 root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
`),
			wantErr: false,
		},
		{
			name: "delay with TCP PSH flag filtering and port ranges",
			opts: DelayOpts{
				Filter: Filter{
					Include: []NetWithPortRange{
						mustParseNetWithPortRange("192.168.1.0/24", "80-85"),
					},
				},
				Delay:      100 * time.Millisecond,
				Jitter:     10 * time.Millisecond,
				Interfaces: []string{"eth0"},
				TcpPshOnly: true,
			},
			wantAdd: []byte(`qdisc add dev eth0 root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
qdisc add dev eth0 parent 1:3 handle 30: netem delay 100ms 10ms
filter add dev eth0 protocol ip parent 1: prio 1 u32 match ip protocol 6 0xff match ip src 192.168.1.0/24 match u16 80 0xfffc at 20 match u8 0x08 0x08 at 33 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 2 u32 match ip protocol 6 0xff match ip dst 192.168.1.0/24 match u16 80 0xfffc at 22 match u8 0x08 0x08 at 33 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 3 u32 match ip protocol 17 0xff match ip src 192.168.1.0/24 match u16 80 0xfffc at 20 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 4 u32 match ip protocol 17 0xff match ip dst 192.168.1.0/24 match u16 80 0xfffc at 22 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 5 u32 match ip protocol 6 0xff match ip src 192.168.1.0/24 match u16 84 0xfffe at 20 match u8 0x08 0x08 at 33 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 6 u32 match ip protocol 6 0xff match ip dst 192.168.1.0/24 match u16 84 0xfffe at 22 match u8 0x08 0x08 at 33 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 7 u32 match ip protocol 17 0xff match ip src 192.168.1.0/24 match u16 84 0xfffe at 20 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 8 u32 match ip protocol 17 0xff match ip dst 192.168.1.0/24 match u16 84 0xfffe at 22 flowid 1:3
`),
			wantDel: []byte(`filter del dev eth0 protocol ip parent 1: prio 8 u32 match ip protocol 17 0xff match ip dst 192.168.1.0/24 match u16 84 0xfffe at 22 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 7 u32 match ip protocol 17 0xff match ip src 192.168.1.0/24 match u16 84 0xfffe at 20 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 6 u32 match ip protocol 6 0xff match ip dst 192.168.1.0/24 match u16 84 0xfffe at 22 match u8 0x08 0x08 at 33 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 5 u32 match ip protocol 6 0xff match ip src 192.168.1.0/24 match u16 84 0xfffe at 20 match u8 0x08 0x08 at 33 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 4 u32 match ip protocol 17 0xff match ip dst 192.168.1.0/24 match u16 80 0xfffc at 22 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 3 u32 match ip protocol 17 0xff match ip src 192.168.1.0/24 match u16 80 0xfffc at 20 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 2 u32 match ip protocol 6 0xff match ip dst 192.168.1.0/24 match u16 80 0xfffc at 22 match u8 0x08 0x08 at 33 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 1 u32 match ip protocol 6 0xff match ip src 192.168.1.0/24 match u16 80 0xfffc at 20 match u8 0x08 0x08 at 33 flowid 1:3
qdisc del dev eth0 parent 1:3 handle 30: netem delay 100ms 10ms
qdisc del dev eth0 root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
`),
			wantErr: false,
		},
		{
			name: "delay with TCP PSH flag filtering and multiple interfaces",
			opts: DelayOpts{
				Filter: Filter{
					Include: []NetWithPortRange{
						mustParseNetWithPortRange("192.168.1.0/24", "80"),
					},
				},
				Delay:      100 * time.Millisecond,
				Jitter:     10 * time.Millisecond,
				Interfaces: []string{"eth0", "eth1"},
				TcpPshOnly: true,
			},
			wantAdd: []byte(`qdisc add dev eth0 root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
qdisc add dev eth0 parent 1:3 handle 30: netem delay 100ms 10ms
filter add dev eth0 protocol ip parent 1: prio 1 u32 match ip protocol 6 0xff match ip src 192.168.1.0/24 match u16 80 0xffff at 20 match u8 0x08 0x08 at 33 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 2 u32 match ip protocol 6 0xff match ip dst 192.168.1.0/24 match u16 80 0xffff at 22 match u8 0x08 0x08 at 33 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 3 u32 match ip protocol 17 0xff match ip src 192.168.1.0/24 match u16 80 0xffff at 20 flowid 1:3
filter add dev eth0 protocol ip parent 1: prio 4 u32 match ip protocol 17 0xff match ip dst 192.168.1.0/24 match u16 80 0xffff at 22 flowid 1:3
qdisc add dev eth1 root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
qdisc add dev eth1 parent 1:3 handle 30: netem delay 100ms 10ms
filter add dev eth1 protocol ip parent 1: prio 1 u32 match ip protocol 6 0xff match ip src 192.168.1.0/24 match u16 80 0xffff at 20 match u8 0x08 0x08 at 33 flowid 1:3
filter add dev eth1 protocol ip parent 1: prio 2 u32 match ip protocol 6 0xff match ip dst 192.168.1.0/24 match u16 80 0xffff at 22 match u8 0x08 0x08 at 33 flowid 1:3
filter add dev eth1 protocol ip parent 1: prio 3 u32 match ip protocol 17 0xff match ip src 192.168.1.0/24 match u16 80 0xffff at 20 flowid 1:3
filter add dev eth1 protocol ip parent 1: prio 4 u32 match ip protocol 17 0xff match ip dst 192.168.1.0/24 match u16 80 0xffff at 22 flowid 1:3
`),
			wantDel: []byte(`filter del dev eth1 protocol ip parent 1: prio 4 u32 match ip protocol 17 0xff match ip dst 192.168.1.0/24 match u16 80 0xffff at 22 flowid 1:3
filter del dev eth1 protocol ip parent 1: prio 3 u32 match ip protocol 17 0xff match ip src 192.168.1.0/24 match u16 80 0xffff at 20 flowid 1:3
filter del dev eth1 protocol ip parent 1: prio 2 u32 match ip protocol 6 0xff match ip dst 192.168.1.0/24 match u16 80 0xffff at 22 match u8 0x08 0x08 at 33 flowid 1:3
filter del dev eth1 protocol ip parent 1: prio 1 u32 match ip protocol 6 0xff match ip src 192.168.1.0/24 match u16 80 0xffff at 20 match u8 0x08 0x08 at 33 flowid 1:3
qdisc del dev eth1 parent 1:3 handle 30: netem delay 100ms 10ms
qdisc del dev eth1 root handle 1: prio priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
filter del dev eth0 protocol ip parent 1: prio 4 u32 match ip protocol 17 0xff match ip dst 192.168.1.0/24 match u16 80 0xffff at 22 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 3 u32 match ip protocol 17 0xff match ip src 192.168.1.0/24 match u16 80 0xffff at 20 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 2 u32 match ip protocol 6 0xff match ip dst 192.168.1.0/24 match u16 80 0xffff at 22 match u8 0x08 0x08 at 33 flowid 1:3
filter del dev eth0 protocol ip parent 1: prio 1 u32 match ip protocol 6 0xff match ip src 192.168.1.0/24 match u16 80 0xffff at 20 match u8 0x08 0x08 at 33 flowid 1:3
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

func TestDelayOpts_String(t *testing.T) {
	tests := []struct {
		name string
		opts DelayOpts
		want string
	}{
		{
			name: "delay without PSH flag",
			opts: DelayOpts{
				Filter: Filter{
					Include: []NetWithPortRange{
						mustParseNetWithPortRange("192.168.1.0/24", "80"),
					},
				},
				Delay:      100 * time.Millisecond,
				Jitter:     10 * time.Millisecond,
				Interfaces: []string{"eth0"},
				TcpPshOnly: false,
			},
			want: "delaying traffic by 100ms (jitter: 10ms, interfaces: eth0) including 192.168.1.0/24:80",
		},
		{
			name: "delay with PSH flag",
			opts: DelayOpts{
				Filter: Filter{
					Include: []NetWithPortRange{
						mustParseNetWithPortRange("192.168.1.0/24", "80"),
					},
				},
				Delay:      100 * time.Millisecond,
				Jitter:     10 * time.Millisecond,
				Interfaces: []string{"eth0"},
				TcpPshOnly: true,
			},
			want: "delaying traffic by 100ms (jitter: 10ms, interfaces: eth0) including 192.168.1.0/24:80",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetMatchersForDelay(t *testing.T) {
	tests := []struct {
		name         string
		nwp          NetWithPortRange
		tcpPshOnly   bool
		wantMatchers []string
		wantErr      bool
	}{
		{
			name:       "IPv4 without PSH flag",
			nwp:        mustParseNetWithPortRange("192.168.1.0/24", "80"),
			tcpPshOnly: false,
			wantMatchers: []string{
				"match ip src 192.168.1.0/24 match ip sport 80 0xffff",
				"match ip dst 192.168.1.0/24 match ip dport 80 0xffff",
			},
			wantErr: false,
		},
		{
			name:       "IPv4 with PSH flag",
			nwp:        mustParseNetWithPortRange("192.168.1.0/24", "80"),
			tcpPshOnly: true,
			wantMatchers: []string{
				"match ip protocol 6 0xff match ip src 192.168.1.0/24 match u16 80 0xffff at 20 match u8 0x08 0x08 at 33",
				"match ip protocol 6 0xff match ip dst 192.168.1.0/24 match u16 80 0xffff at 22 match u8 0x08 0x08 at 33",
				"match ip protocol 17 0xff match ip src 192.168.1.0/24 match u16 80 0xffff at 20",
				"match ip protocol 17 0xff match ip dst 192.168.1.0/24 match u16 80 0xffff at 22",
			},
			wantErr: false,
		},
		{
			name:       "IPv6 with PSH flag",
			nwp:        mustParseNetWithPortRange("2001:db8::/32", "8080"),
			tcpPshOnly: true,
			wantMatchers: []string{
				"match ip6 nexthdr 6 0xff match ip6 src 2001:db8::/32 match u16 8080 0xffff at 40 match u8 0x08 0x08 at 53",
				"match ip6 nexthdr 6 0xff match ip6 dst 2001:db8::/32 match u16 8080 0xffff at 42 match u8 0x08 0x08 at 53",
				"match ip6 nexthdr 17 0xff match ip6 src 2001:db8::/32 match u16 8080 0xffff at 40",
				"match ip6 nexthdr 17 0xff match ip6 dst 2001:db8::/32 match u16 8080 0xffff at 42",
			},
			wantErr: false,
		},
		{
			name:       "IPv4 with PSH flag and port range",
			nwp:        mustParseNetWithPortRange("192.168.1.0/24", "80-82"),
			tcpPshOnly: true,
			wantMatchers: []string{
				"match ip protocol 6 0xff match ip src 192.168.1.0/24 match u16 80 0xfffe at 20 match u8 0x08 0x08 at 33",
				"match ip protocol 6 0xff match ip dst 192.168.1.0/24 match u16 80 0xfffe at 22 match u8 0x08 0x08 at 33",
				"match ip protocol 17 0xff match ip src 192.168.1.0/24 match u16 80 0xfffe at 20",
				"match ip protocol 17 0xff match ip dst 192.168.1.0/24 match u16 80 0xfffe at 22",
				"match ip protocol 6 0xff match ip src 192.168.1.0/24 match u16 82 0xffff at 20 match u8 0x08 0x08 at 33",
				"match ip protocol 6 0xff match ip dst 192.168.1.0/24 match u16 82 0xffff at 22 match u8 0x08 0x08 at 33",
				"match ip protocol 17 0xff match ip src 192.168.1.0/24 match u16 82 0xffff at 20",
				"match ip protocol 17 0xff match ip dst 192.168.1.0/24 match u16 82 0xffff at 22",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMatchers, err := getMatchersForDelay(tt.nwp, tt.tcpPshOnly)
			if (err != nil) != tt.wantErr {
				t.Errorf("getMatchersForDelay() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !assert.Equal(t, tt.wantMatchers, gotMatchers) {
				t.Errorf("getMatchersForDelay() = %v, want %v", gotMatchers, tt.wantMatchers)
			}
		})
	}
}

func TestGetTcpPortOffsets(t *testing.T) {
	tests := []struct {
		name    string
		family  Family
		wantSrc int
		wantDst int
	}{
		{
			name:    "IPv4",
			family:  FamilyV4,
			wantSrc: 20,
			wantDst: 22,
		},
		{
			name:    "IPv6",
			family:  FamilyV6,
			wantSrc: 40,
			wantDst: 42,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getTcpSrcPortOffset(tt.family); got != tt.wantSrc {
				t.Errorf("getTcpSrcPortOffset() = %v, want %v", got, tt.wantSrc)
			}
			if got := getTcpDstPortOffset(tt.family); got != tt.wantDst {
				t.Errorf("getTcpDstPortOffset() = %v, want %v", got, tt.wantDst)
			}
		})
	}
}
