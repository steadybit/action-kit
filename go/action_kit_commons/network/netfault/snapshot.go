// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package netfault

import (
	"sync"

	"github.com/florianl/go-tc"
)

// snapshotEnabled is the package-level feature flag for the qdisc snapshot/
// restore behaviour. Off by default so the change is opt-in for the first
// release.
var snapshotEnabled bool

// SetSnapshotRestore toggles the qdisc snapshot+restore behaviour. When
// enabled, Apply captures the root qdisc tree for every interface the attack
// touches and Revert replays it after the attack's tc del. This preserves
// cloud-tuned root qdiscs (e.g. GKE's `mq + fq` with buckets=32768 horizon=2s)
// that would otherwise revert to kernel defaults after `tc qdisc del root`.
//
// Disabled by default. Operators can enable it via the extension config (e.g.
// STEADYBIT_EXTENSION_NETWORK_SNAPSHOT_RESTORE=true in extension-container).
//
// Snapshot/restore uses RTNETLINK (github.com/florianl/go-tc) and only takes
// effect on Linux. On non-Linux builds the feature flag is accepted but the
// snapshot is a no-op.
func SetSnapshotRestore(enabled bool) { snapshotEnabled = enabled }

// interfaceSnapshot holds the qdisc and filter state for one interface within
// a single network namespace.
type interfaceSnapshot struct {
	Name    string
	Ifindex uint32
	Qdiscs  []tc.Object
	Filters []tc.Object
}

// qdiscSnapshot holds the snapshot for every interface an attack touches in a
// network namespace.
type qdiscSnapshot struct {
	NetNsID    string
	Interfaces map[string]interfaceSnapshot
}

// snapshotStore keeps snapshots in memory keyed by the runner's netns id. The
// first concurrent attack on a netns takes the snapshot; subsequent attacks
// reuse it. The last attack to be reverted triggers the restore.
var (
	snapshotStoreLock sync.Mutex
	snapshotStore     = map[string]qdiscSnapshot{}
)

func storeSnapshot(snap qdiscSnapshot) {
	snapshotStoreLock.Lock()
	defer snapshotStoreLock.Unlock()
	snapshotStore[snap.NetNsID] = snap
}

func loadSnapshot(netNsID string) (qdiscSnapshot, bool) {
	snapshotStoreLock.Lock()
	defer snapshotStoreLock.Unlock()
	snap, ok := snapshotStore[netNsID]
	return snap, ok
}

func deleteSnapshot(netNsID string) {
	snapshotStoreLock.Lock()
	defer snapshotStoreLock.Unlock()
	delete(snapshotStore, netNsID)
}

// isKernelAutoManaged returns true for qdisc kinds the kernel automatically
// attaches to an interface when no other root qdisc is present. After
// `tc qdisc del root` removes the attack's qdisc, the kernel re-creates one of
// these for the device. We don't restore these ourselves (we'd race the
// kernel); we restore their tuned children instead.
func isKernelAutoManaged(kind string) bool {
	switch kind {
	case "mq", "clsact", "ingress":
		return true
	}
	return false
}

// tcHRoot is the parent handle the kernel uses to indicate a root qdisc.
const tcHRoot uint32 = 0xfffffff1

// handleMajor returns the major portion of a netlink qdisc handle. Netlink
// encodes a handle as (major << 16) | minor. A qdisc's own handle has minor=0
// (e.g. 0x80260000 for major 0x8026). A child's `Parent` field points at one
// of the parent qdisc's classes — same major, non-zero minor (e.g.
// 0x8026000c). To resolve a parent reference back to the owning qdisc we
// compare on major only.
func handleMajor(h uint32) uint32 { return h & 0xffff0000 }

// orderQdiscsForRestore returns the snapshot's qdiscs sorted parent-first via
// topological sort over the parent->child relation. A qdisc is emitted only
// after every other qdisc in the input that owns its parent's major has
// already been emitted. Qdiscs whose Parent is not produced by any other
// snapshotted qdisc (the roots — Parent == TC_H_ROOT, Parent == 0, or Parent
// resolves to a handle outside the snapshot) are emitted first.
//
// This correctly handles N-level trees: Root -> Child -> GrandChild produces
// [Root, Child, GrandChild] regardless of the input order, so Replace() of a
// descendant never runs before its ancestor exists.
func orderQdiscsForRestore(qs []tc.Object) []tc.Object {
	if len(qs) == 0 {
		return nil
	}
	// Map every qdisc's handle-major to its index in qs so we can resolve
	// parent references in O(1). Qdisc handles in the kernel always have
	// minor=0; children reference their parent via (major<<16)|class-minor,
	// so we key by the major portion only.
	handleIndex := make(map[uint32]int, len(qs))
	for i, q := range qs {
		if q.Handle != 0 {
			handleIndex[handleMajor(q.Handle)] = i
		}
	}
	out := make([]tc.Object, 0, len(qs))
	emitted := make([]bool, len(qs))
	// Walk repeatedly over the input until no more progress is possible.
	// Each pass emits every qdisc whose parent is already emitted (or is an
	// ancestor outside the snapshot — a root). Worst-case O(depth * N) which
	// is fine for the small N we deal with.
	for progress := true; progress; {
		progress = false
		for i, q := range qs {
			if emitted[i] {
				continue
			}
			parentIdx, parentInSnapshot := handleIndex[handleMajor(q.Parent)]
			if isRootQdisc(q) || !parentInSnapshot || emitted[parentIdx] {
				out = append(out, q)
				emitted[i] = true
				progress = true
			}
		}
	}
	// Cycle or self-reference: append anything left in input order so the
	// caller at least sees the data and Replace can decide.
	for i, q := range qs {
		if !emitted[i] {
			out = append(out, q)
		}
	}
	return out
}

// isRootQdisc reports whether the qdisc was attached at the device root. Both
// `Parent == TC_H_ROOT` and `Parent == 0` are observed in `tc qdisc show`
// output depending on kernel/iproute2 version; we treat both as root.
func isRootQdisc(q tc.Object) bool {
	return q.Parent == tcHRoot || q.Parent == 0
}

// shouldSkipQdiscOnRestore reports whether restoreSnapshot will skip this
// qdisc rather than calling Replace. We skip kernel-auto-managed kinds (mq,
// clsact, ingress) because the kernel re-attaches them automatically after
// `tc qdisc del root`; restoring them ourselves would race the kernel.
// Pure function — no side effects, no netlink calls — so the decision is
// unit-testable without an actual RTNETLINK socket.
func shouldSkipQdiscOnRestore(q tc.Object) bool {
	return isKernelAutoManaged(q.Kind)
}
