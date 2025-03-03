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
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
)

const maxFirewallRules = 1000

type Shell = string

const (
	PS       Shell = "PowerShell" // regular powershell.
	PSInvoke Shell = "PSInvoke"   // powershell Invoke-Command
)

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

	winDivertCommands, err := opts.WinDivertCommands(mode)

	if err != nil {
		return err
	}

	if len(fwCommandsV4) > 0 || len(fwCommandsV6) > 0 {
		logCurrentFwRules(ctx, "before")
	}

	if len(qosCommands) > 0 {
		logCurrentQoSRules(ctx, "before")
	}

	if len(fwCommandsV4) > 0 {
		if _, fwErr := executeFwCommands(ctx, fwCommandsV4); fwErr != nil {
			err = errors.Join(err, FilterBatchErrors(fwErr, mode, fwCommandsV4))
		}
	}

	if len(fwCommandsV6) > 0 {
		if _, fwErr := executeFwCommands(ctx, fwCommandsV6); fwErr != nil {
			err = errors.Join(err, FilterBatchErrors(fwErr, mode, fwCommandsV6))
		}
	}

	if len(qosCommands) > 0 {
		if _, qosErr := executeQoSCommands(ctx, qosCommands); qosErr != nil {
			err = errors.Join(err, FilterBatchErrors(qosErr, mode, qosCommands))
		}
	}

	if len(winDivertCommands) > 0 {
		if _, wdErr := executeWinDivertCommands(ctx, winDivertCommands); wdErr != nil {
			err = errors.Join(err, FilterBatchErrors(wdErr, mode, winDivertCommands))
		}
	}

	if len(fwCommandsV4) > 0 || len(fwCommandsV6) > 0 {
		logCurrentFwRules(ctx, "after")
	}

	if len(qosCommands) > 0 {
		logCurrentQoSRules(ctx, "after")
	}

	if mode == ModeDelete {
		popActiveFw("windows", opts)
	}

	return err
}

func logCurrentFwRules(ctx context.Context, when string) {
	if !log.Trace().Enabled() {
		return
	}
	var outb, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, "powershell", "-Command", "Get-NetFirewallRule", "|", "Where-Object", "{ $_.DisplayName -like \"STEADYBIT*\" }")
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()

	if err != nil {
		log.Trace().Err(err).Msg("failed to get current firewall rules")
		return
	} else {
		log.Trace().Str("when", when).Str("rules", outb.String()).Msg("current fw rules")
	}
}

func logCurrentQoSRules(ctx context.Context, when string) {
	if !log.Trace().Enabled() {
		return
	}
	var outb, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, "powershell", "-Command", "Get-NetQosPolicy", "-PolicyStore", "ActiveStore", "|", "Where-Object", "{ $_.Name -like \"STEADYBIT*\" }")
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()

	if err != nil {
		log.Trace().Err(err).Msg("failed to get current firewall rules")
		return
	} else {
		log.Trace().Str("when", when).Str("rules", outb.String()).Msg("current fw rules")
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

func executeFwCommands(ctx context.Context, cmds []string) (string, error) {
	if len(cmds) == 0 {
		return "", nil
	}

	return executeInNetwork(ctx, cmds, PSInvoke)
}

func executeQoSCommands(ctx context.Context, cmds []string) (string, error) {
	if len(cmds) == 0 {
		return "", nil
	}

	return executeInNetwork(ctx, cmds, PSInvoke)
}

func executeWinDivertCommands(ctx context.Context, cmds []string) (string, error) {
	if len(cmds) == 0 {
		return "", nil
	}

	return executeInNetwork(ctx, cmds, PS)
}

func executeInNetwork(ctx context.Context, cmds []string, shell Shell) (string, error) {
	log.Info().Strs("cmds", cmds).Msg("running commands in network")

	var outb, errb bytes.Buffer
	var cmd *exec.Cmd
	if shell == PSInvoke {
		joinedCommands := "\"" + strings.Join(cmds, ";") + "\""
		cmd = exec.CommandContext(ctx, "powershell", "-Command", "Invoke-Expression", joinedCommands)
		cmd.Stdout = &outb
		cmd.Stderr = &errb

		err := cmd.Run()

		if err != nil {
			return "", fmt.Errorf("execution failed: %w, output: %s, error: %s", err, outb.String(), errb.String())
		}

		return outb.String(), err
	} else {
		go func() {
			cmd = exec.Command("powershell", "-Command", strings.Join(cmds, ";"))
			cmd.Stdout = &outb
			cmd.Stderr = &errb
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			cmd.Run()
		}()

		return "", nil
	}
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
