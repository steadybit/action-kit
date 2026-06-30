// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package netfault

import (
	"testing"

	"github.com/florianl/go-tc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These regression tests cover bugs discovered on a real GKE Standard
// cluster and missed by the original unit suite. Each test is named after
// the root-cause condition it pins down so a future change that re-breaks
// the bug fails the test by name.

// TestStripRuntimeStats_ZeroesAllCounterFields covers the "functionality
// not yet implemented" failure in restore.
//
// Background: a real test against a tuned GKE-style host (mq 8026: + 2x fq
// 802b:/8029: buckets=32768 horizon=2s) failed restore with
//
//	netlink receive: functionality not yet implemented
//
// because go-tc's Qdisc().Get() populates Stats/XStats/Stats2 from the
// kernel, and validateQdiscObject (go-tc qdisc.go:174) refuses any
// non-DELETE request whose object carries any of those fields:
//
//	if (info.Stats != nil || info.XStats != nil || info.Stats2 != nil) && action != unix.RTM_DELQDISC {
//	    return options, ErrNotImplemented
//	}
//
// stripRuntimeStats is the targeted shim called before every Replace() to
// neutralise this. This test asserts it does exactly that — and nothing
// else (kind, handle, parent, kind-specific attribute pointers must
// survive).
func TestStripRuntimeStats_ZeroesAllCounterFields(t *testing.T) {
	stats := &tc.Stats{}
	xstats := &tc.XStats{}
	stats2 := &tc.Stats2{}
	fq := &tc.Fq{BucketsLog: uint32Ptr(15), Horizon: uint32Ptr(2_000_000)}

	obj := tc.Object{
		Msg: tc.Msg{Ifindex: 2, Handle: handle(0x802b, 0), Parent: handle(0x8026, 1)},
		Attribute: tc.Attribute{
			Kind:   "fq",
			Stats:  stats,
			XStats: xstats,
			Stats2: stats2,
			Fq:     fq,
		},
	}

	stripRuntimeStats(&obj)

	// counter fields zeroed
	assert.Nil(t, obj.Stats, "Stats must be cleared so Replace doesn't reject the object")
	assert.Nil(t, obj.XStats, "XStats must be cleared")
	assert.Nil(t, obj.Stats2, "Stats2 must be cleared")
	// settable fields preserved
	assert.Equal(t, "fq", obj.Kind)
	assert.Equal(t, handle(0x802b, 0), obj.Handle)
	assert.Equal(t, handle(0x8026, 1), obj.Parent)
	require.NotNil(t, obj.Fq, "Fq attribute must survive — restore must re-apply tuned params")
	require.NotNil(t, obj.Fq.BucketsLog)
	assert.Equal(t, uint32(15), *obj.Fq.BucketsLog, "tuned BucketsLog must survive the strip")
	require.NotNil(t, obj.Fq.Horizon)
	assert.Equal(t, uint32(2_000_000), *obj.Fq.Horizon)
}

// TestReAnchorAutoManagedParents_RewritesGkeStyleMqChildren covers the
// ENOENT (`netlink receive: no such file or directory`) failure in restore.
//
// Background: after `tc qdisc del root`, the kernel re-attaches mq with a
// fresh handle (commonly 0:), not whatever boot-time tuning assigned
// (e.g. GKE COS uses 8026:). Children in the saved snapshot still carry
// Parent=8026:N, so Replace fails because no qdisc with major 8026 exists
// on the interface anymore.
//
// reAnchorAutoManagedParents reads the live tree, finds the auto-managed
// root of matching kind, and rewrites each saved child's Parent.major to
// point at the live root. This test pins that behaviour down with the
// exact handles from the customer's GKE COS log.
func TestReAnchorAutoManagedParents_RewritesGkeStyleMqChildren(t *testing.T) {
	const eth0 uint32 = 2
	savedMqMajor := uint32(0x80260000)
	liveMqMajor := uint32(0x00000000) // kernel re-attaches mq with handle 0:

	saved := InterfaceSnapshot{
		Name:    "eth0",
		Ifindex: eth0,
		Qdiscs: []tc.Object{
			// saved root: mq at 8026:0
			{Msg: tc.Msg{Ifindex: eth0, Handle: savedMqMajor, Parent: tcHRoot}, Attribute: tc.Attribute{Kind: "mq"}},
			// saved children: fq 802b: under 8026:1, fq 8029: under 8026:2
			{Msg: tc.Msg{Ifindex: eth0, Handle: handle(0x802b, 0), Parent: savedMqMajor | 1}, Attribute: tc.Attribute{Kind: "fq", Fq: &tc.Fq{BucketsLog: uint32Ptr(15)}}},
			{Msg: tc.Msg{Ifindex: eth0, Handle: handle(0x8029, 0), Parent: savedMqMajor | 2}, Attribute: tc.Attribute{Kind: "fq", Fq: &tc.Fq{BucketsLog: uint32Ptr(15)}}},
		},
	}

	// live tree after `tc del root`: kernel has re-attached an mq with handle 0:
	current := []tc.Object{
		{Msg: tc.Msg{Ifindex: eth0, Handle: liveMqMajor, Parent: tcHRoot}, Attribute: tc.Attribute{Kind: "mq"}},
	}

	out := reAnchorAutoManagedParents(saved, current)

	require.Len(t, out.Qdiscs, 3, "no qdiscs added or dropped during re-anchor")
	// mq root: handle unchanged in the saved snapshot (we don't restore it, kernel does)
	assert.Equal(t, savedMqMajor, out.Qdiscs[0].Handle)
	// fq children: Parent.major rewritten from 8026 → 0, minor preserved
	assert.Equal(t, liveMqMajor|1, out.Qdiscs[1].Parent, "first fq child's Parent.major must be re-anchored to the live mq, minor=1 preserved")
	assert.Equal(t, liveMqMajor|2, out.Qdiscs[2].Parent, "second fq child's Parent.major must be re-anchored, minor=2 preserved")
	// tuned fq parameters preserved through the rewrite
	require.NotNil(t, out.Qdiscs[1].Fq)
	require.NotNil(t, out.Qdiscs[1].Fq.BucketsLog)
	assert.Equal(t, uint32(15), *out.Qdiscs[1].Fq.BucketsLog, "Fq params survive the rewrite")
}

// TestReAnchorAutoManagedParents_NoMatchingRootIsNoOp documents that the
// function only rewrites when there's a kernel-auto-managed root in BOTH
// the saved snapshot and the live tree. If the operator pre-tuned the
// interface with htb (not auto-managed) the children's Parent should pass
// through untouched.
func TestReAnchorAutoManagedParents_NoMatchingRootIsNoOp(t *testing.T) {
	const eth0 uint32 = 2
	saved := InterfaceSnapshot{
		Name:    "eth0",
		Ifindex: eth0,
		Qdiscs: []tc.Object{
			{Msg: tc.Msg{Ifindex: eth0, Handle: handle(1, 0), Parent: tcHRoot}, Attribute: tc.Attribute{Kind: "htb"}},
			{Msg: tc.Msg{Ifindex: eth0, Handle: handle(0x30, 0), Parent: handle(1, 0x10)}, Attribute: tc.Attribute{Kind: "sfq"}},
		},
	}
	// live tree has the same htb root (operator-installed, not auto-managed)
	current := []tc.Object{
		{Msg: tc.Msg{Ifindex: eth0, Handle: handle(1, 0), Parent: tcHRoot}, Attribute: tc.Attribute{Kind: "htb"}},
	}

	out := reAnchorAutoManagedParents(saved, current)

	assert.Equal(t, saved.Qdiscs[1].Parent, out.Qdiscs[1].Parent, "non-auto-managed roots leave child Parent untouched")
}

// TestReAnchorAutoManagedParents_OnlyRewritesMatchingKind verifies a
// cross-kind safety: if the saved root was mq but the live root is
// clsact, we must not rewrite (the relationship doesn't carry over).
func TestReAnchorAutoManagedParents_OnlyRewritesMatchingKind(t *testing.T) {
	const eth0 uint32 = 2
	saved := InterfaceSnapshot{
		Name:    "eth0",
		Ifindex: eth0,
		Qdiscs: []tc.Object{
			{Msg: tc.Msg{Ifindex: eth0, Handle: 0x80260000, Parent: tcHRoot}, Attribute: tc.Attribute{Kind: "mq"}},
			{Msg: tc.Msg{Ifindex: eth0, Handle: handle(0x802b, 0), Parent: 0x80260001}, Attribute: tc.Attribute{Kind: "fq"}},
		},
	}
	// live: clsact, not mq — different kind, no rewrite
	current := []tc.Object{
		{Msg: tc.Msg{Ifindex: eth0, Handle: 0xffff0000, Parent: tcHRoot}, Attribute: tc.Attribute{Kind: "clsact"}},
	}

	out := reAnchorAutoManagedParents(saved, current)

	assert.Equal(t, uint32(0x80260001), out.Qdiscs[1].Parent, "different-kind live root does not trigger rewrite")
}

// TestReAnchorAutoManagedParents_FiltersIfindex confirms we only consider
// live qdiscs on the same interface — a multi-interface netns with mq on
// both eth0 and eth1 must not cross-pollinate handles.
func TestReAnchorAutoManagedParents_FiltersIfindex(t *testing.T) {
	const eth0 uint32 = 2
	const eth1 uint32 = 3
	saved := InterfaceSnapshot{
		Name:    "eth0",
		Ifindex: eth0,
		Qdiscs: []tc.Object{
			{Msg: tc.Msg{Ifindex: eth0, Handle: 0x80260000, Parent: tcHRoot}, Attribute: tc.Attribute{Kind: "mq"}},
			{Msg: tc.Msg{Ifindex: eth0, Handle: handle(0x802b, 0), Parent: 0x80260001}, Attribute: tc.Attribute{Kind: "fq"}},
		},
	}
	// live tree has mq on eth1 (different interface) but NOT eth0 — should not
	// be used as a re-anchor source for eth0's children.
	current := []tc.Object{
		{Msg: tc.Msg{Ifindex: eth1, Handle: 0x12340000, Parent: tcHRoot}, Attribute: tc.Attribute{Kind: "mq"}},
	}

	out := reAnchorAutoManagedParents(saved, current)

	assert.Equal(t, uint32(0x80260001), out.Qdiscs[1].Parent, "live mq on a different interface must not re-anchor this interface's children")
}

// TestReAnchorAutoManagedParents_PreservesAllFields confirms re-anchor
// doesn't accidentally clear kind, handle, or kind-specific attribute
// pointers on the rewritten children.
func TestReAnchorAutoManagedParents_PreservesAllFields(t *testing.T) {
	const eth0 uint32 = 2
	tunedFq := &tc.Fq{BucketsLog: uint32Ptr(15), Horizon: uint32Ptr(2_000_000)}
	saved := InterfaceSnapshot{
		Name:    "eth0",
		Ifindex: eth0,
		Qdiscs: []tc.Object{
			{Msg: tc.Msg{Ifindex: eth0, Handle: 0x80260000, Parent: tcHRoot}, Attribute: tc.Attribute{Kind: "mq"}},
			{Msg: tc.Msg{Ifindex: eth0, Handle: handle(0x802b, 0), Parent: 0x80260001}, Attribute: tc.Attribute{Kind: "fq", Fq: tunedFq}},
		},
	}
	current := []tc.Object{
		{Msg: tc.Msg{Ifindex: eth0, Handle: 0x00000000, Parent: tcHRoot}, Attribute: tc.Attribute{Kind: "mq"}},
	}

	out := reAnchorAutoManagedParents(saved, current)

	// Child must keep kind, handle, Fq attribute pointer (same instance)
	child := out.Qdiscs[1]
	assert.Equal(t, "fq", child.Kind)
	assert.Equal(t, handle(0x802b, 0), child.Handle)
	assert.Equal(t, tunedFq, child.Fq, "Fq pointer (with tuned values) survives unchanged")
}
