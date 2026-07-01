package action_kit_sdk

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestMonitorHeartbeat_restart_does_not_leak_goroutines verifies that repeatedly starting a
// heartbeat monitor for the same execution id (e.g. a retried Start) replaces and stops the
// previous monitor rather than leaking its goroutines. Each monitor spins two goroutines;
// without the Swap-and-Stop they would accumulate.
func TestMonitorHeartbeat_restart_does_not_leak_goroutines(t *testing.T) {
	id := uuid.New()
	base := runtime.NumGoroutine()
	for i := 0; i < 50; i++ {
		monitorHeartbeatWithCallback(id, time.Hour, time.Hour, func() {})
	}
	stopMonitorHeartbeat(id)
	assert.Eventually(t, func() bool {
		return runtime.NumGoroutine() <= base+4
	}, 2*time.Second, 20*time.Millisecond, "restarting the monitor must not leak goroutines")
}

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
