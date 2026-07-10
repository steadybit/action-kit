// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package action_kit_sdk

import (
	"testing"

	"github.com/steadybit/extension-kit/exthttp"
	"github.com/stretchr/testify/assert"
)

func TestRegisterActionBumpsRevision(t *testing.T) {
	ClearRegisteredActions()
	t.Cleanup(ClearRegisteredActions)

	before := exthttp.Revision()
	RegisterAction(NewExampleAction(make(chan Call, 10)))
	assert.NotEqual(t, before, exthttp.Revision(), "RegisterAction must bump the index revision")
}

func TestClearRegisteredActionsBumpsRevision(t *testing.T) {
	before := exthttp.Revision()
	ClearRegisteredActions()
	assert.NotEqual(t, before, exthttp.Revision(), "ClearRegisteredActions must bump the index revision")
}
