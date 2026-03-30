// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

//go:build !windows

package dnsinject

import (
	"net"
	"testing"

	"github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/stretchr/testify/assert"
)

func TestOptsToArgs(t *testing.T) {
	opts := Opts{
		ErrorTypes: []ErrorType{ErrorTypeNXDOMAIN, ErrorTypeSERVFAIL},
		CIDRs: []net.IPNet{
			{IP: net.IPv4(10, 0, 0, 1), Mask: net.CIDRMask(32, 32)},
			{IP: net.IPv4(172, 17, 0, 0), Mask: net.CIDRMask(16, 32)},
		},
		PortRange:  network.PortRange{From: 53, To: 53},
		Interfaces: []string{"eth0", "docker0"},
	}

	args := opts.toArgs()

	assert.Equal(t, []string{
		"--error-type", "NXDOMAIN",
		"--error-type", "SERVFAIL",
		"--cidr", "10.0.0.1/32",
		"--cidr", "172.17.0.0/16",
		"--port", "53",
		"--interface", "eth0",
		"--interface", "docker0",
	}, args)
}

func TestOptsToArgsPortRange(t *testing.T) {
	opts := Opts{
		ErrorTypes: []ErrorType{ErrorTypeTimeout},
		PortRange:  network.PortRange{From: 1, To: 65534},
	}

	args := opts.toArgs()

	assert.Contains(t, args, "--port")
	assert.Contains(t, args, "1-65534")
}

func TestOptsToArgsMinimal(t *testing.T) {
	opts := Opts{
		ErrorTypes: []ErrorType{ErrorTypeNXDOMAIN},
		PortRange:  network.PortRange{From: 53, To: 53},
	}

	args := opts.toArgs()

	assert.Equal(t, []string{
		"--error-type", "NXDOMAIN",
		"--port", "53",
	}, args)
}
