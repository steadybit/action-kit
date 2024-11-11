// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package heartbeat

import (
	"sync/atomic"
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
		t.Fatal("callback should not have been called")
	default:
	}
}

func TestHeartbeat_timeout_should_close_channel(t *testing.T) {
	i := atomic.Uint32{}
	w := make(chan interface{})
	ch := make(chan time.Time)
	hb := Notify(ch, 10*time.Millisecond, 20*time.Millisecond)
	defer hb.Stop()

	go func() {
		for {
			select {
			case <-w:
				return
			case <-ch:
				if i.Add(1) > 1 {
					t.Error("callback called multiple times")
				}
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)
	w <- nil
}
