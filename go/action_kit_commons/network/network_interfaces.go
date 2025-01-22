// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"github.com/rs/zerolog/log"
	"net"
)

func GetOwnNetworkInterfaces() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get network interfaces")
		return []string{}
	}
	resultNICs := make([]string, 0)
	for _, i := range ifaces {
		resultNICs = append(resultNICs, i.Name)
	}
	return resultNICs
}

func GetLoopbackNetworkInterfaces() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get network interfaces")
		return []string{}
	}

	var ifcNames []string
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagLoopback != 0 {
			ifcNames = append(ifcNames, ifc.Name)
		}
	}
	return ifcNames
}

func GetNonLoopbackNetworkInterfaces() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get network interfaces")
		return []string{}
	}

	var ifcNames []string
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagLoopback == 0 {
			ifcNames = append(ifcNames, ifc.Name)
		}
	}
	return ifcNames
}

func GetNetworkInterfacesByName(names []string) []net.Interface {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var matchingInterfaces []net.Interface
	for _, iface := range ifaces {
		for i, name := range names {
			if iface.Name == name {
				matchingInterfaces = append(matchingInterfaces, iface)
				names = append(names[:i], names[i+1:]...)
				break
			}
		}
	}
	if len(names) != 0 {
		log.Info().Interface("interface", names).Msg("Requested network interfaces missing")
	}
	return matchingInterfaces
}

func GetNetworkInterfaceIndexesByName(names []string) []int {
	var indexes []int
	for _, iface := range GetNetworkInterfacesByName(names) {
		indexes = append(indexes, iface.Index)
	}
	return indexes
}
