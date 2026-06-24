// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build linux

package netfault

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"

	"github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
	"github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"
)

// openNetNs opens the given netns path (e.g. /proc/<pid>/ns/net or
// /var/run/netns/<name>) and returns its file descriptor. The caller must
// close the returned file when done.
func openNetNs(path string) (*os.File, error) {
	if path == "" {
		return nil, errors.New("empty network namespace path")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open netns %s: %w", path, err)
	}
	return f, nil
}

// takeSnapshot captures the root qdisc tree and filters for every interface in
// `interfaces` within the netns identified by `netNsFd`. Interfaces not found
// in the netns are silently skipped (they may be CNI veths that come and go).
func takeSnapshot(netNsFd int, netNsID string, interfaces []string) (qdiscSnapshot, error) {
	snap := qdiscSnapshot{NetNsID: netNsID, Interfaces: map[string]interfaceSnapshot{}}

	conn, err := tc.Open(&tc.Config{NetNS: netNsFd})
	if err != nil {
		return snap, fmt.Errorf("open tc netlink connection: %w", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			log.Warn().Err(cerr).Msg("close tc netlink connection")
		}
	}()

	qdiscs, err := conn.Qdisc().Get()
	if err != nil {
		return snap, fmt.Errorf("read qdiscs: %w", err)
	}

	ifindexByName, err := interfaceIndexes(netNsFd)
	if err != nil {
		return snap, fmt.Errorf("list interfaces: %w", err)
	}

	for _, name := range interfaces {
		idx, ok := ifindexByName[name]
		if !ok {
			log.Trace().Str("interface", name).Msg("interface not present in netns; skipping snapshot")
			continue
		}
		ifSnap := interfaceSnapshot{Name: name, Ifindex: idx}
		for _, q := range qdiscs {
			if q.Ifindex == idx {
				ifSnap.Qdiscs = append(ifSnap.Qdiscs, q)
			}
		}

		filters, ferr := getFiltersForInterface(conn, idx)
		if ferr != nil {
			// Fail the whole snapshot rather than silently storing an
			// incomplete one — a partial snapshot leads to orphaned filter
			// state on restore, which is worse than no snapshot at all
			// (revert then degrades gracefully to the existing tc-del path).
			return qdiscSnapshot{NetNsID: netNsID}, fmt.Errorf("read filters on %s: %w", name, ferr)
		}
		ifSnap.Filters = filters

		snap.Interfaces[name] = ifSnap
	}

	return snap, nil
}

// restoreSnapshot replays a previously captured qdisc tree onto the same netns.
// For each snapshotted qdisc:
//   - If the kind is kernel-auto-managed (mq, clsact, ingress), skip restoring
//     the qdisc itself. The kernel re-attaches it automatically after
//     `tc qdisc del root`. Its children, however, are restored (replaced over
//     the kernel's defaults).
//   - Otherwise, Replace the qdisc with the saved Object so the operation is
//     idempotent even if some leftover qdisc state already exists.
//
// Restore failures are logged but do not stop the loop; we attempt to restore
// as much state as possible. Per-qdisc errors are joined and returned.
func restoreSnapshot(netNsFd int, snap qdiscSnapshot) error {
	conn, err := tc.Open(&tc.Config{NetNS: netNsFd})
	if err != nil {
		return fmt.Errorf("open tc netlink connection: %w", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			log.Warn().Err(cerr).Msg("close tc netlink connection")
		}
	}()

	// Snapshot the current qdisc tree so we can re-anchor saved children
	// onto whatever kernel-auto-managed root has been re-attached. After
	// `tc qdisc del root` the kernel picks a fresh handle for the new mq /
	// clsact / ingress (commonly handle 0:), so children carrying the
	// saved root's old handle as their Parent field would fail Replace
	// with ENOENT.
	currentQdiscs, err := conn.Qdisc().Get()
	if err != nil {
		return fmt.Errorf("read current qdiscs for re-anchoring: %w", err)
	}

	// Before re-anchoring children, claim the saved auto-managed root
	// handles so children's Parent references resolve. The kernel
	// auto-attaches mq after `tc qdisc del root` but the kernel doesn't
	// expose its real handle to userspace (tc qdisc show prints "0:" and
	// go-tc's Get() returns 0); any attempt to attach a child with
	// Parent=<saved-handle>:N fails with ENOENT. The fix is to issue
	// `tc qdisc replace dev <ifc> root handle <saved>: <kind>`, which
	// makes the kernel re-attach the meta-qdisc under the explicit handle
	// the children expect. go-tc's Qdisc().Replace() doesn't support these
	// parameterless kinds (its validateQdiscObject switch returns
	// ErrNotImplemented for mq), so we shell out to /usr/sbin/tc inside
	// the target netns via setns.
	for name, ifSnap := range snap.Interfaces {
		for _, q := range ifSnap.Qdiscs {
			if !isRootQdisc(q) || !isKernelAutoManaged(q.Kind) || q.Handle == 0 {
				continue
			}
			handleStr := fmt.Sprintf("%x:", q.Handle>>16)
			if err := execTcInNetNs(netNsFd, "qdisc", "replace", "dev", name, "root", "handle", handleStr, q.Kind); err != nil {
				log.Warn().Err(err).Str("interface", name).Str("kind", q.Kind).Str("handle", handleStr).Msg("failed to claim auto-managed root handle; children may fail to restore")
				continue
			}
			log.Debug().Str("interface", name).Str("kind", q.Kind).Str("handle", handleStr).Msg("claimed auto-managed root handle for child re-anchoring")
		}
	}

	// Re-read the live tree after claiming root handles so re-anchor sees
	// the updated state.
	currentQdiscs, err = conn.Qdisc().Get()
	if err != nil {
		return fmt.Errorf("re-read current qdiscs after root reclaim: %w", err)
	}

	var combined error
	for name, ifSnap := range snap.Interfaces {
		// Find the new auto-managed root's handle for this interface and
		// rewrite each child's Parent.major if it matched a saved auto-root.
		ifSnap = reAnchorAutoManagedParents(ifSnap, currentQdiscs)
		ordered := orderQdiscsForRestore(ifSnap.Qdiscs)
		for _, q := range ordered {
			if shouldSkipQdiscOnRestore(q) {
				log.Debug().Str("interface", name).Str("kind", q.Kind).Msg("skipping kernel-auto-managed root qdisc (kernel re-attaches)")
				continue
			}
			obj := q
			stripRuntimeStats(&obj)
			if rerr := conn.Qdisc().Replace(&obj); rerr != nil {
				log.Warn().Err(rerr).Str("interface", name).Str("kind", q.Kind).Uint32("handle", q.Handle).Msg("restore qdisc failed")
				combined = errors.Join(combined, fmt.Errorf("restore qdisc %s on %s: %w", q.Kind, name, rerr))
				continue
			}
			log.Debug().Str("interface", name).Str("kind", q.Kind).Uint32("handle", q.Handle).Uint32("parent", q.Parent).Msg("restored qdisc")
		}

		for _, f := range ifSnap.Filters {
			obj := f
			stripRuntimeStats(&obj)
			// Replace (not Add): if a leftover filter from incomplete attack
			// cleanup is still attached at the same parent/handle, Add fails
			// with "File exists" and the host stays in a mixed state. Replace
			// is idempotent and matches the qdisc restore path above.
			if ferr := conn.Filter().Replace(&obj); ferr != nil {
				log.Warn().Err(ferr).Str("interface", name).Str("kind", f.Kind).Msg("restore filter failed")
				combined = errors.Join(combined, fmt.Errorf("restore filter %s on %s: %w", f.Kind, name, ferr))
				continue
			}
		}
	}
	return combined
}

// interfaceIndexes returns name->ifindex for every interface in the netns
// identified by netNsFd. Uses setns(CLONE_NEWNET) on a locked OS thread to
// scope net.Interfaces() to the target netns, then restores the original.
func interfaceIndexes(netNsFd int) (map[string]uint32, error) {
	runtime.LockOSThread()
	unlock := true
	defer func() {
		if unlock {
			runtime.UnlockOSThread()
		}
	}()

	origNs, err := os.Open("/proc/thread-self/ns/net")
	if err != nil {
		return nil, fmt.Errorf("open current thread netns: %w", err)
	}
	defer func() { _ = origNs.Close() }()

	if err := unix.Setns(netNsFd, unix.CLONE_NEWNET); err != nil {
		return nil, fmt.Errorf("setns target: %w", err)
	}

	ifaces, listErr := net.Interfaces()

	if rerr := unix.Setns(int(origNs.Fd()), unix.CLONE_NEWNET); rerr != nil {
		unlock = false
		return nil, fmt.Errorf("restore netns: %w (list error: %v)", rerr, listErr)
	}

	if listErr != nil {
		return nil, fmt.Errorf("list interfaces in target netns: %w", listErr)
	}

	out := make(map[string]uint32, len(ifaces))
	for _, ifc := range ifaces {
		out[ifc.Name] = uint32(ifc.Index)
	}
	return out, nil
}

// execTcInNetNs runs `/usr/sbin/tc <args...>` inside the network namespace
// identified by netNsFd. Used as a fallback for qdisc kinds that go-tc's
// validateQdiscObject doesn't support (notably mq, which has no parameters
// but isn't in go-tc's switch). The pattern mirrors interfaceIndexes:
// LockOSThread + setns + run + setns-back + UnlockOSThread.
func execTcInNetNs(netNsFd int, args ...string) error {
	runtime.LockOSThread()
	unlock := true
	defer func() {
		if unlock {
			runtime.UnlockOSThread()
		}
	}()

	origNs, err := os.Open("/proc/thread-self/ns/net")
	if err != nil {
		return fmt.Errorf("open current thread netns: %w", err)
	}
	defer func() { _ = origNs.Close() }()

	if err := unix.Setns(netNsFd, unix.CLONE_NEWNET); err != nil {
		return fmt.Errorf("setns target: %w", err)
	}

	cmd := exec.Command("/usr/sbin/tc", args...)
	out, runErr := cmd.CombinedOutput()

	if rerr := unix.Setns(int(origNs.Fd()), unix.CLONE_NEWNET); rerr != nil {
		unlock = false
		return fmt.Errorf("restore netns: %w (tc error: %v, output: %s)", rerr, runErr, string(out))
	}
	if runErr != nil {
		return fmt.Errorf("tc %v failed: %w, output: %s", args, runErr, string(out))
	}
	return nil
}

// getFiltersForInterface enumerates all filters attached to the root qdisc of
// the given interface. The current netfault attacks install all their filters
// under the root prio qdisc's parent space (1:), so capturing filters scoped
// to that parent covers the existing attack patterns. Filters under other
// parents are not captured by this v1.
func getFiltersForInterface(conn *tc.Tc, ifindex uint32) ([]tc.Object, error) {
	parent := core.BuildHandle(1, 0)
	msg := &tc.Msg{
		Family:  unix.AF_UNSPEC,
		Ifindex: ifindex,
		Parent:  parent,
	}
	return conn.Filter().Get(msg)
}
