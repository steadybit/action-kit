// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

//go:build linux

package runc

import (
	"context"
	"github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"
	"sync"
)

var isCapSysPtraceSet = hasCap(unix.CAP_SYS_PTRACE)
var logWarning = sync.OnceFunc(func() {
	log.Warn().Msg("CAP_SYS_PTRACE capability is not set. Using fallback with reduced performance.")
})

func executeReadlinkInProc(ctx context.Context, nsPaths ...string) ([]string, error) {
	if isCapSysPtraceSet {
		return executeReadlinkUsingSyscall(ctx, nsPaths...)
	} else {
		logWarning()
		return executeReadlinkUsingExec(ctx, nsPaths...)
	}
}

func hasCap(cap uint) bool {
	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	data := [2]unix.CapUserData{}
	if err := unix.Capget(&hdr, &data[0]); err != nil {
		log.Trace().Err(err).Msg("failed to get capabilities")
		return false
	}
	return data[0].Effective&(1<<cap) != 0
}
