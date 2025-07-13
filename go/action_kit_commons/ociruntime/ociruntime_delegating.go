// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package ociruntime

import (
	"context"
	"os/exec"
	"strings"
	"syscall"
)

type delegatingOciRuntime struct {
	runtimes       map[string]OciRuntime
	defaultRuntime OciRuntime
}

func NewDelegatingOciRuntime(defaultRuntime OciRuntime, runtimes map[string]OciRuntime) OciRuntime {
	return &delegatingOciRuntime{
		runtimes:       runtimes,
		defaultRuntime: defaultRuntime,
	}
}

func (r *delegatingOciRuntime) delegateForContainer(id string) OciRuntime {
	for prefix, runtime := range r.runtimes {
		if strings.HasPrefix(id, prefix) {
			return runtime
		}
	}

	return r.defaultRuntime
}

func (r *delegatingOciRuntime) State(ctx context.Context, id string) (*ContainerState, error) {
	return r.delegateForContainer(id).State(ctx, id)
}

func (r *delegatingOciRuntime) Create(ctx context.Context, image, id string) (ContainerBundle, error) {
	return r.delegateForContainer(id).Create(ctx, image, id)
}

func (r *delegatingOciRuntime) Run(ctx context.Context, container ContainerBundle, ioOpts IoOpts) error {
	return r.delegateForContainer(container.ContainerId()).Run(ctx, container, ioOpts)
}

func (r *delegatingOciRuntime) RunCommand(ctx context.Context, container ContainerBundle) (*exec.Cmd, error) {
	return r.delegateForContainer(container.ContainerId()).RunCommand(ctx, container)
}

func (r *delegatingOciRuntime) Delete(ctx context.Context, id string, force bool) error {
	return r.delegateForContainer(id).Delete(ctx, id, force)
}

func (r *delegatingOciRuntime) Kill(ctx context.Context, id string, signal syscall.Signal) error {
	return r.delegateForContainer(id).Kill(ctx, id, signal)

}

var _ OciRuntime = &delegatingOciRuntime{}
