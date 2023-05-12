// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"bytes"
	"testing"
	"time"
)

// R is passed to each run of a flaky test run, manages state and accumulates log statements.
type R struct {
	// The number of current attempt.
	Attempt int

	failed bool
	log    *bytes.Buffer
}

func Retry(t *testing.T, maxAttempts int, sleep time.Duration, f func(r *R)) bool {
	t.Helper()

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		r := &R{Attempt: attempt, log: &bytes.Buffer{}}

		f(r)

		if !r.failed {
			return true
		}

		if attempt == maxAttempts {
			t.Fatalf("failed after %d attempts: %s", attempt, r.log.String())
		}

		time.Sleep(sleep)
	}
	return false
}
