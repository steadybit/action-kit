// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

//go:build !windows

package dnsresolve

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
)

type digCommand struct {
}

func NewDig() Resolver {
	return &digCommand{}
}

func (d *digCommand) Resolve(ctx context.Context, hostnames ...string) ([]net.IP, error) {
	return resolve(ctx, d, hostnames...)
}

func (d *digCommand) run(ctx context.Context, arg []string, stdin io.Reader) ([]byte, error) {
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
