// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package network

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/runc"
	"strings"
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

type Route struct {
	Dst      string   `json:"dst"`
	Gateway  string   `json:"gateway"`
	Dev      string   `json:"dev"`
	Flags    []string `json:"flags"`
	Protocol string   `json:"protocol"`
	Prefsrc  string   `json:"prefsrc"`
	Scope    string   `json:"scope"`
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

func HasCiliumIpRoutes(ctx context.Context, r runc.Runc, sidecar SidecarOpts) (bool, error) {
	out, err := executeIpCommands(ctx, r, sidecar, []string{"route list"}, "--json")
	if err != nil {
		return false, err
	}

	var rules []Route
	if err = json.Unmarshal([]byte(out), &rules); err != nil {
		return false, fmt.Errorf("failed to unmarshal rules: %w", err)
	}

	log.Trace().Interface("rules", rules).Msg("listed routes")

	for _, rule := range rules {
		if strings.HasPrefix(rule.Dev, "cilium_") {
			return true, nil
		}
	}
	return false, nil
}

func ListInterfaces(ctx context.Context, r runc.Runc, sidecar SidecarOpts) ([]Interface, error) {
	out, err := executeIpCommands(ctx, r, sidecar, []string{"address show up"}, "--json")
	if err != nil {
		return nil, err
	}

	var interfaces []Interface
	if err = json.Unmarshal([]byte(out), &interfaces); err != nil {
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
