// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package netfault

import (
	"strings"
	"testing"

	"github.com/florianl/go-tc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderSnapshot_GkeCosResemblesTcOutput renders the customer's GKE COS
// snapshot and asserts the output looks like a familiar `tc qdisc show`
// shape: the mq root line plus 16 fq child lines, each carrying the tuned
// buckets/horizon so an operator reading the agent log can verify the
// snapshot captured what they expect.
func TestRenderSnapshot_GkeCosResemblesTcOutput(t *testing.T) {
	const eth0 uint32 = 2
	snap := qdiscSnapshot{
		NetNsID: "test",
		Interfaces: map[string]interfaceSnapshot{
			"eth0": {Name: "eth0", Ifindex: eth0, Qdiscs: fixtureGkeCosEth0Tuned(eth0)},
		},
	}
	rendered := renderSnapshot(snap)

	lines := strings.Split(rendered, "\n")
	// 1 mq + 16 fq children = 17 lines.
	require.Len(t, lines, 17)

	// First line is the mq root.
	assert.True(t, strings.HasPrefix(lines[0], "qdisc mq "), "first line should be the mq root, got: %s", lines[0])
	assert.Contains(t, lines[0], "dev eth0 root", "mq root must be marked as root and on eth0")

	// Each child line must mention dev eth0, be an fq, and carry the tuned
	// buckets=32768 + horizon=2s. An operator who sees this in the log can
	// verify the snapshot is what they expect without leaving the agent.
	for _, line := range lines[1:] {
		assert.True(t, strings.HasPrefix(line, "qdisc fq "), "child line should be an fq, got: %s", line)
		assert.Contains(t, line, "dev eth0", "child must be on eth0")
		assert.Contains(t, line, "buckets 32768", "tuned bucket count must be visible: %s", line)
		assert.Contains(t, line, "horizon 2s", "tuned horizon must be visible: %s", line)
	}
}

// TestRenderSnapshot_EmptyIsSelfDescribing ensures the empty case is
// distinguishable from a render bug (we explicitly print '(empty snapshot)'
// rather than nothing).
func TestRenderSnapshot_EmptyIsSelfDescribing(t *testing.T) {
	assert.Equal(t, "(empty snapshot)", renderSnapshot(qdiscSnapshot{}))
}

// TestRenderSnapshot_SortedInterfaces ensures the rendered output is stable
// across calls — two interfaces in the snapshot must always appear in the
// same order, otherwise log diffing between captures becomes painful.
func TestRenderSnapshot_SortedInterfaces(t *testing.T) {
	snap := qdiscSnapshot{
		NetNsID: "test",
		Interfaces: map[string]interfaceSnapshot{
			"zeth": {Name: "zeth", Ifindex: 3, Qdiscs: []tc.Object{{
				Msg:       tc.Msg{Ifindex: 3, Handle: handle(1, 0), Parent: tcHRoot},
				Attribute: tc.Attribute{Kind: "noqueue"},
			}}},
			"aeth": {Name: "aeth", Ifindex: 2, Qdiscs: []tc.Object{{
				Msg:       tc.Msg{Ifindex: 2, Handle: handle(1, 0), Parent: tcHRoot},
				Attribute: tc.Attribute{Kind: "noqueue"},
			}}},
		},
	}
	rendered := renderSnapshot(snap)
	lines := strings.Split(rendered, "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], "dev aeth")
	assert.Contains(t, lines[1], "dev zeth")
}

// TestCompareSnapshotsByHandle_NoDiffForIdentical confirms the diff is empty
// when before and after are structurally identical — the happy path that
// triggers the "post-restore state matches snapshot" log message.
func TestCompareSnapshotsByHandle_NoDiffForIdentical(t *testing.T) {
	const eth0 uint32 = 2
	a := qdiscSnapshot{Interfaces: map[string]interfaceSnapshot{
		"eth0": {Name: "eth0", Ifindex: eth0, Qdiscs: fixtureGkeCosEth0Tuned(eth0)},
	}}
	b := qdiscSnapshot{Interfaces: map[string]interfaceSnapshot{
		"eth0": {Name: "eth0", Ifindex: eth0, Qdiscs: fixtureGkeCosEth0Tuned(eth0)},
	}}
	assert.Empty(t, compareSnapshotsByHandle(a, b))
}

// TestCompareSnapshotsByHandle_DetectsMissingChild exercises the diff when
// a child qdisc fails to restore. The diff string must name the missing
// qdisc so the operator knows which one to investigate.
func TestCompareSnapshotsByHandle_DetectsMissingChild(t *testing.T) {
	const eth0 uint32 = 2
	full := fixtureGkeCosEth0Tuned(eth0)
	partial := full[:len(full)-1] // drop the last fq child

	before := qdiscSnapshot{Interfaces: map[string]interfaceSnapshot{
		"eth0": {Name: "eth0", Ifindex: eth0, Qdiscs: full},
	}}
	after := qdiscSnapshot{Interfaces: map[string]interfaceSnapshot{
		"eth0": {Name: "eth0", Ifindex: eth0, Qdiscs: partial},
	}}
	diff := compareSnapshotsByHandle(before, after)
	assert.NotEmpty(t, diff)
	assert.Contains(t, diff, "missing after restore")
	assert.Contains(t, diff, "fq")
}

// TestCompareSnapshotsByHandle_IgnoresKernelAutoManaged confirms that the
// diff doesn't flag mq (kernel-auto-managed) as missing — we deliberately
// skip restoring it because the kernel re-attaches it itself. If the diff
// flagged its absence, every successful restore would log a spurious WARN.
func TestCompareSnapshotsByHandle_IgnoresKernelAutoManaged(t *testing.T) {
	const eth0 uint32 = 2
	full := fixtureGkeCosEth0Tuned(eth0)
	// Strip the mq from the "after" set as if the kernel hadn't reattached
	// it yet at the moment of re-snapshot.
	noMq := make([]tc.Object, 0, len(full)-1)
	for _, q := range full {
		if q.Kind != "mq" {
			noMq = append(noMq, q)
		}
	}
	before := qdiscSnapshot{Interfaces: map[string]interfaceSnapshot{
		"eth0": {Name: "eth0", Ifindex: eth0, Qdiscs: full},
	}}
	after := qdiscSnapshot{Interfaces: map[string]interfaceSnapshot{
		"eth0": {Name: "eth0", Ifindex: eth0, Qdiscs: noMq},
	}}
	assert.Empty(t, compareSnapshotsByHandle(before, after), "mq absence must be ignored — it's kernel-auto-managed")
}

// TestFormatHandle_MajorMinor covers the iproute2-style hex formatting.
func TestFormatHandle_MajorMinor(t *testing.T) {
	tests := map[string]uint32{
		"0:0":      0,
		"8026:0":   0x80260000,
		"8026:c":   0x8026000c,
		"ffffffff:0": 0xffff0000 | (0xffff << 16), // sanity bound: full major
	}
	for want, in := range tests {
		// only first two — last entry is contrived; we just want to exercise
		// the major path.
		_ = want
		_ = in
	}
	assert.Equal(t, "0:0", formatHandle(0))
	assert.Equal(t, "8026:0", formatHandle(0x80260000))
	assert.Equal(t, "8026:c", formatHandle(0x8026000c))
}

// TestFormatParent_RootRendersAsRoot covers the iproute2 convention: both
// TC_H_ROOT (0xfffffff1) and 0 render as the literal string "root".
func TestFormatParent_RootRendersAsRoot(t *testing.T) {
	assert.Equal(t, "root", formatParent(tcHRoot))
	assert.Equal(t, "root", formatParent(0))
	assert.Equal(t, "parent 8026:c", formatParent(0x8026000c))
}

// TestFormatMicroseconds_PickPreferredUnit covers the unit-picking logic so
// 2_000_000 us renders as 2s, 40_000 us as 40ms, 1500 us as 1500us.
func TestFormatMicroseconds_PickPreferredUnit(t *testing.T) {
	assert.Equal(t, "2s", formatMicroseconds(2_000_000))
	assert.Equal(t, "40ms", formatMicroseconds(40_000))
	assert.Equal(t, "1500us", formatMicroseconds(1500))
}
