// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package network

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/runc"
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"os"
	"runtime/trace"
	"strconv"
	"strings"
	"time"
)

var (
	runLock  = utils.NewHashedKeyMutex(10)
	activeTc = map[string][]Opts{}
)

type SidecarOpts struct {
	TargetProcess runc.LinuxProcessInfo
	IdSuffix      string
	ImagePath     string
}

func Apply(ctx context.Context, r runc.Runc, sidecar SidecarOpts, opts Opts) error {
	return generateAndRunCommands(ctx, r, sidecar, opts, ModeAdd)
}

func Revert(ctx context.Context, r runc.Runc, sidecar SidecarOpts, opts Opts) error {
	return generateAndRunCommands(ctx, r, sidecar, opts, ModeDelete)
}

func generateAndRunCommands(ctx context.Context, r runc.Runc, sidecar SidecarOpts, opts Opts, mode Mode) error {
	defer trace.StartRegion(ctx, "network.generateAndRunCommands").End()
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
		if ipErr := executeIpCommands(ctx, r, sidecar, FamilyV4, ipCommandsV4); ipErr != nil {
			err = errors.Join(err, FilterBatchErrors(ipErr, mode, ipCommandsV4))
		}
	}

	if len(ipCommandsV6) > 0 {
		if ipErr := executeIpCommands(ctx, r, sidecar, FamilyV6, ipCommandsV6); ipErr != nil {
			err = errors.Join(err, FilterBatchErrors(ipErr, mode, ipCommandsV6))
		}
	}

	if len(tcCommands) > 0 {
		if tcErr := executeTcCommands(ctx, r, sidecar, tcCommands); tcErr != nil {
			err = errors.Join(err, FilterBatchErrors(tcErr, mode, tcCommands))
		}
	}

	if mode == ModeDelete {
		popActiveTc(netNsID, opts)
	}

	return err
}

func pushActiveTc(netNsId string, opts Opts) error {
	for _, active := range activeTc[netNsId] {
		if !equals(opts, active) {
			return errors.New("running multiple tc configs at the same time on the same namespace is not supported")
		}
	}

	activeTc[netNsId] = append(activeTc[netNsId], opts)
	return nil
}

func popActiveTc(id string, opts Opts) {
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
	if content, err := os.ReadFile("/sys/module/ipv6/parameters/disable"); err == nil {
		disabled := strings.TrimSpace(string(content)) == "1"
		log.Trace().Bool("disabled", disabled).Msg("read ipv6 module parameters")
		return !disabled
	} else {
		log.Warn().Err(err).Msg("Failed to read /sys/module/ipv6/parameters/disable. Assuming ipv6 is disabled.")
		return false
	}
}

func executeIpCommands(ctx context.Context, r runc.Runc, sidecar SidecarOpts, family Family, cmds []string) error {
	defer trace.StartRegion(ctx, "network.executeIpCommands").End()
	if len(cmds) == 0 {
		return nil
	}

	id := getNextContainerId("ip", sidecar.IdSuffix)
	bundle, err := r.Create(ctx, sidecar.ImagePath, id)
	if err != nil {
		return err
	}
	defer func() {
		if err := bundle.Remove(); err != nil {
			log.Warn().Str("id", id).Err(err).Msg("failed to remove bundle")
		}
	}()

	runc.RefreshNamespaces(ctx, sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)

	processArgs := []string{"ip", "-family", string(family), "-force", "-batch", "-"}
	if err = bundle.EditSpec(
		ctx,
		runc.WithHostname(fmt.Sprintf("ip-%s", id)),
		runc.WithAnnotations(map[string]string{"com.steadybit.sidecar": "true"}),
		runc.WithNamespaces(runc.FilterNamespaces(sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)),
		runc.WithCapabilities("CAP_NET_ADMIN"),
		runc.WithProcessArgs(processArgs...),
	); err != nil {
		return err
	}

	log.Debug().Strs("cmds", cmds).Str("family", string(family)).Msg("running ip commands")
	var outb bytes.Buffer
	err = r.Run(ctx, bundle, runc.IoOpts{
		Stdin:  ToReader(cmds),
		Stdout: &outb,
		Stderr: &outb,
	})
	defer func() {
		if err := r.Delete(context.Background(), id, true); err != nil {
			log.Warn().Str("id", id).Err(err).Msg("failed to delete container")
		}
	}()
	if err != nil {
		if parsed := ParseBatchError(processArgs, bytes.NewReader(outb.Bytes())); parsed != nil {
			return parsed
		}
		return fmt.Errorf("%s ip failed: %w, output: %s", id, err, outb.String())
	}
	return nil
}

func executeTcCommands(ctx context.Context, r runc.Runc, sidecar SidecarOpts, cmds []string) error {
	defer trace.StartRegion(ctx, "network.executeTcCommands").End()
	if len(cmds) == 0 {
		return nil
	}

	id := getNextContainerId("tc", sidecar.IdSuffix)
	bundle, err := r.Create(ctx, sidecar.ImagePath, id)
	if err != nil {
		return err
	}
	defer func() {
		if err := bundle.Remove(); err != nil {
			log.Warn().Str("id", id).Err(err).Msg("failed to remove bundle")
		}
	}()

	runc.RefreshNamespaces(ctx, sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)

	processArgs := []string{"tc", "-force", "-batch", "-"}
	if err = bundle.EditSpec(
		ctx,
		runc.WithHostname(fmt.Sprintf("tc-%s", id)),
		runc.WithAnnotations(map[string]string{"com.steadybit.sidecar": "true"}),
		runc.WithNamespaces(runc.FilterNamespaces(sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)),
		runc.WithCapabilities("CAP_NET_ADMIN"),
		runc.WithProcessArgs(processArgs...),
	); err != nil {
		return err
	}

	log.Debug().Strs("cmds", cmds).Msg("running tc commands")
	var outb bytes.Buffer
	err = r.Run(ctx, bundle, runc.IoOpts{
		Stdin:  ToReader(cmds),
		Stdout: &outb,
		Stderr: &outb,
	})
	defer func() {
		if err := r.Delete(context.Background(), id, true); err != nil {
			log.Warn().Str("id", id).Err(err).Msg("failed to delete container")
		}
	}()
	if err != nil {
		if parsed := ParseBatchError(processArgs, bytes.NewReader(outb.Bytes())); parsed != nil {
			return parsed
		}
		return fmt.Errorf("%s tc failed: %w, output: %s", id, err, outb.String())
	}
	return nil
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

func getNextContainerId(tool, suffix string) string {
	return fmt.Sprintf("sb-%s-%d-%s", tool, time.Now().UnixMilli(), suffix)
}
