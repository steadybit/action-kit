// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package utils

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func Test_readCpusAllowedCount(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		want    int
		wantErr assert.ErrorAssertionFunc
	}{
		{name: "parse error", status: "", want: -1, wantErr: assert.Error},
		{name: "parse error", status: "Cpus_allowed: invalid", want: -1, wantErr: assert.Error},
		{name: "four cpus", status: "Cpus_allowed:   f\n", want: 4, wantErr: assert.NoError},
		{name: "two cpus", status: "Cpus_allowed:   6", want: 2, wantErr: assert.NoError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := fakeStatus(t, tt.status)
			defer func() { _ = os.Remove(file) }()

			got, err := ReadCpusAllowedCount(file)
			if !tt.wantErr(t, err, fmt.Sprintf("readCpusAllowedCount(%v)", tt.status)) {
				return
			}
			assert.Equalf(t, tt.want, got, "readCpusAllowedCount(%v)", tt.status)
		})
	}
}

func fakeStatus(t *testing.T, c string) string {
	f, err := os.CreateTemp(t.TempDir(), "status")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	_, _ = f.WriteString(c)
	return f.Name()
}
