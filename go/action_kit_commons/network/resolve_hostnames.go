// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build !windows

package network

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

type HostnameResolver struct {
	Dig DigRunner
}

var defaultHostnameResolver = &HostnameResolver{Dig: &CommandDigRunner{}}

func Resolve(ctx context.Context, hostnames ...string) ([]net.IP, error) {
	return defaultHostnameResolver.Resolve(ctx, hostnames...)
}

func (h *HostnameResolver) Resolve(ctx context.Context, hostnames ...string) ([]net.IP, error) {
	if len(hostnames) == 0 {
		return nil, nil
	}

	unresolved := make([]string, 0, len(hostnames))
	var invalid []string
	var sb strings.Builder
	for _, hostname := range hostnames {
		if len(strings.TrimSpace(hostname)) == 0 {
			invalid = append(invalid, hostname)
			continue
		}
		sb.WriteString(hostname)
		sb.WriteString(" A\n")
		sb.WriteString(hostname)
		sb.WriteString(" AAAA\n")
		unresolved = append(unresolved, hostname)
	}

	if len(invalid) > 0 {
		return nil, fmt.Errorf("could not resolve invalid hostnames: '%s'", strings.Join(invalid, "', '"))
	}

	outb, err := h.Dig.Run(ctx, []string{"-f-", fmt.Sprintf("+timeout=%d", dnsTimeout()), "+noall", "+nottlid", "+answer"}, strings.NewReader(sb.String()))
	log.Trace().Bytes("output", outb).Err(err).Msg("dig result")

	var resolved []net.IP
	var messages []string

	scanner := bufio.NewScanner(bytes.NewReader(outb))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, ";;") {
			messages = append(messages, strings.TrimSpace(strings.TrimPrefix(line, ";;")))
			continue
		} else if fields := strings.Fields(line); len(fields) >= 4 {
			domain := strings.TrimSuffix(fields[0], ".")
			resolved = append(resolved, net.ParseIP(fields[3]))
			unresolved = slices.DeleteFunc(unresolved, func(hostname string) bool {
				return strings.EqualFold(hostname, domain)
			})
		}
	}

	if err != nil {
		detail := ""
		if len(messages) > 0 {
			detail = fmt.Sprintf("\n%s", strings.Join(messages, "\n"))
		}
		return nil, fmt.Errorf("could not resolve hostnames: %w%s", err, detail)
	}

	if len(unresolved) > 0 {
		return nil, fmt.Errorf("could not resolve hostnames: '%s'", strings.Join(unresolved, "', '"))
	}
	return resolved, nil
}

func dnsTimeout() int {
	if s, _ := os.LookupEnv("STEADYBIT_EXTENSION_DIG_TIMEOUT"); s != "" {
		if timeout, err := strconv.Atoi(s); err == nil && timeout > 0 {
			return timeout
		}
		log.Warn().Msgf("Invalid DNS timeout value: %s, using default value of 5 seconds", s)
	}
	return 5
}
