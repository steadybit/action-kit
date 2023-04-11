// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package heartbeat

import (
	"testing"
	"time"
)

func TestHeartbeat_should_timeout(t *testing.T) {
	ch := make(chan time.Time)
	hb := Notify(ch, 300*time.Millisecond, 150*time.Millisecond)
	defer hb.Stop()

	time.Sleep(150 * time.Millisecond)
	hb.RecordHeartbeat()
	time.Sleep(350 * time.Millisecond)

	select {
	case <-ch:
	default:
		t.Fatal("callback should have been called")
	}
}

func TestHeartbeat_should_not_timeout(t *testing.T) {
	ch := make(chan time.Time)
	hb := Notify(ch, 300*time.Millisecond, 150*time.Millisecond)
	defer hb.Stop()

	hb.RecordHeartbeat()
	time.Sleep(150 * time.Millisecond)
	hb.RecordHeartbeat()
	time.Sleep(150 * time.Millisecond)
	hb.RecordHeartbeat()
	time.Sleep(150 * time.Millisecond)

	select {
	case <-ch:
		t.Fatal("callback should have not been called")
	default:
	}
}
