// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

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
				assert.Equal(t, 3, len(err.(*BatchErrors).Errors))
				assert.Equal(t, exampleError, strings.TrimPrefix(err.Error(), "Command failed test -b -\n"))
				assert.Equal(t, "Error: Exclusivity flag on, cannot modify.", err.(*BatchErrors).Errors[0].Msg)
				assert.Equal(t, "RTNETLINK answers: File exists", err.(*BatchErrors).Errors[1].Msg)
				assert.Equal(t, "Error: Exclusivity flag on, cannot modify.", err.(*BatchErrors).Errors[2].Msg)
			},
		},
		{
			name: "should add kernel module hint",
			args: args{
				cmd: []string{"test", "-b -"},
				r:   strings.NewReader(exampleErrorTcKernelModuleMissing),
			},
			assert: func(t assert.TestingT, err error, message string) {
				assert.Equal(t, 5, len(err.(*BatchErrors).Errors))
				assert.Equal(t, "Kernel configuration error. Please check if the required kernel modules are loaded.", err.(*BatchErrors).Errors[0].Msg)
				assert.Equal(t, "This is expected, for example, when using minikube under Windows with WLS2 (https://github.com/microsoft/WSL/issues/6065).", err.(*BatchErrors).Errors[1].Msg)
				assert.Equal(t, "Error: Specified qdisc kind is unknown.", err.(*BatchErrors).Errors[2].Msg)
				assert.Equal(t, "Error: Failed to find specified qdisc.", err.(*BatchErrors).Errors[3].Msg)
				assert.Equal(t, "Error: Parent Qdisc doesn't exists.\nWe have an error talking to the kernel", err.(*BatchErrors).Errors[4].Msg)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assert(t, ParseBatchError(tt.args.cmd, tt.args.r), fmt.Sprintf("ParseBatchError(%v, %v)", tt.args.cmd, tt.args.r))
		})
	}
}

func TestFilterBatchErrors(t *testing.T) {
	assert.NoError(t, FilterBatchErrors(nil, ModeAdd, nil))

	errAdd := ParseBatchError([]string{"test", "-b -"}, strings.NewReader(`Error: Exclusivity flag on, cannot modify.
RTNETLINK answers: File exists
Command failed -:1
RTNETLINK answers: File exists
Command failed -:2
`))

	assert.NoError(t, FilterBatchErrors(errAdd, ModeAdd, nil))
	assert.Equal(t, errAdd, FilterBatchErrors(errAdd, ModeDelete, nil))

	errDel := ParseBatchError([]string{"test", "-b -"}, strings.NewReader(`Error: Invalid handle.
Command failed -:1
Error: Parent Qdisc doesn't exists.
Command failed -:2
`))

	assert.NoError(t, FilterBatchErrors(errDel, ModeDelete, nil))
	assert.Equal(t, errDel, FilterBatchErrors(errDel, ModeAdd, nil))
}
