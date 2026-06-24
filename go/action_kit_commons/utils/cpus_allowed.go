// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package utils

import (
	"bufio"
	"fmt"
	"math/bits"
	"os"
	"strconv"
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
		if value, ok := strings.CutPrefix(l, "Cpus_allowed:"); ok {
			return countCpusAllowed(value)
		}
	}

	return -1, fmt.Errorf("failed to read Cpus_allowed")
}

// countCpusAllowed counts the set bits of the Cpus_allowed mask. The kernel
// prints the mask as a comma-separated list of 32-bit hex words for hosts with
// more than 32 CPUs, e.g. "ffffffff,ffffffff,ffffffff,ffffffff" for 128 CPUs.
func countCpusAllowed(value string) (int, error) {
	count := 0
	for _, word := range strings.Split(strings.TrimSpace(value), ",") {
		bitmask, err := strconv.ParseUint(strings.TrimSpace(word), 16, 32)
		if err != nil {
			return -1, err
		}
		count += bits.OnesCount64(bitmask)
	}
	return count, nil
}
