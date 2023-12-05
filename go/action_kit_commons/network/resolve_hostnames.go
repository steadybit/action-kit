/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
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
	"strings"
)

type DigRunner interface {
	Run(ctx context.Context, arg []string, stdin io.Reader) ([]byte, error)
}

type HostnameResolver struct {
	Dig DigRunner
}

var defaultHostnameResolver = &HostnameResolver{Dig: &CommandDigRunner{}}

func Resolve(ctx context.Context, ipOrHostnames ...string) ([]net.IP, error) {
	return defaultHostnameResolver.Resolve(ctx, ipOrHostnames...)
}

func (h *HostnameResolver) Resolve(ctx context.Context, ipOrHostnames ...string) ([]net.IP, error) {
	hostnames, ips := classify(ipOrHostnames)
	if len(hostnames) == 0 {
		return ips, nil
	}

	var sb strings.Builder
	for _, hostname := range hostnames {
		sb.WriteString(hostname)
		sb.WriteString(" A\n")
		sb.WriteString(hostname)
		sb.WriteString(" AAAA\n")
	}

	outb, err := h.Dig.Run(ctx, []string{"-f-", "+timeout=4", "+noall", "+nottlid", "+answer"}, strings.NewReader(sb.String()))
	if err != nil {
		return nil, fmt.Errorf("could not resolve hostnames: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(outb))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			domain := strings.TrimSuffix(fields[0], ".")
			ips = append(ips, net.ParseIP(fields[3]))
			for i, hostname := range hostnames {
				if hostname == domain {
					hostnames = append(hostnames[:i], hostnames[i+1:]...)
					break
				}
			}
		}
	}

	if len(hostnames) > 0 {
		return ips, fmt.Errorf("could not resolve hostnames: %s", strings.Join(hostnames, ", "))
	}

	log.Trace().Interface("ips", ips).Strs("ipOrHostnames", ipOrHostnames).Msg("resolved ips")
	return ips, nil
}

func classify(ipOrHostnames []string) (unresolved []string, resolved []net.IP) {
	for _, ipOrHostname := range ipOrHostnames {
		if len(ipOrHostname) == 0 {
			continue
		}
		if ip := net.ParseIP(strings.TrimPrefix(strings.TrimSuffix(ipOrHostname, "]"), "[")); ip == nil {
			unresolved = append(unresolved, ipOrHostname)
		} else {
			resolved = append(resolved, ip)
		}
	}
	return unresolved, resolved
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
