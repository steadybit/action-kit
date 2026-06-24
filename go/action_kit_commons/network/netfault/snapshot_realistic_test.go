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

// TestRestorePlan_GkeCosCustomerCase reproduces the exact qdisc shape from
// the customer ticket: an mq root with 16 fq children tuned to
// buckets=32768, horizon=2s. The planner must:
//   - skip restoring the mq root (kernel auto-attaches it after `tc del root`)
//   - replace all 16 fq children (to overwrite the kernel's default-tuned
//     children with the GKE-tuned ones)
//
// Without this PR's logic, the `tc del root` at revert leaves the host with
// mq + 16 default fq children (buckets=1024, horizon=10s) which is the
// degradation the customer reported.
func TestRestorePlan_GkeCosCustomerCase(t *testing.T) {
	const eth0 uint32 = 2
	qdiscs := fixtureGkeCosEth0Tuned(eth0)

	require.Len(t, qdiscs, 17, "fixture should have 1 mq root + 16 fq children")

	// Sort, then walk and classify each entry.
	ordered := orderQdiscsForRestore(qdiscs)
	require.Equal(t, "mq", ordered[0].Kind, "mq root must come first")
	require.True(t, isRootQdisc(ordered[0]), "first entry must be a root")

	var skipped, replaced int
	var replacedKinds = map[string]int{}
	for _, q := range ordered {
		if shouldSkipQdiscOnRestore(q) {
			skipped++
		} else {
			replaced++
			replacedKinds[q.Kind]++
		}
	}

	assert.Equal(t, 1, skipped, "exactly the mq root should be skipped on restore")
	assert.Equal(t, 16, replaced, "all 16 fq children should be replaced")
	assert.Equal(t, 16, replacedKinds["fq"], "every replaced qdisc should be fq")

	// Spot-check one tuned child: it must carry the GKE buckets/horizon so
	// Replace() faithfully re-applies the cloud's tuning.
	tunedChild := qdiscs[1]
	require.NotNil(t, tunedChild.Fq, "fq attribute must be populated")
	require.NotNil(t, tunedChild.Fq.BucketsLog)
	require.NotNil(t, tunedChild.Fq.Horizon)
	assert.Equal(t, uint32(15), *tunedChild.Fq.BucketsLog, "BucketsLog=15 means 32768 buckets")
	assert.Equal(t, uint32(2_000_000), *tunedChild.Fq.Horizon, "horizon=2s in microseconds")
}

// TestRestorePlan_GkeCosMultiInterface covers the full snapshot for a GKE
// COS node: eth0 with tuned mq+fq, lo with noqueue. After revert the
// snapshot store should round-trip both and the planner should issue the
// right action for each interface independently.
func TestRestorePlan_GkeCosMultiInterface(t *testing.T) {
	const eth0 uint32 = 2
	const lo uint32 = 1
	t.Cleanup(func() { deleteSnapshot("gke-multi") })

	snap := qdiscSnapshot{
		NetNsID: "gke-multi",
		Interfaces: map[string]interfaceSnapshot{
			"eth0": {Name: "eth0", Ifindex: eth0, Qdiscs: fixtureGkeCosEth0Tuned(eth0)},
			"lo":   {Name: "lo", Ifindex: lo, Qdiscs: fixtureGkeCosLoopback(lo)},
		},
	}
	storeSnapshot(snap)

	got, ok := loadSnapshot("gke-multi")
	require.True(t, ok)
	require.Len(t, got.Interfaces, 2)

	// eth0: 1 skip (mq) + 16 replaces (fq children)
	plan := planForInterface(got.Interfaces["eth0"].Qdiscs)
	assert.Equal(t, planCounts{skip: 1, replace: 16}, plan, "eth0: skip mq, replace 16 fq children")

	// lo: noqueue is NOT in isKernelAutoManaged so it would be replaced.
	// But noqueue is a stateless kind — Replace is harmless.
	plan = planForInterface(got.Interfaces["lo"].Qdiscs)
	assert.Equal(t, planCounts{skip: 0, replace: 1}, plan, "lo: noqueue gets replaced (harmless no-op effectively)")
}

// TestRestorePlan_AksDefault covers a typical AKS / Ubuntu node where
// fq_codel is the kernel's default root qdisc. fq_codel is NOT in
// isKernelAutoManaged (the kernel does re-attach an fq_codel after `tc del`,
// but only under the right sysctl; safer to actively restore it). The
// planner replaces it.
func TestRestorePlan_AksDefault(t *testing.T) {
	const eth0 uint32 = 2
	plan := planForInterface(fixtureAksDefaultEth0(eth0))
	assert.Equal(t, planCounts{skip: 0, replace: 1}, plan)
}

// TestRestorePlan_EksDefault covers EKS Amazon Linux 2 with pfifo_fast.
// pfifo_fast is a kernel default kind but doesn't appear in
// isKernelAutoManaged today (we conservatively replace it). Test documents
// that decision.
func TestRestorePlan_EksDefault(t *testing.T) {
	const eth0 uint32 = 2
	plan := planForInterface(fixtureEksDefaultEth0(eth0))
	assert.Equal(t, planCounts{skip: 0, replace: 1}, plan)
}

// TestRestorePlan_BareMetalHtb covers a host with a user-installed htb
// shaper tree. In production this case is caught by preflight (htb is not
// in safeRootQdiscKinds), so the snapshot path shouldn't run. The test
// asserts the planner produces consistent output even if it did run.
func TestRestorePlan_BareMetalHtb(t *testing.T) {
	const eth0 uint32 = 2
	qdiscs := fixtureBareMetalHtbWithClasses(eth0)
	ordered := orderQdiscsForRestore(qdiscs)

	require.Equal(t, "htb", ordered[0].Kind, "htb root must be first in restore order")
	require.True(t, isRootQdisc(ordered[0]))
	require.Equal(t, "sfq", ordered[1].Kind, "sfq child must come after htb root")
	require.False(t, isRootQdisc(ordered[1]))

	plan := planForInterface(qdiscs)
	assert.Equal(t, planCounts{skip: 0, replace: 2}, plan, "neither htb nor sfq is kernel-auto-managed; both restored")
}

// TestRestorePlan_PriorAttackResidue covers the recovery edge case where a
// previous attack crashed mid-revert. The next snapshot might include the
// previous attack's prio root + netem child. Restore must walk parent-first
// so prio is recreated before netem tries to attach under it.
func TestRestorePlan_PriorAttackResidue(t *testing.T) {
	const eth0 uint32 = 2
	qdiscs := fixturePriorAttackResidue(eth0)
	ordered := orderQdiscsForRestore(qdiscs)

	require.Equal(t, "prio", ordered[0].Kind, "prio root must be created before its netem child")
	require.Equal(t, "netem", ordered[1].Kind)
	require.True(t, isRootQdisc(ordered[0]))
	require.False(t, isRootQdisc(ordered[1]), "netem child has prio as its parent")

	plan := planForInterface(qdiscs)
	assert.Equal(t, planCounts{skip: 0, replace: 2}, plan)
}

// TestRestorePlan_ClsactSkipped verifies clsact (modern BPF-friendly
// ingress+egress qdisc) is treated as kernel-auto-managed. Restoring it
// ourselves would race the kernel and potentially break BPF programs that
// re-attach on the kernel's clsact, so we skip.
func TestRestorePlan_ClsactSkipped(t *testing.T) {
	const eth0 uint32 = 2
	plan := planForInterface(fixtureClsactWithIngress(eth0))
	assert.Equal(t, planCounts{skip: 1, replace: 0}, plan, "clsact is kernel-auto-managed; skip restore")
}

// TestRestorePlan_OrderingParentFirst is a synthetic stress test: a deeper
// tree (prio root, classes attached, netem children below classes) to
// confirm orderQdiscsForRestore is purely parent-first and doesn't accidentally
// reorder among children. Important for ensuring deeper attack topologies
// (not currently used by netfault but possible in future) restore cleanly.
func TestRestorePlan_OrderingParentFirst(t *testing.T) {
	const eth0 uint32 = 2
	qdiscs := []tc.Object{
		// Intentionally not in dependency order so the function has to sort.
		{Msg: tc.Msg{Ifindex: eth0, Handle: handle(40, 0), Parent: handle(1, 3)}, Attribute: tc.Attribute{Kind: "netem"}},
		{Msg: tc.Msg{Ifindex: eth0, Handle: handle(30, 0), Parent: handle(1, 2)}, Attribute: tc.Attribute{Kind: "sfq"}},
		{Msg: tc.Msg{Ifindex: eth0, Handle: handle(1, 0), Parent: tcHRoot}, Attribute: tc.Attribute{Kind: "prio"}},
	}
	ordered := orderQdiscsForRestore(qdiscs)

	require.Len(t, ordered, 3)
	assert.True(t, isRootQdisc(ordered[0]), "first entry must be the root")
	assert.Equal(t, "prio", ordered[0].Kind, "prio root must come first")
	for i := 1; i < len(ordered); i++ {
		assert.False(t, isRootQdisc(ordered[i]), "all entries after the first must be children of the root")
	}
}

// TestRestorePlan_OrderingThreeLevelNesting covers the latent bug the
// previous two-pass partition could not handle: a 3-level qdisc tree where
// a grandchild's parent is itself a non-root child. The earlier algorithm
// would emit the grandchild before its parent (both are non-roots, so they
// were partitioned together and preserved input order). The topological
// sort emits each child only after its parent's handle has been emitted.
func TestRestorePlan_OrderingThreeLevelNesting(t *testing.T) {
	const eth0 uint32 = 2
	// Root htb (handle 1:0) -> child htb (handle 10:0, parent 1:1) ->
	// grandchild sfq (handle 100:0, parent 10:1). Input is intentionally
	// scrambled so any algorithm that just preserves input order would
	// emit grandchild before child.
	qdiscs := []tc.Object{
		{Msg: tc.Msg{Ifindex: eth0, Handle: handle(100, 0), Parent: handle(10, 1)}, Attribute: tc.Attribute{Kind: "sfq"}},
		{Msg: tc.Msg{Ifindex: eth0, Handle: handle(1, 0), Parent: tcHRoot}, Attribute: tc.Attribute{Kind: "htb"}},
		{Msg: tc.Msg{Ifindex: eth0, Handle: handle(10, 0), Parent: handle(1, 1)}, Attribute: tc.Attribute{Kind: "htb"}},
	}
	ordered := orderQdiscsForRestore(qdiscs)

	require.Len(t, ordered, 3, "no entries dropped")
	// Index each qdisc by major in the output, then assert each child
	// appears after its parent's major.
	pos := map[uint32]int{}
	for i, q := range ordered {
		pos[q.Handle&0xffff0000] = i
	}
	for i, q := range ordered {
		if isRootQdisc(q) {
			continue
		}
		parentPos, ok := pos[q.Parent&0xffff0000]
		require.True(t, ok)
		assert.Less(t, parentPos, i, "child %s at index %d must come after its parent at %d", q.Kind, i, parentPos)
	}
	assert.Equal(t, "htb", ordered[0].Kind, "root htb must come first")
}

// TestRestorePlan_EmptySnapshot covers veth/CNI interfaces with no
// pre-existing root qdisc (a fresh netns). The snapshot captures nothing
// and restore is a no-op. Important: storeSnapshot should NOT later think
// there's nothing to restore and crash; it should just iterate over an
// empty map.
func TestRestorePlan_EmptySnapshot(t *testing.T) {
	t.Cleanup(func() { deleteSnapshot("empty-ns") })

	storeSnapshot(qdiscSnapshot{NetNsID: "empty-ns", Interfaces: map[string]interfaceSnapshot{}})

	got, ok := loadSnapshot("empty-ns")
	require.True(t, ok)
	assert.Empty(t, got.Interfaces, "empty snapshot has no interfaces to restore")
}

// TestRestorePlan_MqWithoutChildren covers the (rare) case where an mq root
// exists but the snapshot didn't capture any tuned children (e.g., the NIC
// is single-queue and the kernel's auto-default already matches the user's
// expectation). Skip the mq, replace nothing — entirely a no-op.
func TestRestorePlan_MqWithoutChildren(t *testing.T) {
	const eth0 uint32 = 2
	qdiscs := []tc.Object{{
		Msg:       tc.Msg{Ifindex: eth0, Handle: handle(0x8026, 0), Parent: tcHRoot},
		Attribute: tc.Attribute{Kind: "mq"},
	}}
	plan := planForInterface(qdiscs)
	assert.Equal(t, planCounts{skip: 1, replace: 0}, plan)
}

// TestRestorePlan_StoreLifecycleAcrossMultipleAttacks simulates the
// snapshot-store guard: the first attack on a netns takes a snapshot; a
// second concurrent attack on the same netns reuses the existing snapshot
// (loadSnapshot returns ok=true) without overwriting it. Lifecycle ends
// when the last attack reverts and we call deleteSnapshot.
func TestRestorePlan_StoreLifecycleAcrossMultipleAttacks(t *testing.T) {
	const netNs = "multi-attack"
	t.Cleanup(func() { deleteSnapshot(netNs) })

	// First attack takes snapshot.
	_, exists := loadSnapshot(netNs)
	assert.False(t, exists)
	storeSnapshot(qdiscSnapshot{
		NetNsID:    netNs,
		Interfaces: map[string]interfaceSnapshot{"eth0": {Name: "eth0", Qdiscs: fixtureGkeCosEth0Tuned(2)}},
	})
	first, _ := loadSnapshot(netNs)
	firstQdiscCount := len(first.Interfaces["eth0"].Qdiscs)

	// Second concurrent attack arrives — loadSnapshot says one already
	// exists, so the caller should skip taking another.
	_, exists = loadSnapshot(netNs)
	assert.True(t, exists, "second attack should see existing snapshot and skip taking another")

	// Last attack reverts: load + restore + delete.
	loaded, ok := loadSnapshot(netNs)
	require.True(t, ok)
	assert.Equal(t, firstQdiscCount, len(loaded.Interfaces["eth0"].Qdiscs), "snapshot must be unchanged from first attack's capture")
	deleteSnapshot(netNs)
	_, exists = loadSnapshot(netNs)
	assert.False(t, exists)
}

// planCounts aggregates planner decisions for an interface so tests can
// assert plan shape in one comparison.
type planCounts struct {
	skip    int
	replace int
}

func planForInterface(qs []tc.Object) planCounts {
	var p planCounts
	for _, q := range orderQdiscsForRestore(qs) {
		if shouldSkipQdiscOnRestore(q) {
			p.skip++
		} else {
			p.replace++
		}
	}
	return p
}
