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

// hasSnapshot reports whether a snapshot exists for the given netns id without
// mutating the store. Used to skip taking a second snapshot when a
// non-conflicting attack joins an active one.
func hasSnapshot(netNsID string) bool {
	snapshotStoreLock.Lock()
	defer snapshotStoreLock.Unlock()
	_, ok := snapshotStore[netNsID]
	return ok
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

// orderQdiscsForRestore returns the snapshot's qdiscs sorted parent-first.
// Roots get sorted before any child whose Parent equals one of the
// snapshotted handles.
func orderQdiscsForRestore(qs []tc.Object) []tc.Object {
	const tcHRoot uint32 = 0xfffffff1
	out := make([]tc.Object, 0, len(qs))
	for _, q := range qs {
		if q.Parent == tcHRoot || q.Parent == 0 {
			out = append(out, q)
		}
	}
	for _, q := range qs {
		if q.Parent != tcHRoot && q.Parent != 0 {
			out = append(out, q)
		}
	}
	return out
}
