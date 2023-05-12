// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var (
	count = 0
)

func TestRetry(t *testing.T) {
	type args struct {
		t           *testing.T
		maxAttempts int
		sleep       time.Duration
		f           func(r *R)
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "TestRetry",
			args: args{
				t:           t,
				maxAttempts: 3,
				sleep:       1 * time.Millisecond,
				f: func(r *R) {
					r.failed = false
				},
			},
			want: true,
		}, {
			name: "TestRetry failed",
			args: args{
				t:           t,
				maxAttempts: 3,
				sleep:       1 * time.Millisecond,
				f: func(r *R) {
					count++
					r.failed = true
					if count == 2 {
						r.failed = false
					}
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, Retry(tt.args.t, tt.args.maxAttempts, tt.args.sleep, tt.args.f), "Retry(%v, %v, %v, %v)", tt.args.t, tt.args.maxAttempts, tt.args.sleep, tt.args.f)
		})
	}
}
