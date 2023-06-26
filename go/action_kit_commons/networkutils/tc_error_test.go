/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package networkutils

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestParseTcBatchError(t *testing.T) {
	assert.NoError(t, ParseTcBatchError(strings.NewReader("")))

	content := strings.NewReader(`Error: Exclusivity flag on, cannot modify.
Command failed -:1
RTNETLINK answers: File exists
Command failed -:2
Error: Exclusivity flag on, cannot modify.
Command failed -:161
`)
	err := ParseTcBatchError(content)

	assert.Equal(t, 3, len(err.(*TcBatchError).Errors))
	assert.Equal(t, content, err.Error())
	assert.Equal(t, "Error: Exclusivity flag on, cannot modify.", err.(*TcBatchError).Errors[0].Msg)
	assert.Equal(t, "RTNETLINK answers: File exists", err.(*TcBatchError).Errors[1].Msg)
	assert.Equal(t, "Error: Exclusivity flag on, cannot modify.", err.(*TcBatchError).Errors[2].Msg)
}

func TestFilterTcBatchErrors(t *testing.T) {
	assert.NoError(t, FilterTcBatchErrors(nil, ModeAdd, nil))

	errAdd := ParseTcBatchError(strings.NewReader(`Error: Exclusivity flag on, cannot modify.
Command failed -:1
RTNETLINK answers: File exists
Command failed -:2
`))

	assert.NoError(t, FilterTcBatchErrors(errAdd, ModeAdd, nil))
	assert.Equal(t, errAdd, FilterTcBatchErrors(errAdd, ModeDelete, nil))

	errDel := ParseTcBatchError(strings.NewReader(`Error: Invalid handle.
Command failed -:1
Error: Parent Qdisc doesn't exists.
Command failed -:2
`))

	assert.NoError(t, FilterTcBatchErrors(errDel, ModeDelete, nil))
	assert.Equal(t, errDel, FilterTcBatchErrors(errDel, ModeAdd, nil))
}
