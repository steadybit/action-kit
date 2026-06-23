// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package netfault

import (
	"testing"

	"github.com/florianl/go-tc"
	"github.com/stretchr/testify/assert"
)

func TestIsKernelAutoManaged(t *testing.T) {
	tests := []struct {
		kind string
		want bool
	}{
		{"mq", true},
		{"clsact", true},
		{"ingress", true},
		{"fq", false},
		{"fq_codel", false},
		{"htb", false},
		{"netem", false},
		{"prio", false},
		{"pfifo_fast", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			assert.Equal(t, tt.want, isKernelAutoManaged(tt.kind))
		})
	}
}

func TestOrderQdiscsForRestore_RootsBeforeChildren(t *testing.T) {
	const tcHRoot uint32 = 0xfffffff1

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

	assert.Len(t, out, 4, "no entries dropped")

	// All roots (Parent == TC_H_ROOT or 0) must appear before any non-root.
	firstNonRoot := -1
	for i, q := range out {
		if q.Parent != tcHRoot && q.Parent != 0 {
			firstNonRoot = i
			break
		}
	}
	assert.GreaterOrEqual(t, firstNonRoot, 0, "expected at least one non-root in output")
	for i := 0; i < firstNonRoot; i++ {
		isRoot := out[i].Parent == tcHRoot || out[i].Parent == 0
		assert.True(t, isRoot, "qdisc at position %d before first non-root should be a root", i)
	}
}

func TestSnapshotStore_Roundtrip(t *testing.T) {
	t.Cleanup(func() { deleteSnapshot("test-roundtrip") })

	assert.False(t, hasSnapshot("test-roundtrip"))

	snap := qdiscSnapshot{
		NetNsID: "test-roundtrip",
		Interfaces: map[string]interfaceSnapshot{
			"eth0": {Name: "eth0", Ifindex: 2},
		},
	}
	storeSnapshot(snap)

	assert.True(t, hasSnapshot("test-roundtrip"))

	got, ok := loadSnapshot("test-roundtrip")
	assert.True(t, ok)
	assert.Equal(t, "test-roundtrip", got.NetNsID)
	assert.Contains(t, got.Interfaces, "eth0")
	assert.Equal(t, uint32(2), got.Interfaces["eth0"].Ifindex)

	deleteSnapshot("test-roundtrip")
	assert.False(t, hasSnapshot("test-roundtrip"))
	_, ok = loadSnapshot("test-roundtrip")
	assert.False(t, ok)
}

func TestSnapshotStore_OverwriteSameNetNs(t *testing.T) {
	t.Cleanup(func() { deleteSnapshot("test-overwrite") })

	storeSnapshot(qdiscSnapshot{NetNsID: "test-overwrite", Interfaces: map[string]interfaceSnapshot{"eth0": {Name: "eth0"}}})
	storeSnapshot(qdiscSnapshot{NetNsID: "test-overwrite", Interfaces: map[string]interfaceSnapshot{"eth1": {Name: "eth1"}}})

	got, ok := loadSnapshot("test-overwrite")
	assert.True(t, ok)
	assert.NotContains(t, got.Interfaces, "eth0", "second store should replace, not merge")
	assert.Contains(t, got.Interfaces, "eth1")
}

func TestSetSnapshotRestore_TogglesFlag(t *testing.T) {
	t.Cleanup(func() { SetSnapshotRestore(false) })

	SetSnapshotRestore(true)
	assert.True(t, snapshotEnabled)
	SetSnapshotRestore(false)
	assert.False(t, snapshotEnabled)
}
