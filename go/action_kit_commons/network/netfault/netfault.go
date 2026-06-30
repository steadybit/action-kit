// Copyright 2025 steadybit GmbH. All rights reserved.
//go:build !windows

package netfault

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
)

const maxTcCommands = 2048

var (
	ipPath = utils.LocateExecutable("ip", "STEADYBIT_EXTENSION_IP_PATH")

	runLock = utils.NewHashedKeyMutex(10)

	activeNetfaultLock = sync.Mutex{}
	activeNetfault     = map[string][]Opts{}
)

type ErrTooManyTcCommands struct {
	Count int
}

func (e *ErrTooManyTcCommands) Error() string {
	return fmt.Sprintf("too many tc commands: %d", e.Count)
}

type CommandRunner interface {
	run(ctx context.Context, processArgs []string, cmds []string) (string, error)
	id() string
	// netNsPath returns the filesystem path to the network namespace this
	// runner operates on (e.g. /proc/<pid>/ns/net or /var/run/netns/<name>).
	// Used by the snapshot layer to open the netns via RTNETLINK.
	netNsPath() string
}

// Apply installs the attack and returns a QdiscSnapshot describing the
// pre-attack qdisc tree. The caller is expected to persist the snapshot in
// the action's per-execution state (action_kit_sdk JSON state) and pass it
// back to Revert.
//
// The returned snapshot is empty (QdiscSnapshot.IsEmpty() == true) when:
//   - strict-root-qdisc mode is on (preflight refused non-`noqueue` roots —
//     there's nothing to preserve),
//   - the opts do not implement tcCommandProvider (no tc root touched),
//   - the snapshot capture itself errored (logged; attack still proceeds).
//
// On Apply failure the returned snapshot is empty even when capture
// succeeded: the snapshot would describe a state the attack never fully
// replaced, and replaying it from Revert would clobber a partial attack
// install with the original tree. Drop it instead.
func Apply(ctx context.Context, runner CommandRunner, opts Opts) (QdiscSnapshot, error) {
	return generateAndRunCommands(ctx, runner, opts, modeAdd, QdiscSnapshot{})
}

// ErrUserRootQdisc reports that a target interface already carries a
// pre-existing root qdisc that the attack will not replace under the active
// configuration (a user/CNI qdisc such as `htb`/`cake` by default, or — when
// SetStrictRootQdisc is enabled — anything other than `noqueue`, including the
// kernel default `mq`).
type ErrUserRootQdisc struct {
	Interface string
	Kind      string
}

func (e *ErrUserRootQdisc) Error() string {
	return fmt.Sprintf("interface %q already has a root qdisc %q that the network attack will not replace under the current configuration. Remove the existing qdisc or exclude this interface from the attack.", e.Interface, e.Kind)
}

// PreflightCheck inspects the root qdiscs of the interfaces the attack installs
// its own root qdisc on and returns an *ErrUserRootQdisc when any carries a
// non-default (user/CNI-installed) qdisc. It is meant to be called from the
// attack's Prepare step so the experiment fails fast and cleanly without
// touching the host.
//
// Kernel-default root qdiscs (mq/noqueue/fq_codel/fq/pfifo_fast) are accepted:
// `tc qdisc replace` grafts over them at Start and the kernel restores them on
// revert. If a steadybit attack is already active on the same network
// namespace the existing root qdisc is ours, so the check is skipped and the
// apply-time conflict detection decides whether the attacks may coexist.
func PreflightCheck(ctx context.Context, runner CommandRunner, opts Opts) error {
	p, ok := opts.(tcCommandProvider)
	if !ok {
		return nil
	}
	interfaces := p.tcRootQdiscInterfaces()
	if len(interfaces) == 0 {
		return nil
	}
	if hasActiveNetfault(runner.id()) {
		return nil
	}

	kinds, err := inspectRootQdiscs(ctx, runner)
	if err != nil {
		log.Warn().Err(err).Msg("failed to inspect root qdiscs; skipping preflight check")
		return nil
	}
	for _, ifc := range interfaces {
		kind := kinds[ifc]
		if kind == "" {
			continue
		}
		if isSafeRootQdiscKind(kind) {
			continue
		}
		return &ErrUserRootQdisc{Interface: ifc, Kind: kind}
	}
	return nil
}

func hasActiveNetfault(netNsId string) bool {
	activeNetfaultLock.Lock()
	defer activeNetfaultLock.Unlock()
	return len(activeNetfault[netNsId]) > 0
}

// Revert removes the attack and replays the snapshot (if non-empty) onto
// the netns root qdisc tree. `snap` should be the value Apply returned for
// the same opts; pass a zero QdiscSnapshot to skip the restore step (used
// by callers that don't care about preserving the pre-attack tree, e.g.
// iptables-only attacks).
func Revert(ctx context.Context, runner CommandRunner, opts Opts, snap QdiscSnapshot) error {
	_, err := generateAndRunCommands(ctx, runner, opts, modeDelete, snap)
	return err
}

// generateAndRunCommands is the shared body for Apply (modeAdd) and Revert
// (modeDelete). On Add it captures a snapshot before running the attack
// commands and returns it to the caller; on Delete it consumes the snapshot
// from the caller and replays it after running the cleanup commands.
//
// The `incoming` snapshot is meaningful on Delete only — Apply ignores it.
// The returned snapshot is meaningful on Add only — Delete always returns an
// empty one. Splitting Apply/Revert into separate exported functions keeps
// the public API typed (Apply returns a snapshot, Revert doesn't) without
// the shared body needing a sum type.
func generateAndRunCommands(ctx context.Context, runner CommandRunner, opts Opts, mode mode, incoming QdiscSnapshot) (QdiscSnapshot, error) {
	ipCommandsV4, ipCommandsV6, tcCommands, err := generateCommands(opts, mode)
	if err != nil {
		return QdiscSnapshot{}, err
	}
	logPreparedCommands(mode, ipCommandsV4, ipCommandsV6, tcCommands)

	netNsID := runner.id()
	runLock.LockKey(netNsID)
	defer func() { _ = runLock.UnlockKey(netNsID) }()

	var snapshot QdiscSnapshot
	if mode == modeAdd {
		if err := pushActiveNetfault(netNsID, opts); err != nil {
			return QdiscSnapshot{}, err
		}
		snapshot = captureSnapshotForApply(runner, netNsID, opts)
	}

	logBeforeAfterRules(ctx, runner, ipCommandsV4, ipCommandsV6, tcCommands, "before")

	if scriptErr := runIptablesScripts(ctx, runner, opts, mode, &err); scriptErr != nil {
		return QdiscSnapshot{}, scriptErr
	}
	runBatchCommands(ctx, runner, mode, ipCommandsV4, ipCommandsV6, tcCommands, &err)

	logBeforeAfterRules(ctx, runner, ipCommandsV4, ipCommandsV6, tcCommands, "after")

	// If apply failed after taking a snapshot, drop it: the snapshot describes
	// a state the attack never fully replaced, so a later revert would replay
	// the original tree against a partially-installed attack. Better to leave
	// the host with the kernel's default-restore path than with a stale
	// snapshot we can't trust.
	if mode == modeAdd && err != nil && !snapshot.IsEmpty() {
		log.Warn().Str("netNs", netNsID).Msg("dropped qdisc snapshot because apply errored; revert will fall back to kernel-default restore")
		snapshot = QdiscSnapshot{}
	}

	if mode == modeDelete {
		popActiveNetfault(netNsID, opts)
		// Replay the caller-provided snapshot (if any) after the attack's
		// tc-del has run. Strict-mode check guards against a caller passing
		// in a snapshot from a mode flip — there's no scenario where Revert
		// should replay a snapshot when strict mode is on at revert time.
		if !strictRootQdisc && !incoming.IsEmpty() {
			if rerr := applyRestore(runner, incoming); rerr != nil {
				log.Warn().Err(rerr).Str("netNs", netNsID).Msg("qdisc restore failed")
				err = errors.Join(err, rerr)
			}
		}
	}

	return snapshot, err
}

// generateCommands invokes the opt-in command providers and returns the
// prepared ip / tc batch command strings. Any provider error short-circuits
// and is returned to the caller.
func generateCommands(opts Opts, mode mode) (ipV4, ipV6, tcCmds []string, err error) {
	if p, ok := opts.(ipCommandProvider); ok {
		if ipV4, err = p.ipCommands(familyV4, mode); err != nil {
			return nil, nil, nil, err
		}
		if ipv6Supported() {
			if ipV6, err = p.ipCommands(familyV6, mode); err != nil {
				return nil, nil, nil, err
			}
		}
	}
	if p, ok := opts.(tcCommandProvider); ok {
		if tcCmds, err = p.tcCommands(mode); err != nil {
			return nil, nil, nil, err
		}
	}
	return ipV4, ipV6, tcCmds, nil
}

// logPreparedCommands emits the prepared batch commands at DEBUG level so
// operators reproducing an attack can see exactly what would be applied.
// No-op when DEBUG is not enabled.
func logPreparedCommands(mode mode, ipV4, ipV6, tcCmds []string) {
	if !log.Debug().Enabled() {
		return
	}
	if len(ipV4) > 0 {
		log.Debug().Str("mode", string(mode)).Strs("ip_cmds_v4", ipV4).Msg("prepared ip batch commands (IPv4)")
	}
	if len(ipV6) > 0 {
		log.Debug().Str("mode", string(mode)).Strs("ip_cmds_v6", ipV6).Msg("prepared ip batch commands (IPv6)")
	}
	if len(tcCmds) > 0 {
		log.Debug().Str("mode", string(mode)).Strs("tc_cmds", tcCmds).Msg("prepared tc batch commands")
	}
}

// captureSnapshotForApply returns the qdisc snapshot for the given runner +
// opts, or an empty snapshot when capture is not applicable (strict mode on,
// no tc root touched) or fails (logged). The empty-snapshot fallback keeps
// the attack running even when snapshot is unavailable — the experiment
// always proceeds, only revert behaviour degrades.
func captureSnapshotForApply(runner CommandRunner, netNsID string, opts Opts) QdiscSnapshot {
	if strictRootQdisc {
		return QdiscSnapshot{}
	}
	p, ok := opts.(tcCommandProvider)
	if !ok {
		return QdiscSnapshot{}
	}
	ifs := p.tcRootQdiscInterfaces()
	if len(ifs) == 0 {
		return QdiscSnapshot{}
	}
	snap, err := captureSnapshot(runner, netNsID, ifs)
	if err != nil {
		log.Warn().Err(err).Str("netNs", netNsID).Msg("qdisc snapshot failed; revert will not restore prior state")
		return QdiscSnapshot{}
	}
	return snap
}

// logBeforeAfterRules emits the current ip/tc state at TRACE level around
// the batch execution. Either skipped or unfolded into the three protocol
// calls.
func logBeforeAfterRules(ctx context.Context, runner CommandRunner, ipV4, ipV6, tcCmds []string, when string) {
	if len(ipV4) > 0 {
		logCurrentIpRules(ctx, runner, familyV4, when)
	}
	if len(ipV6) > 0 {
		logCurrentIpRules(ctx, runner, familyV6, when)
	}
	if len(tcCmds) > 0 {
		logCurrentTcRules(ctx, runner, when)
	}
}

// runIptablesScripts loads the caller's iptables-restore scripts and pipes
// them through iptables-restore / ip6tables-restore. iptables errors are
// joined into *outErr (non-fatal for the attack lifecycle); only a
// scriptErr from the provider itself short-circuits the caller.
func runIptablesScripts(ctx context.Context, runner CommandRunner, opts Opts, mode mode, outErr *error) error {
	provider, ok := opts.(iptablesScriptProvider)
	if !ok {
		return nil
	}
	v4, v6, scriptErr := provider.iptablesScripts(mode)
	if scriptErr != nil {
		return scriptErr
	}
	logIptablesScripts(mode, v4, v6)
	if len(v4) > 0 {
		if _, restoreErr := runner.run(ctx, []string{"iptables-restore", "-w", "-n"}, v4); restoreErr != nil {
			log.Warn().Err(restoreErr).Str("mode", string(mode)).Msg("iptables-restore failed")
			*outErr = errors.Join(*outErr, restoreErr)
		}
	}
	if ipv6Supported() && len(v6) > 0 {
		if _, restoreErr := runner.run(ctx, []string{"ip6tables-restore", "-w", "-n"}, v6); restoreErr != nil {
			log.Warn().Err(restoreErr).Str("mode", string(mode)).Msg("ip6tables-restore failed")
			*outErr = errors.Join(*outErr, restoreErr)
		}
	}
	return nil
}

// logIptablesScripts emits the prepared iptables-restore scripts at DEBUG
// level. Separate from logPreparedCommands because the iptables scripts are
// multi-line strings rather than command slices.
func logIptablesScripts(mode mode, v4, v6 []string) {
	if !log.Debug().Enabled() {
		return
	}
	if len(v4) > 0 {
		log.Debug().Str("mode", string(mode)).Str("iptables_v4", strings.Join(v4, "\n")).Msg("prepared iptables-restore script (IPv4)")
	}
	if len(v6) > 0 {
		log.Debug().Str("mode", string(mode)).Str("iptables_v6", strings.Join(v6, "\n")).Msg("prepared ip6tables-restore script (IPv6)")
	}
}

// runBatchCommands executes the prepared ip/tc batch commands. Errors are
// joined into *outErr — the caller decides whether to escalate after the
// whole batch has run.
func runBatchCommands(ctx context.Context, runner CommandRunner, mode mode, ipV4, ipV6, tcCmds []string, outErr *error) {
	if len(ipV4) > 0 {
		if _, ipErr := executeIpCommands(ctx, runner, ipV4, "-family", string(familyV4)); ipErr != nil {
			*outErr = errors.Join(*outErr, filterBatchErrors(ipErr, mode, ipV4))
		}
	}
	if len(ipV6) > 0 {
		if _, ipErr := executeIpCommands(ctx, runner, ipV6, "-family", string(familyV6)); ipErr != nil {
			*outErr = errors.Join(*outErr, filterBatchErrors(ipErr, mode, ipV6))
		}
	}
	if len(tcCmds) > 0 {
		if _, tcErr := executeTcCommands(ctx, runner, tcCmds); tcErr != nil {
			*outErr = errors.Join(*outErr, filterBatchErrors(tcErr, mode, tcCmds))
		}
	}
}

// captureSnapshot opens the runner's netns and returns the qdisc snapshot.
// Wrapped in its own function so the netns-fd open/close is one place.
// Logs the rendered snapshot at INFO level so operators investigating a
// restore failure can see exactly what was captured.
func captureSnapshot(runner CommandRunner, netNsID string, interfaces []string) (QdiscSnapshot, error) {
	path := runner.netNsPath()
	f, err := openNetNs(path)
	if err != nil {
		return QdiscSnapshot{}, err
	}
	defer func() { _ = f.Close() }()
	snap, err := takeSnapshot(int(f.Fd()), netNsID, interfaces)
	if err != nil {
		return QdiscSnapshot{}, err
	}
	log.Info().
		Str("netNs", netNsID).
		Int("interfaces", len(snap.Interfaces)).
		Str("snapshot", renderSnapshot(snap)).
		Msg("captured qdisc snapshot")
	return snap, nil
}

// applyRestore opens the runner's netns, replays the saved snapshot, then
// re-snapshots the netns and compares the result against what we tried to
// restore. The post-restore snapshot is logged at INFO so the operator can
// see whether the kernel accepted the replay byte-for-byte. Any structural
// divergence (missing qdisc, extra qdisc, wrong parent) is logged at WARN.
func applyRestore(runner CommandRunner, snap QdiscSnapshot) error {
	path := runner.netNsPath()
	f, err := openNetNs(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	netNsFd := int(f.Fd())

	if rerr := restoreSnapshot(netNsFd, snap); rerr != nil {
		// Restore had errors; still try to render the current state so the
		// operator can see what got partially applied.
		if names := snapshotInterfaceNames(snap); len(names) > 0 {
			if post, perr := takeSnapshot(netNsFd, snap.NetNsID, names); perr == nil {
				log.Warn().
					Str("netNs", snap.NetNsID).
					Str("post_restore_state", renderSnapshot(post)).
					Msg("post-restore state after restore error")
			}
		}
		return rerr
	}

	// Restore returned no error — verify by re-snapshotting and diffing.
	names := snapshotInterfaceNames(snap)
	if len(names) == 0 {
		log.Info().Str("netNs", snap.NetNsID).Msg("qdisc restore completed (empty snapshot, nothing to verify)")
		return nil
	}
	post, perr := takeSnapshot(netNsFd, snap.NetNsID, names)
	if perr != nil {
		// Verification re-snapshot failed; restore itself succeeded so we
		// don't propagate this error. Operators get a partial signal.
		log.Warn().Err(perr).Str("netNs", snap.NetNsID).Msg("qdisc restore completed but post-restore verification could not read state")
		return nil
	}
	if diff := compareSnapshotsByHandle(snap, post); diff != "" {
		log.Warn().
			Str("netNs", snap.NetNsID).
			Str("expected", renderSnapshot(snap)).
			Str("actual", renderSnapshot(post)).
			Str("diff", diff).
			Msg("qdisc restore completed but post-restore state differs from snapshot")
	} else {
		log.Info().
			Str("netNs", snap.NetNsID).
			Int("interfaces", len(snap.Interfaces)).
			Str("restored_state", renderSnapshot(post)).
			Msg("qdisc restore verified: post-restore state matches snapshot")
	}
	return nil
}

// snapshotInterfaceNames returns the sorted set of interface names in the
// snapshot. Used to re-snapshot for post-restore verification.
func snapshotInterfaceNames(snap QdiscSnapshot) []string {
	out := make([]string, 0, len(snap.Interfaces))
	for name := range snap.Interfaces {
		out = append(out, name)
	}
	return out
}

func logCurrentIpRules(ctx context.Context, runner CommandRunner, family family, when string) {
	if !log.Trace().Enabled() {
		return
	}

	stdout, err := executeIpCommands(ctx, runner, []string{"rule show"}, "-family", string(family))
	if err != nil {
		log.Trace().Err(err).Msg("failed to get current ip rules")
		return
	} else {
		log.Trace().Str("family", string(family)).Str("when", when).Str("rules", stdout).Msg("current ip rules")
	}
}

func pushActiveNetfault(netNsId string, opts Opts) error {
	activeNetfaultLock.Lock()
	defer activeNetfaultLock.Unlock()

	for _, active := range activeNetfault[netNsId] {
		if opts.doesConflictWith(active) {
			activeContext := active.toExecutionContext()
			err := fmt.Sprintf("running multiple network attacks at the same time on the same network namespace is not supported. Already running attack started by %s (#%d) in targetExecution %s", activeContext.ExperimentKey, activeContext.ExperimentExecutionId, activeContext.TargetExecutionId)

			log.Warn().
				Str("active", active.String()).
				Str("new", opts.String()).
				Msg("running multiple network attacks at the same time on the same network namespace is not supported")
			return errors.New(err)
		}
	}

	activeNetfault[netNsId] = append(activeNetfault[netNsId], opts)
	return nil
}

func popActiveNetfault(id string, opts Opts) {
	activeNetfaultLock.Lock()
	defer activeNetfaultLock.Unlock()

	active, ok := activeNetfault[id]
	if !ok {
		return
	}
	for i, a := range active {
		if reflect.DeepEqual(opts, a) {
			activeNetfault[id] = append(active[:i], active[i+1:]...)
			return
		}
	}
}

var ipv6Supported = defaultIpv6Supported

func defaultIpv6Supported() bool {
	// execute the following command to check if ipv6 is disabled:
	// ip -family inet6 rule
	// if the command fails, we assume that ipv6 is disabled
	cmd := exec.Command("ip", "-family", "inet6", "rule")
	if err := cmd.Run(); err != nil {
		log.Trace().Err(err).Msg("ipv6 is disabled")
		return false
	}
	return true
}

func executeIpCommands(ctx context.Context, runner CommandRunner, cmds []string, extraArgs ...string) (string, error) {
	if len(cmds) == 0 {
		return "", nil
	}

	processArgs := append([]string{ipPath, "-force", "-batch", "-"}, extraArgs...)

	return runner.run(ctx, processArgs, cmds)
}

func logCurrentTcRules(ctx context.Context, runner CommandRunner, s string) {
	if !log.Trace().Enabled() {
		return
	}

	stdout, err := executeTcCommands(ctx, runner, []string{"qdisc show"})
	if err != nil {
		log.Trace().Err(err).Msg("failed to get current tc rules")
		return
	} else {
		log.Trace().Str("when", s).Str("rules", stdout).Msg("current tc qdisc")
	}

	stdout, err = executeTcCommands(ctx, runner, []string{"filter show"})
	if err != nil {
		log.Trace().Err(err).Msg("failed to get current tc rules")
		return
	} else {
		log.Trace().Str("when", s).Str("rules", stdout).Msg("current tc filter")
	}
}

func executeTcCommands(ctx context.Context, runner CommandRunner, cmds []string) (string, error) {
	if len(cmds) == 0 {
		return "", nil
	}

	return runner.run(ctx, []string{"tc", "-force", "-batch", "-"}, cmds)
}

// CondenseNetWithPortRange condenses a list of NetWithPortRange
// The way this algorithm works:
// 1. Sort the nwp list ascending by BaseIP and port
// 2. For each nwp in the list create a new nwp with the next neighbor if port-ranges are compatible
// 3. From the new list choose the nwp with the longest prefix length, remove all nwp witch are included in the chosen and add the chosen nwp to the result list
// 4. Repeat 3. until either the list is shorter than limit or no more compatible nwp are found
func CondenseNetWithPortRange(nwps []network.NetWithPortRange, limit int) []network.NetWithPortRange {
	if len(nwps) <= limit {
		return nwps
	}

	result := make([]network.NetWithPortRange, len(nwps))
	copy(result, nwps)
	slices.SortFunc(nwps, network.NetWithPortRange.Compare)

	var candidates []network.NetWithPortRange
	for i := 0; i < len(result)-1; i++ {
		if c := getNextMatchingCandidate(result, i); c != nil {
			candidates, _ = insertSorted(candidates, *c, comparePrefixLen)
		}
	}

	slices.SortFunc(candidates, comparePrefixLen)
	for {
		if len(result) <= limit || len(candidates) == 0 {
			return result
		}

		longestPrefix := candidates[0]
		candidates = candidates[1:]

		lenBefore := len(result)
		result = slices.DeleteFunc(result, func(nwp network.NetWithPortRange) bool {
			return longestPrefix.Contains(nwp)
		})

		//when it was an "old" candidate, and it did not actually remove anything, we can skip it
		if len(result) == lenBefore {
			continue
		}

		var i int
		result, i = insertSorted(result, longestPrefix, network.NetWithPortRange.Compare)

		//add new candidates resulting from the insterted nwp
		for j := max(i-1, 0); j <= min(i, len(result)-1); j++ {
			if c := getNextMatchingCandidate(result, j); c != nil {
				candidates, _ = insertSorted(candidates, *c, comparePrefixLen)
			}
		}
	}
}

func getNextMatchingCandidate(result []network.NetWithPortRange, i int) *network.NetWithPortRange {
	a := result[i]
	for j := i + 1; j < len(result); j++ {
		b := result[j]
		if a.PortRange == b.PortRange {
			if merged := a.Merge(b); !merged.Net.IP.IsUnspecified() {
				return &merged
			}
			return nil
		}
	}
	return nil
}

func insertSorted[S ~[]E, E any](x S, target E, cmp func(E, E) int) (S, int) {
	i, _ := slices.BinarySearchFunc(x, target, cmp)
	return slices.Insert(x, i, target), i
}

func comparePrefixLen(a, b network.NetWithPortRange) int {
	prefixLenA, _ := a.Net.Mask.Size()
	prefixLenB, _ := b.Net.Mask.Size()
	return prefixLenB - prefixLenA
}
