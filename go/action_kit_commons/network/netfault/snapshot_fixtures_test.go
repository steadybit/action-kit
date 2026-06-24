// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package netfault

import "github.com/florianl/go-tc"

// Test fixtures modelling real-world `tc qdisc show` output from common
// production environments. Each fixture is shaped exactly as the snapshot
// layer would receive it from `Qdisc().Get()` — Ifindex, Handle and Parent
// match the kernel's representation. Handles are encoded as
// (major << 16) | minor, matching netlink wire format.
//
// These fixtures drive the tests in snapshot_realistic_test.go; the test
// file uses them to assert that orderQdiscsForRestore +
// shouldSkipQdiscOnRestore behave correctly for each environment without
// needing an actual RTNETLINK socket.

// handle encodes a major:minor pair in netlink wire format.
func handle(major, minor uint16) uint32 {
	return (uint32(major) << 16) | uint32(minor)
}

// fixtureGkeCosEth0Tuned models the customer-reported state on a GKE
// Container-Optimized OS node: an mq root with 16 fq children, all tuned by
// GKE at boot to buckets=32768, horizon=2s. This is the case that motivated
// the snapshot/restore PR — without it, `tc qdisc del root` resets every fq
// to buckets=1024, horizon=10s.
//
// Reference: customer log excerpt
//
//	qdisc mq 8026: dev eth0 root
//	qdisc fq 802b: dev eth0 parent 8026:c ... buckets 32768 horizon 2s
//	qdisc fq 8029: dev eth0 parent 8026:e ... buckets 32768 horizon 2s
//	... (14 more fq children)
func fixtureGkeCosEth0Tuned(ifindex uint32) []tc.Object {
	const mqMajor uint16 = 0x8026
	out := []tc.Object{{
		Msg:       tc.Msg{Ifindex: ifindex, Handle: handle(mqMajor, 0), Parent: tcHRoot},
		Attribute: tc.Attribute{Kind: "mq"},
	}}
	// 16 fq children, one per hardware TX queue. Handle values mirror the
	// customer's log (802b, 8029, 802d, 802f, 8031, 8035, 8033, 8028, 802a,
	// 802c, 8030, 802e, 8032, 8034, 8036, 8027), each parented to a distinct
	// minor of the mq major.
	childHandles := []uint16{0x802b, 0x8029, 0x802d, 0x802f, 0x8031, 0x8035, 0x8033, 0x8028, 0x802a, 0x802c, 0x8030, 0x802e, 0x8032, 0x8034, 0x8036, 0x8027}
	// Fq parameters: kernel stores buckets as log2 (15 == 32768 buckets),
	// horizon in microseconds, refill_delay in milliseconds, etc. We allocate
	// a fresh *tc.Fq per child so any test that later mutates one fixture
	// entry doesn't bleed into the other 15 (shared-pointer aliasing bug).
	newTunedFq := func() *tc.Fq {
		return &tc.Fq{
			BucketsLog:      uint32Ptr(15),         // 2^15 == 32768 buckets
			Horizon:         uint32Ptr(2_000_000),  // 2s == 2_000_000 us
			Quantum:         uint32Ptr(2948),
			InitQuantum:     uint32Ptr(14740),
			FlowRefillDelay: uint32Ptr(40_000_000), // 40ms in nanoseconds
			PLimit:          uint32Ptr(10000),
			FlowPLimit:      uint32Ptr(100),
		}
	}
	for i, h := range childHandles {
		parentMinor := uint16(i + 1)
		out = append(out, tc.Object{
			Msg:       tc.Msg{Ifindex: ifindex, Handle: handle(h, 0), Parent: handle(mqMajor, parentMinor)},
			Attribute: tc.Attribute{Kind: "fq", Fq: newTunedFq()},
		})
	}
	return out
}

// fixtureGkeCosLoopback models the lo interface on the same GKE COS node.
// Loopback always uses noqueue and never has children; a snapshot should
// contain just this one entry.
func fixtureGkeCosLoopback(ifindex uint32) []tc.Object {
	return []tc.Object{{
		Msg:       tc.Msg{Ifindex: ifindex, Handle: 0, Parent: tcHRoot},
		Attribute: tc.Attribute{Kind: "noqueue"},
	}}
}

// fixtureAksDefaultEth0 models a typical AKS / vanilla Ubuntu node where the
// kernel default qdisc (`fq_codel`) is attached directly at the root rather
// than under an mq. After `tc del root` the kernel re-attaches fq_codel with
// its compile-time defaults, which may or may not match the operator's
// sysctl tuning — so this is also a degradation risk, just a milder one.
func fixtureAksDefaultEth0(ifindex uint32) []tc.Object {
	return []tc.Object{{
		Msg: tc.Msg{Ifindex: ifindex, Handle: handle(0, 0), Parent: tcHRoot},
		Attribute: tc.Attribute{Kind: "fq_codel", FqCodel: &tc.FqCodel{
			Limit:  uint32Ptr(10240),
			Target: uint32Ptr(4999),
		}},
	}}
}

// fixtureEksDefaultEth0 models a typical EKS Amazon Linux 2 node: pfifo_fast
// at the root. Stateless qdisc, no tunable parameters — restoring is mostly
// a formality but exercises the non-default-kind path.
func fixtureEksDefaultEth0(ifindex uint32) []tc.Object {
	return []tc.Object{{
		Msg:       tc.Msg{Ifindex: ifindex, Handle: handle(0, 0), Parent: tcHRoot},
		Attribute: tc.Attribute{Kind: "pfifo_fast"},
	}}
}

// fixtureBareMetalHtbWithClasses models a bare-metal Linux host with a
// user-installed `htb` root and traffic-shaping classes. Used by some CNI
// bandwidth plugins and by manual `tc` shaping. Preflight refuses this kind
// of host (htb is not in safeRootQdiscKinds), so the snapshot path should
// never run here in practice — but we test the data shape anyway in case
// `force` mode is added later.
func fixtureBareMetalHtbWithClasses(ifindex uint32) []tc.Object {
	const htbMajor uint16 = 0x0001
	return []tc.Object{
		{
			Msg: tc.Msg{Ifindex: ifindex, Handle: handle(htbMajor, 0), Parent: tcHRoot},
			Attribute: tc.Attribute{Kind: "htb", Htb: &tc.Htb{
				Init: &tc.HtbGlob{Rate2Quantum: 10},
			}},
		},
		{
			Msg:       tc.Msg{Ifindex: ifindex, Handle: handle(0x0030, 0), Parent: handle(htbMajor, 0x10)},
			Attribute: tc.Attribute{Kind: "sfq", Sfq: &tc.Sfq{}},
		},
	}
}

// fixturePriorAttackResidue models the state we'd find if a previous attack
// crashed mid-revert and left its `prio` root + `netem` child behind. The
// snapshot is taken AFTER a `tc del root`, so in practice these residues
// shouldn't be in a captured snapshot — but if they are (because the user
// enabled snapshot mid-experiment), restoring them is harmless because the
// next attack will replace them. Tests that the recovery path is robust.
func fixturePriorAttackResidue(ifindex uint32) []tc.Object {
	return []tc.Object{
		{
			Msg: tc.Msg{Ifindex: ifindex, Handle: handle(1, 0), Parent: tcHRoot},
			Attribute: tc.Attribute{Kind: "prio", Prio: &tc.Prio{
				Bands:   3,
				PrioMap: [16]uint8{1, 2, 2, 2, 1, 2, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1},
			}},
		},
		{
			Msg:       tc.Msg{Ifindex: ifindex, Handle: handle(30, 0), Parent: handle(1, 3)},
			Attribute: tc.Attribute{Kind: "netem", Netem: &tc.Netem{}},
		},
	}
}

// fixtureClsactWithIngress models an interface that uses clsact (the modern
// replacement for ingress+egress as separate qdiscs) — common when BPF
// programs or modern CNIs attach to the qdisc-level hooks. clsact is in
// isKernelAutoManaged, so restore should skip it.
func fixtureClsactWithIngress(ifindex uint32) []tc.Object {
	return []tc.Object{
		{
			Msg:       tc.Msg{Ifindex: ifindex, Handle: handle(0xffff, 0), Parent: 0xfffffff1}, // TC_H_CLSACT
			Attribute: tc.Attribute{Kind: "clsact"},
		},
	}
}

// uint32Ptr is a helper for the typed go-tc structs which take pointers for
// optional fields.
func uint32Ptr(v uint32) *uint32 { return &v }
