// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package netfault

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
	"strings"
	"testing"
)

func TestParseBatchError(t *testing.T) {
	type args struct {
		cmd []string
		r   io.Reader
	}

	exampleError := `Error: Exclusivity flag on, cannot modify.
Command failed -:1
RTNETLINK answers: File exists
Command failed -:2
Error: Exclusivity flag on, cannot modify.
Command failed -:161
`
	exampleErrorTcKernelModuleMissing := `Error: Specified qdisc kind is unknown.
Command failed -:1
Error: Failed to find specified qdisc.
Command failed -:2
Error: Parent Qdisc doesn't exists.
We have an error talking to the kernel
Command failed -:3
`

	tests := []struct {
		name   string
		args   args
		assert func(t assert.TestingT, err error, message string)
	}{
		{
			name: "no error",
			args: args{
				cmd: []string{"test", "-b -"},
				r:   strings.NewReader(""),
			},
			assert: func(t assert.TestingT, err error, message string) {
				assert.NoError(t, err, message)
			},
		},
		{
			name: "error",
			args: args{
				cmd: []string{"test", "-b -"},
				r:   strings.NewReader(exampleError),
			},
			assert: func(t assert.TestingT, err error, message string) {
				assert.Equal(t, 3, len(err.(*batchErrors).Errors))
				assert.Equal(t, exampleError, strings.TrimPrefix(err.Error(), "Command failed test -b -\n"))
				assert.Equal(t, "Error: Exclusivity flag on, cannot modify.", err.(*batchErrors).Errors[0].Msg)
				assert.Equal(t, "RTNETLINK answers: File exists", err.(*batchErrors).Errors[1].Msg)
				assert.Equal(t, "Error: Exclusivity flag on, cannot modify.", err.(*batchErrors).Errors[2].Msg)
			},
		},
		{
			name: "should add kernel module hint",
			args: args{
				cmd: []string{"test", "-b -"},
				r:   strings.NewReader(exampleErrorTcKernelModuleMissing),
			},
			assert: func(t assert.TestingT, err error, message string) {
				assert.Equal(t, 5, len(err.(*batchErrors).Errors))
				assert.Equal(t, "Kernel configuration error. Please check if the required kernel modules are loaded.", err.(*batchErrors).Errors[0].Msg)
				assert.Equal(t, "This is expected, for example, when using minikube under Windows with WLS2 (https://github.com/microsoft/WSL/issues/6065).", err.(*batchErrors).Errors[1].Msg)
				assert.Equal(t, "Error: Specified qdisc kind is unknown.", err.(*batchErrors).Errors[2].Msg)
				assert.Equal(t, "Error: Failed to find specified qdisc.", err.(*batchErrors).Errors[3].Msg)
				assert.Equal(t, "Error: Parent Qdisc doesn't exists.\nWe have an error talking to the kernel", err.(*batchErrors).Errors[4].Msg)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assert(t, parseBatchError(tt.args.cmd, tt.args.r), fmt.Sprintf("parseBatchError(%v, %v)", tt.args.cmd, tt.args.r))
		})
	}
}

func TestFilterBatchErrors(t *testing.T) {
	assert.NoError(t, filterBatchErrors(nil, modeAdd, nil))

	errAdd := parseBatchError([]string{"test", "-b -"}, strings.NewReader(`Error: Exclusivity flag on, cannot modify.
RTNETLINK answers: File exists
Command failed -:1
RTNETLINK answers: File exists
Command failed -:2
`))

	assert.NoError(t, filterBatchErrors(errAdd, modeAdd, nil))
	assert.Equal(t, errAdd, filterBatchErrors(errAdd, modeDelete, nil))

	errDel := parseBatchError([]string{"test", "-b -"}, strings.NewReader(`Error: Invalid handle.
Command failed -:1
Error: Parent Qdisc doesn't exists.
Command failed -:2
`))

	assert.NoError(t, filterBatchErrors(errDel, modeDelete, nil))
	assert.Equal(t, errDel, filterBatchErrors(errDel, modeAdd, nil))
}
