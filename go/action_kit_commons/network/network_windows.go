package network

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"slices"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
)

const maxFirewallRules = 1000

var (
	runLock = utils.NewHashedKeyMutex(10)

	activeFWLock   = sync.Mutex{}
	activeFirewall = map[string][]WinOpts{}
)

type ErrTooManyTcCommands struct {
	Count int
}

func (e *ErrTooManyTcCommands) Error() string {
	return fmt.Sprintf("too many 'NetFirewallRule' commands: %d", e.Count)
}

func Apply(ctx context.Context, opts WinOpts) error {
	return generateAndRunCommands(ctx, opts, ModeAdd)
}

func Revert(ctx context.Context, opts WinOpts) error {
	return generateAndRunCommands(ctx, opts, ModeDelete)
}

func generateAndRunCommands(ctx context.Context, opts WinOpts, mode Mode) error {
	fwCommandsV4, err := opts.FwCommands(FamilyV4, mode)
	if err != nil {
		return err
	}

	var fwCommandsV6 []string
	if ipv6Supported() {
		fwCommandsV6, err = opts.FwCommands(FamilyV6, mode)
		if err != nil {
			return err
		}
	}

	qosCommands, err := opts.QoSCommands(mode)
	if err != nil {
		return err
	}

	runLock.LockKey("windows")
	defer func() { _ = runLock.UnlockKey("windows") }()

	if mode == ModeAdd {
		if err := pushActiveFw(opts); err != nil {
			return err
		}
	}

	// if len(fwCommandsV4) > 0 {
	// 	logCurrentFwRules(ctx, FamilyV4, "before")
	// }

	// if len(fwCommandsV6) > 0 {
	// 	logCurrentFwRules(ctx, FamilyV6, "before")
	// }

	// if len(qosCommands) > 0 {
	// 	logCurrentQoSRules(ctx, "before")
	// }

	if len(fwCommandsV4) > 0 {
		if _, ipErr := executeFwCommands(ctx, fwCommandsV4); ipErr != nil {
			err = errors.Join(err, FilterBatchErrors(ipErr, mode, fwCommandsV4))
		}
	}

	if len(fwCommandsV6) > 0 {
		if _, ipErr := executeFwCommands(ctx, fwCommandsV6); ipErr != nil {
			err = errors.Join(err, FilterBatchErrors(ipErr, mode, fwCommandsV6))
		}
	}

	if len(qosCommands) > 0 {
		if _, tcErr := executeQoSCommands(ctx, qosCommands); tcErr != nil {
			err = errors.Join(err, FilterBatchErrors(tcErr, mode, qosCommands))
		}
	}

	// if len(fwCommandsV4) > 0 {
	// 	logCurrentFwRules(ctx, FamilyV4, "after")
	// }

	// if len(fwCommandsV6) > 0 {
	// 	logCurrentFwRules(ctx, FamilyV6, "after")
	// }

	// if len(qosCommands) > 0 {
	// 	logCurrentQoSRules(ctx, "after")
	// }

	if mode == ModeDelete {
		popActiveFw("windows", opts)
	}

	return err
}

func logCurrentFwRules(ctx context.Context, family Family, when string) {
	if !log.Trace().Enabled() {
		return
	}

	stdout, err := executeFwCommands(ctx, []string{"rule show"}, "-family", string(family))
	if err != nil {
		log.Trace().Err(err).Msg("failed to get current ip rules")
		return
	} else {
		log.Trace().Str("family", string(family)).Str("when", when).Str("rules", stdout).Msg("current ip rules")
	}
}

func pushActiveFw(opts WinOpts) error {
	activeFWLock.Lock()
	defer activeFWLock.Unlock()

	for _, active := range activeFirewall["windows"] {
		if !equals(opts, active) {
			return errors.New("running multiple network attacks at the same time on the same network namespace is not supported")
		}
	}

	activeFirewall["windows"] = append(activeFirewall["windows"], opts)
	return nil
}

func popActiveFw(id string, opts WinOpts) {
	activeFWLock.Lock()
	defer activeFWLock.Unlock()

	active, ok := activeFirewall[id]
	if !ok {
		return
	}
	for i, a := range active {
		if equals(opts, a) {
			activeFirewall[id] = append(active[:i], active[i+1:]...)
			return
		}
	}
}

func equals(opts WinOpts, active WinOpts) bool {
	return opts.String() == active.String()
}

var ipv6Supported = defaultIpv6Supported

func defaultIpv6Supported() bool {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Err(err)
		return false
	}

	ipv6Supported := false
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if ok && ipNet.IP.To4() == nil {
				ipv6Supported = true
			}
		}
	}

	return ipv6Supported
}

var executeFwCommands = executeIpCommandsImpl

func executeIpCommandsImpl(ctx context.Context, cmds []string, extraArgs ...string) (string, error) {
	if len(cmds) == 0 {
		return "", nil
	}

	// processArgs := append([]string{"ip", "-force", "-batch", "-"}, extraArgs...)

	return executeInNetwork(ctx, extraArgs, cmds)
}

func logCurrentQoSRules(ctx context.Context, s string) {
	if !log.Trace().Enabled() {
		return
	}

	stdout, err := executeQoSCommands(ctx, []string{"qdisc show"})
	if err != nil {
		log.Trace().Err(err).Msg("failed to get current tc rules")
		return
	} else {
		log.Trace().Str("when", s).Str("rules", stdout).Msg("current tc qdisc")
	}

	stdout, err = executeQoSCommands(ctx, []string{"filter show"})
	if err != nil {
		log.Trace().Err(err).Msg("failed to get current tc rules")
		return
	} else {
		log.Trace().Str("when", s).Str("rules", stdout).Msg("current tc filter")
	}
}

func executeQoSCommands(ctx context.Context, cmds []string) (string, error) {
	if len(cmds) == 0 {
		return "", nil
	}

	return executeInNetwork(ctx, []string{}, cmds)
}

func executeInNetwork(ctx context.Context, processArgs []string, cmds []string) (string, error) {
	log.Info().Strs("cmds", cmds).Strs("processArgs", processArgs).Msg("running commands in network namespace using ip netns")

	joinedCommands := "\"" + strings.Join(cmds, ";") + "\""
	var outb, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, "powershell", "-Command", "Invoke-Expression", joinedCommands)
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()

	if err != nil {
		if parsed := ParseBatchError(processArgs, bytes.NewReader(errb.Bytes())); parsed != nil {
			return "", parsed
		}
		return "", fmt.Errorf("Invoke-Expression failed: %w, output: %s, error: %s", err, outb.String(), errb.String())
	}
	return outb.String(), err
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
			if merged := a.merge(b); !merged.Net.IP.IsUnspecified() {
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
