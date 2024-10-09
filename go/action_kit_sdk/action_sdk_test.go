package action_kit_sdk

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

// This test reproduced an issue in which new heartbeats
// would not be processed anymore and led to a stop of the experiment.
func TestHeartbeat_should_not_timeout(t *testing.T) {
	stop := make(chan interface{})
	id, err := uuid.NewUUID()
	assert.NoError(t, err)

	monitorHeartbeatWithCallback(id, 1*time.Second, 4*time.Second, func() {
		stop <- nil
	})

	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(1 * time.Second):
				recordHeartbeat(id)
			}
		}
	}()

	select {
	case _, ok := <-stop:
		if ok {
			assert.Fail(t, "heartbeat close called")
		}
	case <-time.After(10 * time.Second):
		close(stop)
	}
}
