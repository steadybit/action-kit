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
