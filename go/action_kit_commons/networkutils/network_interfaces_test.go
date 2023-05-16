package networkutils

import (
  "github.com/stretchr/testify/assert"
  "testing"
)

func TestGetOwnNetworkInterfaces(t *testing.T) {
  networkInterfaces := GetOwnNetworkInterfaces()
  assert.NotEmpty(t, networkInterfaces)
  assert.True(t, len(networkInterfaces) > 0)
}
