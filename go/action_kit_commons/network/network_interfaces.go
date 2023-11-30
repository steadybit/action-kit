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
