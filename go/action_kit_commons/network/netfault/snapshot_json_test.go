// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package netfault

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/florianl/go-tc"
)

// TestQdiscSnapshotJSONRoundtrip verifies that a qdiscSnapshot survives
// json.Marshal -> json.Unmarshal with structural identity. This is the
// gating experiment for moving the snapshot off the in-memory store and
// into the action_kit_sdk per-execution state, which serializes via JSON.
//
// If this test passes, the post-roundtrip object is byte-for-byte
// equivalent to the pre-roundtrip object and feeding it to
// Qdisc().Replace() is equivalent to feeding the original.
//
// If this test fails, fields are being lost or transformed by JSON and the
// approach needs a custom marshal layer.
func TestQdiscSnapshotJSONRoundtrip(t *testing.T) {
	original := buildRepresentativeSnapshot()

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded QdiscSnapshot
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Fatalf("roundtrip changed snapshot.\n  before: %#v\n  after:  %#v", original, decoded)
	}

	// Belt-and-braces: confirm the JSON actually carries the tuned values
	// the customer cares about. A "null everywhere" JSON would also
	// trivially roundtrip-equal, so we explicitly assert that the fq
	// buckets-log and horizon are persisted.
	if !strings.Contains(string(encoded), `"BucketsLog":15`) {
		t.Errorf("BucketsLog=15 (GKE-tuned 32768 buckets) missing from JSON output:\n%s", encoded)
	}
	if !strings.Contains(string(encoded), `"Horizon":2000000`) {
		t.Errorf("Horizon=2000000 (GKE-tuned 2s) missing from JSON output:\n%s", encoded)
	}
	if !strings.Contains(string(encoded), `"Kind":"fq"`) {
		t.Errorf("fq qdisc kind missing from JSON output:\n%s", encoded)
	}
}

// buildRepresentativeSnapshot constructs a qdiscSnapshot that covers every
// shape the restore path actually replays:
//   - mq root (kernel auto-managed, kept in snapshot for re-anchoring)
//   - fq child with tuned BucketsLog + Horizon (the customer's GKE profile)
//   - prio root (used on some EKS setups)
//   - clsact + ingress (kernel auto-managed, hook qdiscs)
//   - pfifo_fast / noqueue (kernel default leaves)
//   - htb root with class hierarchy (multi-tenant rate limiting)
//   - one filter so the Filters slice exercises the same path
func buildRepresentativeSnapshot() QdiscSnapshot {
	u32 := func(v uint32) *uint32 { return &v }

	mqRoot := tc.Object{
		Msg: tc.Msg{
			Family:  0,
			Ifindex: 2,
			Handle:  0x80260000,
			Parent:  tcHRoot,
			Info:    0,
		},
		Attribute: tc.Attribute{Kind: "mq"},
	}

	fqChild := tc.Object{
		Msg: tc.Msg{
			Family:  0,
			Ifindex: 2,
			Handle:  0x802b0000,
			Parent:  0x80260001,
			Info:    0,
		},
		Attribute: tc.Attribute{
			Kind: "fq",
			Fq: &tc.Fq{
				PLimit:     u32(10000),
				FlowPLimit: u32(100),
				Quantum:    u32(3028),
				BucketsLog: u32(15), // 32768 buckets — the GKE-tuned value
				Horizon:    u32(2000000),
				PrioMap: &tc.FqPrioQopt{
					Bands:   3,
					PrioMap: [16]uint8{1, 2, 2, 2, 1, 2, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1},
				},
				Weights: &[]int32{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			},
		},
	}

	prio := tc.Object{
		Msg: tc.Msg{Ifindex: 3, Handle: 0x10000, Parent: tcHRoot},
		Attribute: tc.Attribute{
			Kind: "prio",
			Prio: &tc.Prio{
				Bands:   3,
				PrioMap: [16]uint8{1, 2, 2, 2, 1, 2, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1},
			},
		},
	}

	// clsact and ingress share the same kernel slot
	// (TC_H_CLSACT == TC_H_INGRESS == 0xffff0000) so a real interface only
	// ever carries one. We exercise both kinds in the JSON roundtrip by
	// putting them on different interfaces (eth0 has clsact alongside mq+fq;
	// eth3 has ingress on its own). The handle + parent values match what
	// the kernel reports — handleMajor(parent) == handleMajor(handle), but
	// the topological sort treats them as roots via isRootQdisc only when
	// Parent is TC_H_ROOT or 0; here they're hook qdiscs and fall through
	// the cycle-detection fallback in orderQdiscsForRestore. That's fine for
	// the restore path (isKernelAutoManaged skips them), but it would be
	// confusing if a future test reused this fixture against ordering code,
	// so keep them apart.
	clsact := tc.Object{
		Msg:       tc.Msg{Ifindex: 2, Handle: 0xffff0000, Parent: 0xfffffff1},
		Attribute: tc.Attribute{Kind: "clsact"},
	}

	ingress := tc.Object{
		Msg:       tc.Msg{Ifindex: 6, Handle: 0xffff0000, Parent: 0xfffffff1},
		Attribute: tc.Attribute{Kind: "ingress"},
	}

	pfifoFast := tc.Object{
		Msg:       tc.Msg{Ifindex: 5, Handle: 0, Parent: tcHRoot},
		Attribute: tc.Attribute{Kind: "pfifo_fast"},
	}

	noqueue := tc.Object{
		Msg:       tc.Msg{Ifindex: 1, Handle: 0, Parent: tcHRoot},
		Attribute: tc.Attribute{Kind: "noqueue"},
	}

	htbRoot := tc.Object{
		Msg: tc.Msg{Ifindex: 4, Handle: 0x10000, Parent: tcHRoot},
		Attribute: tc.Attribute{
			Kind: "htb",
			Htb: &tc.Htb{
				Init: &tc.HtbGlob{
					Version:      0x30000,
					Rate2Quantum: 10,
					Defcls:       0,
				},
			},
		},
	}

	filter := tc.Object{
		Msg: tc.Msg{Ifindex: 2, Handle: 0x1, Parent: 0x80260001, Info: 0x300},
		Attribute: tc.Attribute{
			Kind: "u32",
		},
	}

	return QdiscSnapshot{
		NetNsID: "/proc/12345/ns/net",
		Interfaces: map[string]InterfaceSnapshot{
			"eth0": {
				Name:    "eth0",
				Ifindex: 2,
				Qdiscs:  []tc.Object{mqRoot, fqChild, clsact},
				Filters: []tc.Object{filter},
			},
			"eth3": {
				Name:    "eth3",
				Ifindex: 6,
				Qdiscs:  []tc.Object{ingress},
				Filters: nil,
			},
			"eth1": {
				Name:    "eth1",
				Ifindex: 3,
				Qdiscs:  []tc.Object{prio},
				Filters: nil,
			},
			"vethX": {
				Name:    "vethX",
				Ifindex: 5,
				Qdiscs:  []tc.Object{pfifoFast},
				Filters: nil,
			},
			"lo": {
				Name:    "lo",
				Ifindex: 1,
				Qdiscs:  []tc.Object{noqueue},
				Filters: nil,
			},
			"eth2": {
				Name:    "eth2",
				Ifindex: 4,
				Qdiscs:  []tc.Object{htbRoot},
				Filters: nil,
			},
		},
	}
}
