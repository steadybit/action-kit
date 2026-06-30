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

func TestIsKernelAutoManaged(t *testing.T) {
	tests := []struct {
		kind string
		want bool
	}{
		{"mq", true},
		{"clsact", true},
		{"ingress", true},
		{"noqueue", true},
		{"pfifo_fast", true},
		{"fq", false},
		{"fq_codel", false},
		{"htb", false},
		{"netem", false},
		{"prio", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			assert.Equal(t, tt.want, isKernelAutoManaged(tt.kind))
		})
	}
}

func TestOrderQdiscsForRestore_RootsBeforeChildren(t *testing.T) {
	// Use the package-level tcHRoot (= 0xffffffff per Linux uapi
	// pkt_sched.h) rather than redefining it locally — an earlier version
	// of this test redefined it as 0xfffffff1 (which is TC_H_INGRESS, not
	// TC_H_ROOT) and silently masked a bug where isRootQdisc never
	// recognised real roots returned by go-tc Get().

	root := tc.Object{
		Msg:       tc.Msg{Ifindex: 2, Handle: 0x80260000, Parent: tcHRoot},
		Attribute: tc.Attribute{Kind: "mq"},
	}
	child1 := tc.Object{
		Msg:       tc.Msg{Ifindex: 2, Handle: 0x802b0000, Parent: 0x8026000c},
		Attribute: tc.Attribute{Kind: "fq"},
	}
	child2 := tc.Object{
		Msg:       tc.Msg{Ifindex: 2, Handle: 0x80290000, Parent: 0x8026000e},
		Attribute: tc.Attribute{Kind: "fq"},
	}
	rootParentZero := tc.Object{
		Msg:       tc.Msg{Ifindex: 3, Handle: 0x10000, Parent: 0}, // some flavours report Parent=0 for roots
		Attribute: tc.Attribute{Kind: "noqueue"},
	}

	in := []tc.Object{child1, root, child2, rootParentZero}
	out := orderQdiscsForRestore(in)

	require.Len(t, out, 4, "no entries dropped")

	// Strong assertion: every child must come AFTER its parent in the
	// output. The previous form ("scan for first non-root, assert
	// everything before it was a root") accepted a no-op implementation
	// that returned input unchanged — if child1 was at index 0,
	// firstNonRoot=0 and the loop body never ran. Here we look up each
	// non-root's parent in the output and assert it appears earlier.
	posByHandleMajor := map[uint32]int{}
	for i, q := range out {
		if q.Handle != 0 {
			posByHandleMajor[q.Handle&0xffff0000] = i
		}
	}
	for i, q := range out {
		if isRootQdisc(q) {
			continue
		}
		parentPos, ok := posByHandleMajor[q.Parent&0xffff0000]
		require.True(t, ok, "child at %d references parent major not present in output", i)
		assert.Less(t, parentPos, i, "child %s at %d must come after its parent (parent at %d)", q.Kind, i, parentPos)
	}
}

func TestQdiscSnapshot_IsEmpty(t *testing.T) {
	assert.True(t, QdiscSnapshot{}.IsEmpty(), "zero value is empty")
	assert.True(t, QdiscSnapshot{NetNsID: "x"}.IsEmpty(), "no interfaces is empty (regardless of NetNsID)")
	assert.True(t, QdiscSnapshot{Interfaces: map[string]InterfaceSnapshot{}}.IsEmpty(), "empty map is empty")
	assert.False(t,
		QdiscSnapshot{Interfaces: map[string]InterfaceSnapshot{"eth0": {Name: "eth0"}}}.IsEmpty(),
		"at least one interface makes it non-empty",
	)
}

