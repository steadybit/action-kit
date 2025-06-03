// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

//go:build darwin

package runc

import "context"

func executeReadlinkInProc(ctx context.Context, nsPaths ...string) ([]string, error) {
	return executeReadlinkUsingExec(ctx, nsPaths...)
}
