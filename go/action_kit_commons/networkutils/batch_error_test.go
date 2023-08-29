/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package networkutils

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestParseBatchError(t *testing.T) {
	assert.NoError(t, ParseBatchError([]string{"test", "-b -"}, strings.NewReader("")))

	content := `Error: Exclusivity flag on, cannot modify.
Command failed -:1
RTNETLINK answers: File exists
Command failed -:2
Error: Exclusivity flag on, cannot modify.
Command failed -:161
`
	err := ParseBatchError([]string{"test", "-b -"}, strings.NewReader(content))

	assert.Equal(t, 3, len(err.(*BatchErrors).Errors))
	assert.Equal(t, content, strings.TrimPrefix(err.Error(), "Command failed test -b -\n"))
	assert.Equal(t, "Error: Exclusivity flag on, cannot modify.", err.(*BatchErrors).Errors[0].Msg)
	assert.Equal(t, "RTNETLINK answers: File exists", err.(*BatchErrors).Errors[1].Msg)
	assert.Equal(t, "Error: Exclusivity flag on, cannot modify.", err.(*BatchErrors).Errors[2].Msg)
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
