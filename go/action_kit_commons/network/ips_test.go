package network

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetOwnIPs(t *testing.T) {
	ips := GetOwnIPs()
	assert.NotEmpty(t, ips)
}
