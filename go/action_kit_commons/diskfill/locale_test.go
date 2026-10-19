// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH
//go:build !windows

package diskfill

import (
	"os/exec"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_withCLocale(t *testing.T) {
	t.Setenv("LC_ALL", "de_DE.UTF-8")

	out, err := withCLocale(exec.Command("sh", "-c", `printf %s "$LC_ALL"`)).Output()

	require.NoError(t, err)
	assert.Equal(t, "C", string(out))
}

func Test_withCLocaleEnv(t *testing.T) {
	spec := &specs.Spec{Process: &specs.Process{Env: []string{"PATH=/bin", "LC_ALL=de_DE.UTF-8", "LANG=de_DE.UTF-8"}}}

	withCLocaleEnv(spec)

	assert.Equal(t, []string{"PATH=/bin", "LANG=de_DE.UTF-8", "LC_ALL=C"}, spec.Process.Env)
}
