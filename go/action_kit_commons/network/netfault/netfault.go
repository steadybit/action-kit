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
}

func Apply(ctx context.Context, runner CommandRunner, opts Opts) error {
	return generateAndRunCommands(ctx, runner, opts, modeAdd)
}

func Revert(ctx context.Context, runner CommandRunner, opts Opts) error {
	return generateAndRunCommands(ctx, runner, opts, modeDelete)
}

func generateAndRunCommands(ctx context.Context, runner CommandRunner, opts Opts, mode mode) error {
	ipCommandsV4, err := opts.ipCommands(familyV4, mode)
	if err != nil {
		return err
	}

	var ipCommandsV6 []string
	if ipv6Supported() {
		ipCommandsV6, err = opts.ipCommands(familyV6, mode)
		if err != nil {
			return err
		}
	}

	tcCommands, err := opts.tcCommands(mode)
	if err != nil {
		return err
	}

	if log.Debug().Enabled() {
		if len(ipCommandsV4) > 0 {
			log.Debug().Str("mode", string(mode)).Strs("ip_cmds_v4", ipCommandsV4).Msg("prepared ip batch commands (IPv4)")
		}
		if len(ipCommandsV6) > 0 {
			log.Debug().Str("mode", string(mode)).Strs("ip_cmds_v6", ipCommandsV6).Msg("prepared ip batch commands (IPv6)")
		}
		if len(tcCommands) > 0 {
			log.Debug().Str("mode", string(mode)).Strs("tc_cmds", tcCommands).Msg("prepared tc batch commands")
		}
	}

	netNsID := runner.id()
	runLock.LockKey(netNsID)
	defer func() { _ = runLock.UnlockKey(netNsID) }()

	if mode == modeAdd {
		if err := pushActiveNetfault(netNsID, opts); err != nil {
			return err
		}
	}

	if len(ipCommandsV4) > 0 {
		logCurrentIpRules(ctx, runner, familyV4, "before")
	}

	if len(ipCommandsV6) > 0 {
		logCurrentIpRules(ctx, runner, familyV6, "before")
	}

	if len(tcCommands) > 0 {
		logCurrentTcRules(ctx, runner, "before")
	}

	// If opts provide iptables scripts, execute them first
	if provider, ok := opts.(iptablesScriptProvider); ok {
		v4, v6, scriptErr := provider.iptablesScripts(mode)
		if scriptErr != nil {
			return scriptErr
		}
		if log.Debug().Enabled() {
			if len(v4) > 0 {
				log.Debug().Str("mode", string(mode)).Str("iptables_v4", strings.Join(v4, "\n")).Msg("prepared iptables-restore script (IPv4)")
			}
			if len(v6) > 0 {
				log.Debug().Str("mode", string(mode)).Str("iptables_v6", strings.Join(v6, "\n")).Msg("prepared ip6tables-restore script (IPv6)")
			}
		}
		if len(v4) > 0 {
			if _, restoreErr := runner.run(ctx, []string{"iptables-restore", "-w", "-n"}, v4); restoreErr != nil {
				err = errors.Join(err, restoreErr)
			}
		}
		if ipv6Supported() && len(v6) > 0 {
			if _, restoreErr := runner.run(ctx, []string{"ip6tables-restore", "-w", "-n"}, v6); restoreErr != nil {
				err = errors.Join(err, restoreErr)
			}
		}
	}

	if len(ipCommandsV4) > 0 {
		if _, ipErr := executeIpCommands(ctx, runner, ipCommandsV4, "-family", string(familyV4)); ipErr != nil {
			err = errors.Join(err, filterBatchErrors(ipErr, mode, ipCommandsV4))
		}
	}

	if len(ipCommandsV6) > 0 {
		if _, ipErr := executeIpCommands(ctx, runner, ipCommandsV6, "-family", string(familyV6)); ipErr != nil {
			err = errors.Join(err, filterBatchErrors(ipErr, mode, ipCommandsV6))
		}
	}

	if len(tcCommands) > 0 {
		if _, tcErr := executeTcCommands(ctx, runner, tcCommands); tcErr != nil {
			err = errors.Join(err, filterBatchErrors(tcErr, mode, tcCommands))
		}
	}

	if len(ipCommandsV4) > 0 {
		logCurrentIpRules(ctx, runner, familyV4, "after")
	}

	if len(ipCommandsV6) > 0 {
		logCurrentIpRules(ctx, runner, familyV6, "after")
	}

	if len(tcCommands) > 0 {
		logCurrentTcRules(ctx, runner, "after")
	}

	if mode == modeDelete {
		popActiveNetfault(netNsID, opts)
	}

	return err
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
