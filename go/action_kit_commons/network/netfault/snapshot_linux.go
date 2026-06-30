// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build linux

package netfault

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"

	"github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
	"github.com/mdlayher/netlink"
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
func takeSnapshot(netNsFd int, netNsID string, interfaces []string) (QdiscSnapshot, error) {
	snap := QdiscSnapshot{NetNsID: netNsID, Interfaces: map[string]InterfaceSnapshot{}}

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
		ifSnap := InterfaceSnapshot{Name: name, Ifindex: idx}
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
			return QdiscSnapshot{NetNsID: netNsID}, fmt.Errorf("read filters on %s: %w", name, ferr)
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
func restoreSnapshot(netNsFd int, snap QdiscSnapshot) error {
	conn, err := tc.Open(&tc.Config{NetNS: netNsFd})
	if err != nil {
		return fmt.Errorf("open tc netlink connection: %w", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			log.Warn().Err(cerr).Msg("close tc netlink connection")
		}
	}()

	claimAutoManagedRootHandles(netNsFd, snap)

	// Re-read the live tree after claiming root handles so re-anchor sees
	// the updated state.
	currentQdiscs, err := conn.Qdisc().Get()
	if err != nil {
		return fmt.Errorf("re-read current qdiscs after root reclaim: %w", err)
	}

	var combined error
	for name, ifSnap := range snap.Interfaces {
		anchored := reAnchorAutoManagedParents(ifSnap, currentQdiscs)
		combined = errors.Join(combined, restoreInterfaceQdiscs(conn, name, anchored))
		combined = errors.Join(combined, restoreInterfaceFilters(conn, name, anchored))
	}
	return combined
}

// claimAutoManagedRootHandles walks each saved auto-managed root with an
// explicit handle and reassigns the kernel's anonymous live root to that
// handle. After `tc qdisc del root` the kernel re-attaches mq / clsact /
// ingress under a hidden handle (tc qdisc show prints "0:", go-tc's Get()
// returns 0); any attempt to attach a saved child with the old handle then
// fails with ENOENT. We use a raw RTNETLINK message (claimAutoManagedRoot)
// because go-tc's validateQdiscObject lacks an "mq" case.
func claimAutoManagedRootHandles(netNsFd int, snap QdiscSnapshot) {
	for name, ifSnap := range snap.Interfaces {
		for _, q := range ifSnap.Qdiscs {
			if !shouldClaimRoot(q) {
				continue
			}
			if err := claimAutoManagedRoot(netNsFd, ifSnap.Ifindex, q.Handle, q.Kind); err != nil {
				log.Warn().Err(err).Str("interface", name).Str("kind", q.Kind).Uint32("handle", q.Handle).Msg("failed to claim auto-managed root handle; children may fail to restore")
				continue
			}
			log.Debug().Str("interface", name).Str("kind", q.Kind).Uint32("handle", q.Handle).Msg("claimed auto-managed root handle for child re-anchoring")
		}
	}
}

func shouldClaimRoot(q tc.Object) bool {
	return isRootQdisc(q) && isKernelAutoManaged(q.Kind) && q.Handle != 0
}

// restoreInterfaceQdiscs replays the saved qdiscs for one interface in
// parent-first order. Kernel-auto-managed kinds are skipped; the rest are
// Replace()d after their kernel-counter fields are zeroed.
func restoreInterfaceQdiscs(conn *tc.Tc, name string, ifSnap InterfaceSnapshot) error {
	var combined error
	for _, q := range orderQdiscsForRestore(ifSnap.Qdiscs) {
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
	return combined
}

// restoreInterfaceFilters replays the saved filters for one interface via
// Replace() (not Add) so leftover filters from incomplete attack cleanup
// are overwritten rather than rejected with "File exists".
func restoreInterfaceFilters(conn *tc.Tc, name string, ifSnap InterfaceSnapshot) error {
	var combined error
	for _, f := range ifSnap.Filters {
		obj := f
		stripRuntimeStats(&obj)
		if ferr := conn.Filter().Replace(&obj); ferr != nil {
			log.Warn().Err(ferr).Str("interface", name).Str("kind", f.Kind).Msg("restore filter failed")
			combined = errors.Join(combined, fmt.Errorf("restore filter %s on %s: %w", f.Kind, name, ferr))
			continue
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

// claimAutoManagedRoot sends a raw RTM_NEWQDISC netlink message that
// reassigns the device's root qdisc to the given (handle, kind). Used to
// claim a saved handle for kernel-auto-managed kinds (mq, clsact, ingress)
// so that saved children's Parent.major references resolve.
//
// We can't go through go-tc here because validateQdiscObject's switch
// doesn't include mq (it has cases for clsact and ingress as parameterless
// kinds but no mq case, so Replace returns ErrNotImplemented). We can't
// shell out to /usr/sbin/tc either because the extension process runs as
// non-root: exec drops capabilities since tc has no file caps, so the
// child gets EPERM from the kernel. Going through the netlink socket
// directly works because the calling process retains CAP_NET_ADMIN in its
// Effective set when sending the RTNETLINK message.
func claimAutoManagedRoot(netNsFd int, ifindex uint32, handle uint32, kind string) error {
	conn, err := netlink.Dial(unix.NETLINK_ROUTE, &netlink.Config{NetNS: netNsFd})
	if err != nil {
		return fmt.Errorf("dial netlink for raw claim: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Build tcmsg (20 bytes, native endianness for the integer fields):
	//   family:1 _:1 _:2 ifindex:4 handle:4 parent:4 info:4
	tcmsg := make([]byte, 20)
	nativeEndian.PutUint32(tcmsg[4:8], ifindex)
	nativeEndian.PutUint32(tcmsg[8:12], handle)
	nativeEndian.PutUint32(tcmsg[12:16], tcHRoot) // TC_H_ROOT = 0xffffffff

	// One TCA_KIND attribute (type=1) with the null-terminated kind string.
	ae := netlink.NewAttributeEncoder()
	ae.String(1, kind)
	attrs, err := ae.Encode()
	if err != nil {
		return fmt.Errorf("encode netlink attribute: %w", err)
	}

	msg := netlink.Message{
		Header: netlink.Header{
			Type:  netlink.HeaderType(unix.RTM_NEWQDISC),
			Flags: netlink.Request | netlink.Create | netlink.Replace | netlink.Acknowledge,
		},
		Data: append(tcmsg, attrs...),
	}

	if _, err := conn.Execute(msg); err != nil {
		return fmt.Errorf("netlink claim qdisc kind=%s handle=%#x: %w", kind, handle, err)
	}
	return nil
}

// nativeEndian is the byte order RTNETLINK expects on the host; it matches
// the architecture's CPU endianness. Go 1.21+ exposes this directly.
var nativeEndian = binary.NativeEndian

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
