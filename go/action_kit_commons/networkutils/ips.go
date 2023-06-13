package networkutils

import (
	"github.com/rs/zerolog/log"
	"net"
)

func GetOwnIPs() []net.IP {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get network interfaces")
		return nil
	}

	resultIP4s := make([]net.IP, 0)
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			log.Error().Err(err).Msg("Failed to get addresses")
			break
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if !ip.IsLoopback() && len(ip) > 0 && !ip.Equal(net.IPv4zero) {
				resultIP4s = append(resultIP4s, ip)
			}
		}
	}
	return resultIP4s
}
