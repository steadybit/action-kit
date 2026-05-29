// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package netfault

import "github.com/steadybit/action-kit/go/action_kit_commons/network"

type mode string
type family string

const (
	modeAdd    mode   = "add"
	modeDelete mode   = "del"
	familyV4   family = "inet"
	familyV6   family = "inet6"
)

type Filter struct {
	Include []network.NetWithPortRange
	Exclude []network.NetWithPortRange
}

// Opts is the common contract every netfault attack implements. Subsystem-
// specific behavior is opt-in via the *Provider interfaces below — an attack
// only implements the providers for the subsystems it actually uses.
type Opts interface {
	String() string
	toExecutionContext() ExecutionContext
	doesConflictWith(opts Opts) bool
}

type ExecutionContext struct {
	ExperimentKey         string
	ExperimentExecutionId int
	TargetExecutionId     string
}

// tcCommandProvider is implemented by attacks that use the tc subsystem.
// tcRootQdiscInterfaces names the interfaces whose root qdisc the attack
// installs; the preflight uses it to detect pre-existing user qdiscs.
type tcCommandProvider interface {
	tcCommands(mode mode) ([]string, error)
	tcRootQdiscInterfaces() []string
}

// ipCommandProvider is implemented by attacks that install ip rules (policy
// routing).
type ipCommandProvider interface {
	ipCommands(family family, mode mode) ([]string, error)
}

// iptablesScriptProvider is implemented by attacks that install iptables /
// ip6tables rules via iptables-restore.
type iptablesScriptProvider interface {
	iptablesScripts(mode mode) (v4 []string, v6 []string, err error)
}
