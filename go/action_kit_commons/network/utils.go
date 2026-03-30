// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package network

import (
	"github.com/rs/zerolog/log"
)

func ComputeExcludesForOwnIpAndPorts(port, healthPort uint16) []NetWithPortRange {
	ownIps := GetOwnIPs()
	nets := IpsToNets(ownIps)

	log.Debug().Msgf("Adding own ip %s to exclude list (Ports %d and %d)", ownIps, port, healthPort)

	var exclHealth, exclPort []NetWithPortRange
	rPort := PortRange{From: port, To: port}
	if healthPort > 0 && healthPort != port {
		rHealth := PortRange{From: healthPort, To: healthPort}
		if rPort.IsNeighbor(rHealth) {
			rPort = rPort.Merge(rHealth)
		} else {
			exclHealth = NewNetWithPortRanges(nets, rHealth)
			for i := range exclHealth {
				exclHealth[i].Comment = "ext. health port"
			}
		}
	}

	exclPort = NewNetWithPortRanges(nets, rPort)
	for i := range exclPort {
		exclPort[i].Comment = "ext. port"
	}

	return append(exclPort, exclHealth...)
}
