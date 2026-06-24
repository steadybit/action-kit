// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !linux && !windows

package netfault

import (
	"errors"
	"os"
)

// errSnapshotUnsupported is returned by snapshot/restore operations on
// non-Linux builds where RTNETLINK is unavailable. Callers should treat the
// feature as a no-op on these platforms.
var errSnapshotUnsupported = errors.New("qdisc snapshot/restore unsupported on this platform (requires Linux RTNETLINK)")

func openNetNs(_ string) (*os.File, error) {
	return nil, errSnapshotUnsupported
}

func takeSnapshot(_ int, netNsID string, _ []string) (qdiscSnapshot, error) {
	return qdiscSnapshot{NetNsID: netNsID, Interfaces: map[string]interfaceSnapshot{}}, errSnapshotUnsupported
}

func restoreSnapshot(_ int, _ qdiscSnapshot) error {
	return errSnapshotUnsupported
}
