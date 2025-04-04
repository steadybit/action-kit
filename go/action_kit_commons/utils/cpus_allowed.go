// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package utils

import (
	"bufio"
	"fmt"
	"math/bits"
	"os"
	"strings"
)

func ReadCpusAllowedCount(fname string) (int, error) {
	file, err := os.Open(fname)
	if err != nil {
		return -1, err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		l := scanner.Text()
		if strings.HasPrefix(l, "Cpus_allowed:") {
			var bitmask uint = 0
			if _, err := fmt.Sscanf(l, "Cpus_allowed: %x", &bitmask); err == nil {
				return bits.OnesCount(bitmask), nil
			} else {
				return -1, err
			}
		}
	}

	return -1, fmt.Errorf("failed to read Cpus_allowed")
}
