// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package netfault

import (
	"context"
	"fmt"
	"strings"
)

// Kinds the Linux kernel re-attaches automatically after `tc qdisc del root`.
// Anything else was user-installed (or installed by a CNI bandwidth plugin)
// and cannot be restored after the attack, so the preflight refuses it.
var safeRootQdiscKinds = map[string]struct{}{
	"mq":         {},
	"noqueue":    {},
	"pfifo_fast": {},
	"fq_codel":   {},
	"fq":         {},
}

// strictSafeRootQdiscKinds is the opt-in fallback safe-set: only an interface
// with no real root qdisc (`noqueue`, e.g. a fresh veth) is considered safe to
// replace. Every other pre-existing root — including the kernel default `mq`
// on managed-cloud nodes — is refused at preflight. Enabled via
// SetStrictRootQdisc for operators who don't want network attacks to replace
// any kernel-managed root qdisc.
var strictSafeRootQdiscKinds = map[string]struct{}{
	"noqueue": {},
}

// strictRootQdisc selects strictSafeRootQdiscKinds when true (default false).
var strictRootQdisc bool

// SetStrictRootQdisc toggles the strict preflight safe-set. When enabled, the
// preflight refuses any interface whose root qdisc is not `noqueue`. Off by
// default; intended as a per-deployment opt-out for customers who prefer the
// attack to never touch a pre-existing root qdisc.
func SetStrictRootQdisc(enabled bool) { strictRootQdisc = enabled }

// isSafeRootQdiscKind reports whether a pre-existing root qdisc of the given
// kind may be replaced by the attack under the active configuration.
func isSafeRootQdiscKind(kind string) bool {
	safe := safeRootQdiscKinds
	if strictRootQdisc {
		safe = strictSafeRootQdiscKinds
	}
	_, ok := safe[kind]
	return ok
}

// inspectRootQdiscs returns a map from interface name to root qdisc kind for
// every interface in the runner's network namespace. Uses the human-readable
// `tc qdisc show` output (not -json) so the check works on older iproute2.
func inspectRootQdiscs(ctx context.Context, runner CommandRunner) (map[string]string, error) {
	out, err := runner.run(ctx, []string{"tc", "qdisc", "show"}, nil)
	if err != nil {
		return nil, fmt.Errorf("tc qdisc show failed: %w", err)
	}
	return parseRootQdiscKinds(out), nil
}

// parseRootQdiscKinds returns a map from interface name to root qdisc kind.
// Root qdiscs are formatted as `qdisc <kind> <handle> dev <ifc> root ...`;
// `ingress`/`clsact` lines use `parent` instead of `root` and are skipped.
func parseRootQdiscKinds(out string) map[string]string {
	kinds := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 6 || fields[0] != "qdisc" || fields[3] != "dev" || fields[5] != "root" {
			continue
		}
		kinds[fields[4]] = fields[1]
	}
	return kinds
}
