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

type Opts interface {
	ipCommands(family family, mode mode) ([]string, error)
	tcCommands(mode mode) ([]string, error)
	String() string
	toExecutionContext() ExecutionContext
	doesConflictWith(opts Opts) bool
}

type ExecutionContext struct {
	ExperimentKey         string
	ExperimentExecutionId int
	TargetExecutionId     string
}

type iptablesScriptProvider interface {
	iptablesScripts(mode mode) (v4 []string, v6 []string, err error)
}
