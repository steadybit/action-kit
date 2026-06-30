// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package netfault

import (
	"github.com/florianl/go-tc"
)

// Snapshot/restore is no longer a separate toggle: it runs whenever strict
// mode is OFF, i.e. when the operator has chosen to let network attacks
// install on non-`noqueue` roots. With strict on, the preflight refuses the
// attack before snapshot would matter. With strict off, snapshot is what
// keeps the cloud-tuned root from getting reset to kernel defaults on
// revert. There's no third state worth supporting (strict=off +
// snapshot=off was the clobber-no-restore behaviour we retired), so we
// drive both off the single `strictRootQdisc` knob.
//
// Snapshot/restore uses RTNETLINK (github.com/florianl/go-tc) and only
// takes effect on Linux. On non-Linux builds the path is a no-op.
//
// Lifecycle: Apply returns the captured snapshot, the caller stores it in
// the action's per-execution state (via action_kit_sdk JSON state), and
// passes it back to Revert. The library holds no cross-call state.

// InterfaceSnapshot holds the qdisc and filter state for one interface within
// a single network namespace.
type InterfaceSnapshot struct {
	Name    string
	Ifindex uint32
	Qdiscs  []tc.Object
	Filters []tc.Object
}

// QdiscSnapshot holds the snapshot for every interface an attack touches in a
// network namespace. The zero value (empty NetNsID, nil Interfaces) is a
// valid "nothing to restore" sentinel — Revert treats it as a no-op.
//
// All fields are JSON-serializable so callers can persist the snapshot in
// per-execution state (e.g. action_kit_sdk's state). The embedded tc.Object
// values from github.com/florianl/go-tc roundtrip cleanly through
// encoding/json (verified by TestQdiscSnapshotJSONRoundtrip).
type QdiscSnapshot struct {
	NetNsID    string
	Interfaces map[string]InterfaceSnapshot
}

// IsEmpty reports whether the snapshot carries no per-interface state and
// Revert should treat it as a no-op (e.g. strict mode was on at Apply time,
// or the attack didn't touch a tc root qdisc).
func (s QdiscSnapshot) IsEmpty() bool {
	return len(s.Interfaces) == 0
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
	handleIndex := indexQdiscsByHandleMajor(qs)
	out := make([]tc.Object, 0, len(qs))
	emitted := make([]bool, len(qs))
	for fixedPoint := false; !fixedPoint; {
		fixedPoint = !emitQdiscsWithKnownParents(qs, handleIndex, emitted, &out)
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

// indexQdiscsByHandleMajor maps every qdisc's handle-major to its index in
// qs so a child's Parent reference can resolve to its parent in O(1).
// Qdisc handles in the kernel always have minor=0; children reference their
// parent via (major<<16)|class-minor.
func indexQdiscsByHandleMajor(qs []tc.Object) map[uint32]int {
	idx := make(map[uint32]int, len(qs))
	for i, q := range qs {
		if q.Handle != 0 {
			idx[handleMajor(q.Handle)] = i
		}
	}
	return idx
}

// emitQdiscsWithKnownParents appends every not-yet-emitted qdisc whose
// parent is a root, lives outside the snapshot, or has already been emitted.
// Returns true if it emitted at least one qdisc this pass — the caller loops
// until a pass emits nothing (fixed point).
func emitQdiscsWithKnownParents(qs []tc.Object, handleIndex map[uint32]int, emitted []bool, out *[]tc.Object) bool {
	progress := false
	for i, q := range qs {
		if emitted[i] {
			continue
		}
		parentIdx, parentInSnapshot := handleIndex[handleMajor(q.Parent)]
		if isRootQdisc(q) || !parentInSnapshot || emitted[parentIdx] {
			*out = append(*out, q)
			emitted[i] = true
			progress = true
		}
	}
	return progress
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
func reAnchorAutoManagedParents(ifSnap InterfaceSnapshot, currentQdiscs []tc.Object) InterfaceSnapshot {
	rewrite := buildAutoManagedRewriteMap(ifSnap, currentQdiscs)
	if len(rewrite) == 0 {
		return ifSnap
	}
	return applyParentRewrites(ifSnap, rewrite)
}

// buildAutoManagedRewriteMap finds saved kernel-auto-managed roots whose
// live counterpart on the same interface has a different major handle and
// returns a saved-major → live-major map. Other saved roots and
// non-auto-managed kinds contribute nothing.
func buildAutoManagedRewriteMap(ifSnap InterfaceSnapshot, currentQdiscs []tc.Object) map[uint32]uint32 {
	rewrite := map[uint32]uint32{}
	for _, saved := range ifSnap.Qdiscs {
		if !isRootQdisc(saved) || !isKernelAutoManaged(saved.Kind) {
			continue
		}
		savedMajor := handleMajor(saved.Handle)
		if liveMajor, ok := findLiveRootMajor(currentQdiscs, ifSnap.Ifindex, saved.Kind); ok && liveMajor != savedMajor {
			rewrite[savedMajor] = liveMajor
		}
	}
	return rewrite
}

// findLiveRootMajor returns the major handle of the first root qdisc on the
// given interface in the current tree that matches the kind. ok=false if
// none is found.
func findLiveRootMajor(currentQdiscs []tc.Object, ifindex uint32, kind string) (uint32, bool) {
	for _, cur := range currentQdiscs {
		if cur.Ifindex == ifindex && isRootQdisc(cur) && cur.Kind == kind {
			return handleMajor(cur.Handle), true
		}
	}
	return 0, false
}

// applyParentRewrites returns a new InterfaceSnapshot whose non-root child
// qdiscs have their Parent.major replaced via the rewrite map (minor is
// preserved). The roots themselves are left untouched — the kernel
// re-attaches them under their own (potentially still-different) live
// handles.
func applyParentRewrites(ifSnap InterfaceSnapshot, rewrite map[uint32]uint32) InterfaceSnapshot {
	out := InterfaceSnapshot{Name: ifSnap.Name, Ifindex: ifSnap.Ifindex, Filters: ifSnap.Filters}
	out.Qdiscs = make([]tc.Object, 0, len(ifSnap.Qdiscs))
	for _, q := range ifSnap.Qdiscs {
		if !isRootQdisc(q) {
			if newMajor, ok := rewrite[handleMajor(q.Parent)]; ok {
				q.Parent = newMajor | (q.Parent & 0x0000ffff)
			}
		}
		out.Qdiscs = append(out.Qdiscs, q)
	}
	return out
}
