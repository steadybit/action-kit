/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

package network

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"net"
	"os/exec"
	"slices"
	"strings"
)

type DigRunner interface {
	Run(ctx context.Context, arg []string, stdin io.Reader) ([]byte, error)
}

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
		return nil, fmt.Errorf("could not resolve hostnames: '%s'", strings.Join(unresolved, "', '"))
	}

	outb, err := h.Dig.Run(ctx, []string{"-f-", "+timeout=4", "+noall", "+nottlid", "+answer"}, strings.NewReader(sb.String()))
	if err != nil {
		return nil, fmt.Errorf("could not resolve hostnames: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(outb))
	var resolved []net.IP
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 4 {
			domain := strings.TrimSuffix(fields[0], ".")
			resolved = append(resolved, net.ParseIP(fields[3]))
			unresolved = slices.DeleteFunc(unresolved, func(hostname string) bool {
				return hostname == domain
			})
		}
	}

	if len(unresolved) > 0 {
		return nil, fmt.Errorf("could not resolve hostnames: '%s'", strings.Join(unresolved, "', '"))
	}

	log.Trace().Interface("resolved", resolved).Strs("hostnames", hostnames).Msg("resolved resolved")
	return resolved, nil
}

type CommandDigRunner struct {
}

func (c *CommandDigRunner) Run(ctx context.Context, arg []string, stdin io.Reader) ([]byte, error) {
	var outb, errb bytes.Buffer

	cmd := exec.CommandContext(ctx, "dig", arg...)
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	cmd.Stdin = stdin

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("could not resolve hostnames: %w: %s", err, errb.String())
	}

	return outb.Bytes(), nil
}
