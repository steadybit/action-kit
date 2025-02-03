//go:build windows
// +build windows

package network

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"
)

type DigRunner interface {
	Run(ctx context.Context, arg []string, stdin io.Reader) ([]byte, error)
}

type HostnameResolver struct {
}

type HostnameInput struct {
	Records  []string
	Hostname string
}

type HostnameOutput struct {
	IPAddresses []net.IP
}

var defaultHostnameResolver = &HostnameResolver{}

func Resolve(ctx context.Context, hostnames ...string) ([]net.IP, error) {
	return defaultHostnameResolver.Resolve(ctx, hostnames...)
}

func (h *HostnameResolver) Resolve(ctx context.Context, hostnames ...string) ([]net.IP, error) {
	if len(hostnames) == 0 {
		return nil, nil
	}

	unresolved := make([]string, 0, len(hostnames))
	resolved := make([]net.IP, 0)
	var toResolve []HostnameInput
	var invalid []string

	for _, hostname := range hostnames {
		if len(strings.TrimSpace(hostname)) == 0 {
			invalid = append(invalid, hostname)
			continue
		}
		toResolve = append(toResolve, HostnameInput{
			Hostname: hostname,
			Records:  []string{"A", "AAAA"},
		})
		unresolved = append(unresolved, hostname)
	}

	if len(invalid) > 0 {
		return nil, fmt.Errorf("could not resolve hostnames: '%s'", strings.Join(invalid, "', '"))
	}

	for _, hostnameInput := range toResolve {
		output, err := hostnameInput.Resolve(ctx)

		if err != nil || len(output.IPAddresses) == 0 {
			return nil, fmt.Errorf("could not resolve hostnames: %w", err)
		}

		unresolved = slices.DeleteFunc(unresolved, func(hostname string) bool {
			return hostname == hostnameInput.Hostname
		})

		resolved = append(resolved, output.IPAddresses...)
	}

	if len(unresolved) > 0 {
		return nil, fmt.Errorf("could not resolve hostnames: '%s'", strings.Join(unresolved, "', '"))
	}

	log.Trace().Interface("resolved", resolved).Strs("hostnames", hostnames).Msg("resolved resolved")
	return resolved, nil
}

func (i *HostnameInput) Resolve(ctx context.Context) (*HostnameOutput, error) {
	var outb, errb bytes.Buffer
	hostnameOutput := HostnameOutput{
		IPAddresses: make([]net.IP, 0),
	}

	for _, record := range i.Records {
		cmd := exec.CommandContext(ctx, "powershell", "-command", "(", "Resolve-DnsName", "-Name", i.Hostname, "-Type", record, ").IPAddress")
		cmd.Stdout = &outb
		cmd.Stderr = &errb

		err := cmd.Run()
		if err != nil {
			return nil, fmt.Errorf("could not resolve hostnames: %w: %s", err, errb.String())
		}

		scanner := bufio.NewScanner(&outb)
		for scanner.Scan() {
			ipStr := strings.TrimSpace(scanner.Text())
			ip := net.ParseIP(ipStr)
			if ip != nil {
				hostnameOutput.IPAddresses = append(hostnameOutput.IPAddresses, ip)
			}
		}

		outb.Reset()
	}

	return &hostnameOutput, nil
}
