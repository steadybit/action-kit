// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package netfault

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

type iface struct {
	Index    uint       `json:"ifindex"`
	Name     string     `json:"ifname"`
	LinkType string     `json:"link_type"`
	Flags    []string   `json:"flags"`
	AddrInfo []addrInfo `json:"addr_info"`
}

type addrInfo struct {
	Family    string `json:"family"`
	Local     string `json:"local"`
	PrefixLen uint   `json:"prefixlen"`
	Scope     string `json:"scope"`
	Label     string `json:"label"`
	Broadcast string `json:"broadcast"`
}

type route struct {
	Dst      string   `json:"dst"`
	Gateway  string   `json:"gateway"`
	Dev      string   `json:"dev"`
	Flags    []string `json:"flags"`
	Protocol string   `json:"protocol"`
	Prefsrc  string   `json:"prefsrc"`
	Scope    string   `json:"scope"`
}

func (i *iface) hasFlag(f string) bool {
	for _, flag := range i.Flags {
		if flag == f {
			return true
		}
	}
	return false
}

func ListNonLoopbackInterfaceNames(ctx context.Context, r CommandRunner) ([]string, error) {
	ifcs, err := listInterfaces(ctx, r)
	if err != nil {
		log.Error().Err(err).Msg("failed to list interfaces")
		return nil, err
	}

	var ifcNames []string
	for _, ifc := range ifcs {
		if !ifc.hasFlag("LOOPBACK") {
			ifcNames = append(ifcNames, ifc.Name)
		}
	}
	return ifcNames, nil
}

func HasCiliumIpRoutes(ctx context.Context, r CommandRunner) (bool, error) {
	out, err := executeIpCommands(ctx, r, []string{"route list"}, "--json")
	if err != nil {
		return false, err
	}

	var rules []route
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

func listInterfaces(ctx context.Context, r CommandRunner) ([]iface, error) {
	out, err := executeIpCommands(ctx, r, []string{"address show up"}, "--json")
	if err != nil {
		return nil, err
	}

	var interfaces []iface
	if err = json.Unmarshal([]byte(out), &interfaces); err != nil {
		return nil, fmt.Errorf("failed to unmarshal interfaces: %w", err)
	}

	var validInterfaces []iface
	for _, i := range interfaces {
		if len(i.Name) > 0 {
			validInterfaces = append(validInterfaces, i)
		}
	}

	log.Trace().Interface("interfaces", interfaces).Msg("listed network interfaces")
	return validInterfaces, nil
}
