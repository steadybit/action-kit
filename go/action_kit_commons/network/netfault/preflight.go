// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package netfault

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"
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
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 6 || fields[0] != "qdisc" || fields[3] != "dev" || fields[5] != "root" {
			continue
		}
		kinds[fields[4]] = fields[1]
	}
	return kinds
}

// ErrNoDefaultLocalRule reports that the network namespace does not carry the
// kernel's default `pref 0 lookup local` routing rule, which blocking localhost
// traffic needs to move aside (and to restore afterwards).
type ErrNoDefaultLocalRule struct {
	Family string
}

func (e *ErrNoDefaultLocalRule) Error() string {
	return fmt.Sprintf("blocking localhost traffic requires the default routing rule '0: from all lookup local' (%s), but it was not found. The routing rules seem to be customized; localhost traffic cannot be blocked safely.", e.Family)
}

// preflightLocalTableRule verifies the default `pref 0 lookup local` rule is
// present for every family a BlockLocalhost blackhole will touch. Skipped when
// a steadybit attack is already active in the namespace: the rule may already
// have been moved by it and the apply-time conflict detection decides.
func preflightLocalTableRule(ctx context.Context, runner CommandRunner, opts Opts) error {
	o, ok := opts.(*BlackholeOpts)
	if !ok || !o.BlockLocalhost {
		return nil
	}
	if hasActiveNetfault(runner.id()) {
		return nil
	}

	families := []family{familyV4}
	if ipv6Supported() {
		families = append(families, familyV6)
	}
	for _, fam := range families {
		out, err := executeIpCommands(ctx, runner, []string{"rule show pref 0"}, ipFamilyFlag, string(fam))
		if err != nil {
			return fmt.Errorf("failed to inspect ip rules: %w", err)
		}
		if !hasDefaultLocalRule(out) {
			return &ErrNoDefaultLocalRule{Family: string(fam)}
		}
	}
	return nil
}

// hasDefaultLocalRule reports whether `ip rule show pref 0` output contains the
// unconditional local table lookup (`0:	from all lookup local`).
func hasDefaultLocalRule(out string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.Join(strings.Fields(line), " ") == "0: from all lookup local" {
			return true
		}
	}
	return false
}

// skipRestoreIfDefaultLocalRulePresent drops the revert's re-add of the default
// local rule when that rule is still present, e.g. because the attack failed to
// start before removing it. The kernel accepts a duplicate pref 0 rule, so the
// re-add must not run blindly.
func skipRestoreIfDefaultLocalRulePresent(ctx context.Context, runner CommandRunner, family family, cmds []string) []string {
	idx := slices.Index(cmds, restoreDefaultLocalRuleCmd)
	if idx < 0 {
		return cmds
	}
	out, err := executeIpCommands(ctx, runner, []string{"rule show pref 0"}, ipFamilyFlag, string(family))
	if err != nil {
		log.Warn().Err(err).Msg("failed to inspect ip rules; restoring the default local rule anyway")
		return cmds
	}
	if !hasDefaultLocalRule(out) {
		return cmds
	}
	return slices.Delete(slices.Clone(cmds), idx, idx+1)
}
