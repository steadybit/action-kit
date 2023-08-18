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

	result := make([]net.IP, 0)
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
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
				result = append(result, ip)
			}
		}
	}
	return result
}
