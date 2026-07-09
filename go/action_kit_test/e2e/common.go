// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"fmt"
	"strings"
)

const (
	separator = "://"
)

func RemovePrefix(containerId string) string {
	if _, after, ok := strings.Cut(containerId, separator); ok {
		return after
	}
	return containerId
}

func AddPrefix(containerId string, runtime Runtime) string {
	if !strings.Contains(containerId, separator) {
		return fmt.Sprintf("%s%s%s", runtime, separator, containerId)
	}
	return containerId
}
