package networkutils

import (
  "github.com/stretchr/testify/assert"
  "testing"
)

func TestGetOwnIPs(t *testing.T) {
  iPs := GetOwnIPs()
  assert.NotEmpty(t, iPs)
  assert.True(t, len(iPs) > 0)
}
