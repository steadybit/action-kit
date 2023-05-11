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
	if i := strings.Index(containerId, separator); i >= 0 {
		return containerId[i+len(separator):]
	}
	return containerId
}

func AddPrefix(containerId string, runtime Runtime) string {
	if !strings.Contains(containerId, separator) {
		return fmt.Sprintf("%s%s%s", runtime, separator, containerId)
	}
	return containerId
}
