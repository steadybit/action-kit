// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package network

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/runc"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
)

const maxTcCommands = 2048

var (
	ipPath = utils.LocateExecutable("ip", "STEADYBIT_EXTENSION_IP_PATH")

	runLock = utils.NewHashedKeyMutex(10)

	activeTCLock = sync.Mutex{}
	activeTc     = map[string][]Opts{}
)

type SidecarOpts struct {
	TargetProcess runc.LinuxProcessInfo
	IdSuffix      string
	ExecutionId   uuid.UUID
}

type ErrTooManyTcCommands struct {
	Count int
}

func (e *ErrTooManyTcCommands) Error() string {
	return fmt.Sprintf("too many tc commands: %d", e.Count)
}

func Apply(ctx context.Context, r runc.Runc, sidecar SidecarOpts, opts Opts) error {
	return generateAndRunCommands(ctx, r, sidecar, opts, ModeAdd)
}

func Revert(ctx context.Context, r runc.Runc, sidecar SidecarOpts, opts Opts) error {
	return generateAndRunCommands(ctx, r, sidecar, opts, ModeDelete)
}

func generateAndRunCommands(ctx context.Context, r runc.Runc, sidecar SidecarOpts, opts Opts, mode Mode) error {
	ipCommandsV4, err := opts.IpCommands(FamilyV4, mode)
	if err != nil {
		return err
	}

	var ipCommandsV6 []string
	if ipv6Supported() {
		ipCommandsV6, err = opts.IpCommands(FamilyV6, mode)
		if err != nil {
			return err
		}
	}

	tcCommands, err := opts.TcCommands(mode)
	if err != nil {
		return err
	}

	netNsID := getNetworkNsIdentifier(sidecar.TargetProcess.Namespaces)
	runLock.LockKey(netNsID)
	defer func() { _ = runLock.UnlockKey(netNsID) }()

	if mode == ModeAdd {
		if err := pushActiveTc(netNsID, opts); err != nil {
			return err
		}
	}

	if len(ipCommandsV4) > 0 {
		logCurrentIpRules(ctx, r, sidecar, FamilyV4, "before")
	}

	if len(ipCommandsV6) > 0 {
		logCurrentIpRules(ctx, r, sidecar, FamilyV6, "before")
	}

	if len(tcCommands) > 0 {
		logCurrentTcRules(ctx, r, sidecar, "before")
	}

	if len(ipCommandsV4) > 0 {
		if _, ipErr := executeIpCommands(ctx, r, sidecar, ipCommandsV4, "-family", string(FamilyV4)); ipErr != nil {
			err = errors.Join(err, FilterBatchErrors(ipErr, mode, ipCommandsV4))
		}
	}

	if len(ipCommandsV6) > 0 {
		if _, ipErr := executeIpCommands(ctx, r, sidecar, ipCommandsV6, "-family", string(FamilyV6)); ipErr != nil {
			err = errors.Join(err, FilterBatchErrors(ipErr, mode, ipCommandsV6))
		}
	}

	if len(tcCommands) > 0 {
		if _, tcErr := executeTcCommands(ctx, r, sidecar, tcCommands); tcErr != nil {
			err = errors.Join(err, FilterBatchErrors(tcErr, mode, tcCommands))
		}
	}

	if len(ipCommandsV4) > 0 {
		logCurrentIpRules(ctx, r, sidecar, FamilyV4, "after")
	}

	if len(ipCommandsV6) > 0 {
		logCurrentIpRules(ctx, r, sidecar, FamilyV6, "after")
	}

	if len(tcCommands) > 0 {
		logCurrentTcRules(ctx, r, sidecar, "after")
	}

	if mode == ModeDelete {
		popActiveTc(netNsID, opts)
	}

	return err
}

func logCurrentIpRules(ctx context.Context, r runc.Runc, sidecar SidecarOpts, family Family, when string) {
	if !log.Trace().Enabled() {
		return
	}

	stdout, err := executeIpCommands(ctx, r, sidecar, []string{"rule show"}, "-family", string(family))
	if err != nil {
		log.Trace().Err(err).Msg("failed to get current ip rules")
		return
	} else {
		log.Trace().Str("family", string(family)).Str("when", when).Str("rules", stdout).Msg("current ip rules")
	}
}

func pushActiveTc(netNsId string, opts Opts) error {
	activeTCLock.Lock()
	defer activeTCLock.Unlock()

	for _, active := range activeTc[netNsId] {
		if !equals(opts, active) {
			return errors.New("running multiple network attacks at the same time on the same network namespace is not supported")
		}
	}

	activeTc[netNsId] = append(activeTc[netNsId], opts)
	return nil
}

func popActiveTc(id string, opts Opts) {
	activeTCLock.Lock()
	defer activeTCLock.Unlock()

	active, ok := activeTc[id]
	if !ok {
		return
	}
	for i, a := range active {
		if equals(opts, a) {
			activeTc[id] = append(active[:i], active[i+1:]...)
			return
		}
	}
}

func equals(opts Opts, active Opts) bool {
	return opts.String() == active.String()
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

var executeIpCommands = executeIpCommandsImpl

func executeIpCommandsImpl(ctx context.Context, r runc.Runc, sidecar SidecarOpts, cmds []string, extraArgs ...string) (string, error) {
	if len(cmds) == 0 {
		return "", nil
	}

	processArgs := append([]string{ipPath, "-force", "-batch", "-"}, extraArgs...)

	return executeInNetworkNamespace(ctx, r, sidecar, processArgs, cmds)
}

func logCurrentTcRules(ctx context.Context, r runc.Runc, sidecar SidecarOpts, s string) {
	if !log.Trace().Enabled() {
		return
	}

	stdout, err := executeTcCommands(ctx, r, sidecar, []string{"qdisc show"})
	if err != nil {
		log.Trace().Err(err).Msg("failed to get current tc rules")
		return
	} else {
		log.Trace().Str("when", s).Str("rules", stdout).Msg("current tc qdisc")
	}

	stdout, err = executeTcCommands(ctx, r, sidecar, []string{"filter show"})
	if err != nil {
		log.Trace().Err(err).Msg("failed to get current tc rules")
		return
	} else {
		log.Trace().Str("when", s).Str("rules", stdout).Msg("current tc filter")
	}
}

func executeTcCommands(ctx context.Context, r runc.Runc, sidecar SidecarOpts, cmds []string) (string, error) {
	if len(cmds) == 0 {
		return "", nil
	}

	return executeInNetworkNamespace(ctx, r, sidecar, []string{"tc", "-force", "-batch", "-"}, cmds)
}

func executeInNetworkNamespace(ctx context.Context, r runc.Runc, sidecar SidecarOpts, processArgs []string, cmds []string) (string, error) {
	runc.RefreshNamespaces(ctx, sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)

	if runc.HasNamedNetworkNamespace(sidecar.TargetProcess.Namespaces...) {
		ns := ""
		for _, n := range sidecar.TargetProcess.Namespaces {
			if n.Type == specs.NetworkNamespace {
				ns = n.Path
				break
			}
		}

		return executeInNamedNetworkUsingIpNetNs(ctx, ns, processArgs, cmds)
	} else {
		return executeInNetworkNamespaceUsingRunc(ctx, r, sidecar, processArgs, cmds)
	}
}

func executeInNamedNetworkUsingIpNetNs(ctx context.Context, netns string, processArgs []string, cmds []string) (string, error) {
	log.Info().Strs("cmds", cmds).Strs("processArgs", processArgs).Msg("running commands in network namespace using ip netns")

	ipArgs := append([]string{"netns", "exec", netns}, processArgs...)
	var outb, errb bytes.Buffer
	cmd := runc.RootCommandContext(ctx, ipPath, ipArgs...)
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	cmd.Stdin = ToReader(cmds)
	err := cmd.Run()

	if err != nil {
		if parsed := ParseBatchError(processArgs, bytes.NewReader(errb.Bytes())); parsed != nil {
			return "", parsed
		}
		return "", fmt.Errorf("netns exec failed: %w, output: %s, error: %s", err, outb.String(), errb.String())
	}
	return outb.String(), err
}

func executeInNetworkNamespaceUsingRunc(ctx context.Context, r runc.Runc, sidecar SidecarOpts, processArgs []string, cmds []string) (string, error) {
	log.Trace().Strs("cmds", cmds).Strs("processArgs", processArgs).Msg("running commands in network namespace using runc")

	id := getNextContainerId(sidecar.ExecutionId, path.Base(processArgs[0]), sidecar.IdSuffix)
	bundle, err := r.Create(ctx, "/", id)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := bundle.Remove(); err != nil {
			log.Warn().Str("id", id).Err(err).Msg("failed to remove bundle")
		}
	}()

	if err = bundle.EditSpec(
		runc.WithHostname(id),
		runc.WithAnnotations(map[string]string{"com.steadybit.sidecar": "true"}),
		runc.WithNamespaces(runc.FilterNamespaces(sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)),
		runc.WithCapabilities("CAP_NET_ADMIN"),
		runc.WithCopyEnviron(),
		runc.WithProcessArgs(processArgs...),
	); err != nil {
		return "", err
	}

	var outb, errb bytes.Buffer
	err = r.Run(ctx, bundle, runc.IoOpts{
		Stdin:  ToReader(cmds),
		Stdout: &outb,
		Stderr: &errb,
	})
	defer func() {
		if err := r.Delete(context.Background(), id, true); err != nil {
			level := zerolog.WarnLevel
			if errors.Is(err, runc.ErrContainerNotFound) {
				level = zerolog.DebugLevel
			}
			log.WithLevel(level).Str("id", id).Err(err).Msg("failed to delete container")
		}
	}()

	if err != nil {
		if parsed := ParseBatchError(processArgs, bytes.NewReader(errb.Bytes())); parsed != nil {
			return "", parsed
		}
		return "", fmt.Errorf("%s failed: %w, output: %s, error: %s", id, err, outb.String(), errb.String())
	}
	return outb.String(), err
}

func getNetworkNsIdentifier(namespaces []runc.LinuxNamespace) string {
	for _, ns := range namespaces {
		if ns.Type == specs.NetworkNamespace {
			if ns.Inode != 0 {
				return strconv.FormatUint(ns.Inode, 10)
			} else {
				return ns.Path
			}
		}
	}
	return ""
}

func getNextContainerId(executionId uuid.UUID, tool, suffix string) string {
	return fmt.Sprintf("sb-%s-%d-%s-%s", tool, time.Now().UnixMilli(), utils.ShortenUUID(executionId), suffix)
}

// CondenseNetWithPortRange condenses a list of NetWithPortRange
// The way this algorithm works:
// 1. Sort the nwp list ascending by BaseIP and port
// 2. For each nwp in the list create a new nwp with the next neighbor if port-ranges are compatible
// 3. From the new list choose the nwp with the longest prefix length, remove all nwp witch are included in the chosen and add the chosen nwp to the result list
// 4. Repeat 3. until either the list is shorter than limit or no more compatible nwp are found
func CondenseNetWithPortRange(nwps []NetWithPortRange, limit int) []NetWithPortRange {
	if len(nwps) <= limit {
		return nwps
	}

	result := make([]NetWithPortRange, len(nwps))
	copy(result, nwps)
	slices.SortFunc(nwps, NetWithPortRange.Compare)

	var candidates []NetWithPortRange
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
		result = slices.DeleteFunc(result, func(nwp NetWithPortRange) bool {
			return longestPrefix.Contains(nwp)
		})

		//when it was an "old" candidate, and it did not actually remove anything, we can skip it
		if len(result) == lenBefore {
			continue
		}

		var i int
		result, i = insertSorted(result, longestPrefix, NetWithPortRange.Compare)

		//add new candidates resulting from the insterted nwp
		for j := max(i-1, 0); j <= min(i, len(result)-1); j++ {
			if c := getNextMatchingCandidate(result, j); c != nil {
				candidates, _ = insertSorted(candidates, *c, comparePrefixLen)
			}
		}
	}
}

func getNextMatchingCandidate(result []NetWithPortRange, i int) *NetWithPortRange {
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

func comparePrefixLen(a, b NetWithPortRange) int {
	prefixLenA, _ := a.Net.Mask.Size()
	prefixLenB, _ := b.Net.Mask.Size()
	return prefixLenB - prefixLenA
}
