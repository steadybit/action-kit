// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package network

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/runc"
)

type Interface struct {
	Index    uint       `json:"ifindex"`
	Name     string     `json:"ifname"`
	LinkType string     `json:"link_type"`
	Flags    []string   `json:"flags"`
	AddrInfo []AddrInfo `json:"addr_info"`
}

type AddrInfo struct {
	Family    string `json:"family"`
	Local     string `json:"local"`
	PrefixLen uint   `json:"prefixlen"`
	Scope     string `json:"scope"`
	Label     string `json:"label"`
	Broadcast string `json:"broadcast"`
}

func (i *Interface) HasFlag(f string) bool {
	for _, flag := range i.Flags {
		if flag == f {
			return true
		}
	}
	return false
}

type ExtraMount struct {
	Source string `json:"source"`
	Path   string `json:"path"`
}

func ListInterfacesForAllNamespaces(ctx context.Context, r runc.Runc) ([]Interface, error) {
	nsList, err := runc.ListNamespaces(ctx, "net")
	if err != nil {
		return nil, err
	}

	var allIfcs []Interface
	for _, ns := range nsList {
		opts := SidecarOpts{TargetProcess: runc.LinuxProcessInfo{Namespaces: []runc.LinuxNamespace{ns}}}
		if ifcs, err := ListInterfaces(ctx, r, opts); err == nil {
			allIfcs = append(allIfcs, ifcs...)
		} else {
			return nil, err
		}
	}

	return allIfcs, nil
}

func ListNonLoopbackInterfaceNames(ctx context.Context, r runc.Runc, sidecar SidecarOpts) ([]string, error) {
	ifcs, err := ListInterfaces(ctx, r, sidecar)
	if err != nil {
		return nil, err
	}

	var ifcNames []string
	for _, ifc := range ifcs {
		if !ifc.HasFlag("LOOPBACK") {
			ifcNames = append(ifcNames, ifc.Name)
		}
	}
	return ifcNames, nil
}

func ListInterfaces(ctx context.Context, r runc.Runc, sidecar SidecarOpts) ([]Interface, error) {
	id := getNextContainerId("ip-addr", sidecar.IdSuffix)

	bundle, err := r.Create(ctx, "/", id)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := bundle.Remove(); err != nil {
			log.Warn().Str("id", id).Err(err).Msg("failed to remove bundle")
		}
	}()

	runc.RefreshNamespaces(ctx, sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)

	if err = bundle.EditSpec(
		runc.WithHostname(fmt.Sprintf("ip-link-show-%s", id)),
		runc.WithAnnotations(map[string]string{
			"com.steadybit.sidecar": "true",
		}),
		runc.WithNamespaces(runc.FilterNamespaces(sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)),
		runc.WithCapabilities("CAP_NET_ADMIN"),
		runc.WithProcessArgs("ip", "--json", "address", "show", "up")); err != nil {
		return nil, err
	}

	var outb, errb bytes.Buffer
	err = r.Run(ctx, bundle, runc.IoOpts{Stdout: &outb, Stderr: &errb})
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
		return nil, fmt.Errorf("failed to list interfaces: %w: %s", err, errb.String())
	}

	var interfaces []Interface
	err = json.Unmarshal(outb.Bytes(), &interfaces)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal interfaces: %w", err)
	}

	//the json output has empty objects for non-up interfaces, we filter those.
	var validInterfaces []Interface
	for _, iface := range interfaces {
		if len(iface.Name) > 0 {
			validInterfaces = append(validInterfaces, iface)
		}
	}

	log.Trace().Interface("interfaces", interfaces).Msg("listed network interfaces")
	return validInterfaces, nil
}
