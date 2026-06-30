// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package netfault

import (
	"fmt"
	"sort"
	"strings"

	"github.com/florianl/go-tc"
)

// renderSnapshot returns a multi-line human-readable representation of the
// snapshot, similar in spirit to `tc qdisc show` / `tc filter show` output.
// Used for debug logging so operators can verify what was captured at apply
// and what was replayed at revert.
//
// Interface names are sorted so the output is stable across calls.
// Per-interface, qdiscs are rendered in restore order (parent-first via
// orderQdiscsForRestore) and filters are listed afterwards.
func renderSnapshot(snap QdiscSnapshot) string {
	if len(snap.Interfaces) == 0 {
		return "(empty snapshot)"
	}
	names := make([]string, 0, len(snap.Interfaces))
	for name := range snap.Interfaces {
		names = append(names, name)
	}
	sort.Strings(names)

	var sb strings.Builder
	for _, name := range names {
		ifSnap := snap.Interfaces[name]
		for _, q := range orderQdiscsForRestore(ifSnap.Qdiscs) {
			renderQdiscLine(&sb, name, q)
		}
		for _, f := range ifSnap.Filters {
			renderFilterLine(&sb, name, f)
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// renderQdiscLine writes one `tc qdisc show`-style line into sb. Handle and
// Parent are printed in hex major:minor form to match iproute2's convention,
// and the qdisc's kind-specific parameters are pulled from the typed
// Attribute fields where supported, with a generic fallback for kinds we
// haven't formatted explicitly.
func renderQdiscLine(sb *strings.Builder, ifc string, q tc.Object) {
	fmt.Fprintf(sb, "qdisc %s %s dev %s %s", q.Kind, formatHandle(q.Handle), ifc, formatParent(q.Parent))
	if details := renderQdiscDetails(q); details != "" {
		sb.WriteString(" ")
		sb.WriteString(details)
	}
	sb.WriteByte('\n')
}

// renderFilterLine writes one `tc filter show`-style line into sb.
func renderFilterLine(sb *strings.Builder, ifc string, f tc.Object) {
	fmt.Fprintf(sb, "filter %s dev %s parent %s handle %s", f.Kind, ifc, formatParent(f.Parent), formatHandle(f.Handle))
	if f.Chain != nil {
		fmt.Fprintf(sb, " chain %d", *f.Chain)
	}
	sb.WriteByte('\n')
}

// renderQdiscDetails renders the kind-specific Attribute fields for the
// qdiscs netfault actually deals with (fq, netem, htb, prio, fq_codel, tbf).
// For other kinds it returns the empty string — the caller still gets the
// kind/handle/parent header line, which is enough to identify the qdisc.
func renderQdiscDetails(q tc.Object) string {
	switch q.Kind {
	case "fq":
		return renderFq(q.Fq)
	case "fq_codel":
		return renderFqCodel(q.FqCodel)
	case "netem":
		return renderNetem(q.Netem)
	case "htb":
		return renderHtb(q.Htb)
	case "prio":
		return renderPrio(q.Prio)
	case "tbf":
		return renderTbf(q.Tbf)
	}
	return ""
}

func renderFq(fq *tc.Fq) string {
	if fq == nil {
		return ""
	}
	var parts []string
	if fq.PLimit != nil {
		parts = append(parts, fmt.Sprintf("limit %dp", *fq.PLimit))
	}
	if fq.FlowPLimit != nil {
		parts = append(parts, fmt.Sprintf("flow_limit %dp", *fq.FlowPLimit))
	}
	if fq.BucketsLog != nil {
		parts = append(parts, fmt.Sprintf("buckets %d", 1<<(*fq.BucketsLog)))
	}
	if fq.Quantum != nil {
		parts = append(parts, fmt.Sprintf("quantum %db", *fq.Quantum))
	}
	if fq.InitQuantum != nil {
		parts = append(parts, fmt.Sprintf("initial_quantum %db", *fq.InitQuantum))
	}
	if fq.FlowRefillDelay != nil {
		parts = append(parts, fmt.Sprintf("refill_delay %s", formatMicroseconds(*fq.FlowRefillDelay/1000)))
	}
	if fq.Horizon != nil {
		parts = append(parts, fmt.Sprintf("horizon %s", formatMicroseconds(*fq.Horizon)))
	}
	return strings.Join(parts, " ")
}

func renderFqCodel(fc *tc.FqCodel) string {
	if fc == nil {
		return ""
	}
	var parts []string
	if fc.Limit != nil {
		parts = append(parts, fmt.Sprintf("limit %dp", *fc.Limit))
	}
	if fc.Flows != nil {
		parts = append(parts, fmt.Sprintf("flows %d", *fc.Flows))
	}
	if fc.Target != nil {
		parts = append(parts, fmt.Sprintf("target %s", formatMicroseconds(*fc.Target)))
	}
	if fc.Interval != nil {
		parts = append(parts, fmt.Sprintf("interval %s", formatMicroseconds(*fc.Interval)))
	}
	if fc.Quantum != nil {
		parts = append(parts, fmt.Sprintf("quantum %d", *fc.Quantum))
	}
	return strings.Join(parts, " ")
}

func renderNetem(n *tc.Netem) string {
	if n == nil {
		return ""
	}
	var parts []string
	if n.Qopt.Latency != 0 {
		parts = append(parts, fmt.Sprintf("delay %dus", n.Qopt.Latency))
	}
	if n.Qopt.Jitter != 0 {
		parts = append(parts, fmt.Sprintf("jitter %dus", n.Qopt.Jitter))
	}
	if n.Qopt.Loss != 0 {
		parts = append(parts, fmt.Sprintf("loss %d/%d", n.Qopt.Loss, 0xffffffff))
	}
	if n.Corrupt != nil {
		parts = append(parts, fmt.Sprintf("corrupt %d/%d", n.Corrupt.Probability, 0xffffffff))
	}
	if n.Qopt.Duplicate != 0 {
		parts = append(parts, fmt.Sprintf("duplicate %d/%d", n.Qopt.Duplicate, 0xffffffff))
	}
	return strings.Join(parts, " ")
}

func renderHtb(h *tc.Htb) string {
	if h == nil || h.Init == nil {
		return ""
	}
	return fmt.Sprintf("r2q %d default %d", h.Init.Rate2Quantum, h.Init.Defcls)
}

func renderPrio(p *tc.Prio) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("bands %d", p.Bands)
}

func renderTbf(t *tc.Tbf) string {
	if t == nil || t.Parms == nil {
		return ""
	}
	return fmt.Sprintf("rate %d burst %d", t.Parms.Rate.Rate, t.Parms.Buffer)
}

// formatHandle renders a netlink handle as the iproute2 `major:minor` form.
// A zero handle is rendered as `0:` (matches `tc qdisc show` for newly-added
// qdiscs the kernel hasn't assigned a handle to yet).
func formatHandle(h uint32) string {
	major := h >> 16
	minor := h & 0xffff
	return fmt.Sprintf("%x:%x", major, minor)
}

// formatParent renders the parent field. TC_H_ROOT (0xfffffff1) and 0 both
// indicate "attached at the device root" so we render them as `root`,
// matching iproute2.
func formatParent(p uint32) string {
	if p == tcHRoot || p == 0 {
		return "root"
	}
	return fmt.Sprintf("parent %s", formatHandle(p))
}

// formatMicroseconds renders a microsecond duration as seconds, milliseconds,
// or microseconds depending on magnitude — chosen to mirror iproute2's
// human-readable output (`2s`, `40ms`, `10us`).
func formatMicroseconds(us uint32) string {
	switch {
	case us >= 1_000_000 && us%1_000_000 == 0:
		return fmt.Sprintf("%ds", us/1_000_000)
	case us >= 1_000 && us%1_000 == 0:
		return fmt.Sprintf("%dms", us/1_000)
	default:
		return fmt.Sprintf("%dus", us)
	}
}

// compareSnapshotsByHandle reports a short human-readable diff between two
// snapshots, comparing qdiscs by (interface, handle, kind, parent) and
// filters by (interface, kind, parent, handle). Stats and counter-only
// differences are ignored — those legitimately diverge after a restore.
// Returns the empty string when the two snapshots are equivalent at the
// shape level.
func compareSnapshotsByHandle(before, after QdiscSnapshot) string {
	var diffs []string
	for name, b := range before.Interfaces {
		a, ok := after.Interfaces[name]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("%s: missing from post-restore state", name))
			continue
		}
		diffs = append(diffs, diffQdiscSets(name, b.Qdiscs, a.Qdiscs)...)
		diffs = append(diffs, diffFilterSets(name, b.Filters, a.Filters)...)
	}
	for name := range after.Interfaces {
		if _, ok := before.Interfaces[name]; !ok {
			diffs = append(diffs, fmt.Sprintf("%s: appeared in post-restore state but not in snapshot", name))
		}
	}
	return strings.Join(diffs, "; ")
}

func diffQdiscSets(ifc string, before, after []tc.Object) []string {
	// Build sets by (handle, kind, parent). Kernel-auto-managed kinds we
	// don't restore so we exclude them on both sides.
	type key struct {
		Handle uint32
		Kind   string
		Parent uint32
	}
	set := func(qs []tc.Object) map[key]struct{} {
		out := map[key]struct{}{}
		for _, q := range qs {
			if isKernelAutoManaged(q.Kind) {
				continue
			}
			out[key{Handle: handleMajor(q.Handle), Kind: q.Kind, Parent: q.Parent}] = struct{}{}
		}
		return out
	}
	bs, as := set(before), set(after)
	var diffs []string
	for k := range bs {
		if _, ok := as[k]; !ok {
			diffs = append(diffs, fmt.Sprintf("%s: qdisc %s %s parent %s missing after restore", ifc, k.Kind, formatHandle(k.Handle), formatParent(k.Parent)))
		}
	}
	for k := range as {
		if _, ok := bs[k]; !ok {
			diffs = append(diffs, fmt.Sprintf("%s: qdisc %s %s parent %s present after restore but not in snapshot", ifc, k.Kind, formatHandle(k.Handle), formatParent(k.Parent)))
		}
	}
	return diffs
}

func diffFilterSets(ifc string, before, after []tc.Object) []string {
	type key struct {
		Handle uint32
		Kind   string
		Parent uint32
	}
	set := func(fs []tc.Object) map[key]struct{} {
		out := map[key]struct{}{}
		for _, f := range fs {
			out[key{Handle: f.Handle, Kind: f.Kind, Parent: f.Parent}] = struct{}{}
		}
		return out
	}
	bs, as := set(before), set(after)
	var diffs []string
	for k := range bs {
		if _, ok := as[k]; !ok {
			diffs = append(diffs, fmt.Sprintf("%s: filter %s parent %s handle %s missing after restore", ifc, k.Kind, formatParent(k.Parent), formatHandle(k.Handle)))
		}
	}
	return diffs
}
