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
// re-attaches when no other qdisc is present, or for stateless kinds that
// carry no tunable parameters worth restoring. After `tc qdisc del root`
// removes the attack's qdisc, the kernel re-creates one of these for the
// device. We don't restore these ourselves (we'd race the kernel and, for
// some kinds like `pfifo_fast`, go-tc rejects them with ErrNotImplemented);
// we restore their tuned children instead.
//
// Specifically:
//   - mq, clsact, ingress: kernel auto-attaches as multi-queue / hook qdiscs.
//   - noqueue: default on loopback and veth interfaces; no parameters.
//   - pfifo_fast: kernel default leaf for non-multi-queue NICs; only carries
//     a priomap that the kernel always restores from /sys defaults.
func isKernelAutoManaged(kind string) bool {
	switch kind {
	case "mq", "clsact", "ingress", "noqueue", "pfifo_fast":
		return true
	}
	return false
}

// tcHRoot is the parent handle the kernel uses to indicate a root qdisc.
// From the kernel's uapi/linux/pkt_sched.h:
//
//	#define TC_H_ROOT     (0xFFFFFFFFU)
//	#define TC_H_INGRESS  (0xFFFFFFF1U)
//
// Don't confuse the two — using 0xfffffff1 here would mean every root qdisc
// reported by go-tc would fail isRootQdisc and silently skip both the
// re-anchor and the claim steps in restoreSnapshot.
const tcHRoot uint32 = 0xffffffff

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

// stripRuntimeStats zeros the kernel-counter fields on a tc.Object so it can
// be safely passed to Qdisc().Replace() / Filter().Replace(). go-tc's Get
// populates Stats/XStats/Stats2 from the kernel, but its validateQdiscObject
// (qdisc.go:174) rejects any non-DELETE request whose object carries them
// with bare ErrNotImplemented — stats marshalling isn't implemented for the
// write direction. Mutates the object in place.
func stripRuntimeStats(obj *tc.Object) {
	obj.Stats = nil
	obj.XStats = nil
	obj.Stats2 = nil
}

// reAnchorAutoManagedParents rewrites the Parent.major of every child qdisc
// in ifSnap that referenced a saved kernel-auto-managed root, replacing it
// with the major of whatever auto-managed root the kernel has re-attached
// (post `tc qdisc del root`). Without this re-anchoring, restoring a child
// whose saved Parent points at the OLD mq handle fails with ENOENT because
// the kernel-attached mq has a different (usually 0:) handle.
//
// The rewrite is conservative: it only touches children whose Parent.major
// matches the major of a saved root that (a) is itself kernel-auto-managed
// and (b) has been replaced by a kernel-auto-managed root of the same kind
// in the live tree. Everything else passes through unchanged.
func reAnchorAutoManagedParents(ifSnap interfaceSnapshot, currentQdiscs []tc.Object) interfaceSnapshot {
	rewrite := map[uint32]uint32{}
	for _, saved := range ifSnap.Qdiscs {
		if !isRootQdisc(saved) || !isKernelAutoManaged(saved.Kind) {
			continue
		}
		savedMajor := handleMajor(saved.Handle)
		for _, cur := range currentQdiscs {
			if cur.Ifindex != ifSnap.Ifindex {
				continue
			}
			if !isRootQdisc(cur) || cur.Kind != saved.Kind {
				continue
			}
			liveMajor := handleMajor(cur.Handle)
			if liveMajor != savedMajor {
				rewrite[savedMajor] = liveMajor
			}
			break
		}
	}
	if len(rewrite) == 0 {
		return ifSnap
	}
	out := interfaceSnapshot{Name: ifSnap.Name, Ifindex: ifSnap.Ifindex}
	out.Qdiscs = make([]tc.Object, 0, len(ifSnap.Qdiscs))
	for _, q := range ifSnap.Qdiscs {
		if !isRootQdisc(q) {
			if newMajor, ok := rewrite[handleMajor(q.Parent)]; ok {
				q.Parent = newMajor | (q.Parent & 0x0000ffff)
			}
		}
		out.Qdiscs = append(out.Qdiscs, q)
	}
	out.Filters = ifSnap.Filters
	return out
}
