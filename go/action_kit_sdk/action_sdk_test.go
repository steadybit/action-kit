package action_kit_sdk

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

// TestStopEvents_concurrent_access exercises markAsStopped (write) and getStopEvent (read)
// from many goroutines, mirroring the HTTP stop/status handlers, the heartbeat-timeout
// goroutine and the signal handler racing on the shared stopEvents slice. Run under -race.
func TestStopEvents_concurrent_access(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		id := uuid.New()
		wg.Add(2)
		go func() { defer wg.Done(); markAsStopped(id, "test") }()
		go func() { defer wg.Done(); _ = getStopEvent(id) }()
	}
	wg.Wait()
}

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
