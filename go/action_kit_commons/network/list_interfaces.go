// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package network

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/runc"
	"runtime/trace"
)

type Interface struct {
	Index    uint     `json:"ifindex"`
	Name     string   `json:"ifname"`
	LinkType string   `json:"link_type"`
	Flags    []string `json:"flags"`
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

func ListInterfaces(ctx context.Context, r runc.Runc, sidecar SidecarOpts) ([]Interface, error) {
	defer trace.StartRegion(ctx, "network.ListInterfaces").End()
	id := getNextContainerId("ip-link", sidecar.IdSuffix)

	bundle, err := r.Create(ctx, sidecar.ImagePath, id)
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
		ctx,
		runc.WithHostname(fmt.Sprintf("ip-link-show-%s", id)),
		runc.WithAnnotations(map[string]string{
			"com.steadybit.sidecar": "true",
		}),
		runc.WithNamespaces(runc.FilterNamespaces(sidecar.TargetProcess.Namespaces, specs.NetworkNamespace)),
		runc.WithCapabilities("CAP_NET_ADMIN"),
		runc.WithProcessArgs("ip", "-json", "link", "show"),
	); err != nil {
		return nil, err
	}

	var outb, errb bytes.Buffer
	err = r.Run(ctx, bundle, runc.IoOpts{Stdout: &outb, Stderr: &errb})
	defer func() {
		if err := r.Delete(context.Background(), id, true); err != nil {
			log.Warn().Str("id", id).Err(err).Msg("failed to delete container")
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

	log.Trace().Interface("interfaces", interfaces).Msg("listed network interfaces")
	return interfaces, nil
}
