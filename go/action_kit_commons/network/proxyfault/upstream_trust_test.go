// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package proxyfault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartArgs_upstreamInsecureSkipVerify(t *testing.T) {
	o := sampleOpts(t)
	assert.NotContains(t, strings.Join(o.startArgs(), " "), "--upstream-insecure-skip-verify")

	o.UpstreamInsecureSkipVerify = true
	assert.Contains(t, strings.Join(o.startArgs(), " "), "--upstream-insecure-skip-verify")
	// Revert only removes interception rules; it never forwards anything.
	assert.NotContains(t, strings.Join(o.revertArgs(), " "), "--upstream")
}

// The custom CA an extension trusts (SSL_CERT_DIR / SSL_CERT_FILE, the
// exttls convention) is usually a mounted volume, which the sidecar's overlay
// of the extension's filesystem leaves out. Each existing path is therefore
// bind-mounted into the sidecar, read-only, at the same place.
func TestCustomCAMounts(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(t.TempDir(), "company-ca.pem")
	require.NoError(t, os.WriteFile(file, []byte("certificate"), 0o600))

	t.Setenv("SSL_CERT_DIR", dir+":/nonexistent/certs")
	t.Setenv("SSL_CERT_FILE", file)
	mounts := customCAMounts()
	require.Len(t, mounts, 2, "one mount per existing path; the missing one is skipped")
	for _, m := range mounts {
		assert.Contains(t, []string{dir, file}, m.Destination)
		assert.Equal(t, m.Destination, m.Source)
		assert.Equal(t, "bind", m.Type)
		assert.Contains(t, m.Options, "ro")
	}

	t.Setenv("SSL_CERT_DIR", "")
	t.Setenv("SSL_CERT_FILE", "")
	assert.Empty(t, customCAMounts())
}
