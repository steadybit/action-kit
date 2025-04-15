// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetOwnIPs(t *testing.T) {
	ips := GetOwnIPs()
	assert.NotEmpty(t, ips)
}
