// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package heartbeat

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestHeartbeat_should_timeout(t *testing.T) {
	hb := Start(300*time.Millisecond, 150*time.Millisecond)
	defer hb.Stop()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	time.Sleep(150 * time.Millisecond)
	hb.handler(rr, req, nil)
	time.Sleep(350 * time.Millisecond)

	select {
	case <-hb.channel:
	default:
		t.Fatal("callback should have been called")
	}
}

func TestHeartbeat_should_not_timeout(t *testing.T) {
	hb := Start(300*time.Millisecond, 150*time.Millisecond)
	defer hb.Stop()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	hb.handler(rr, req, nil)
	time.Sleep(150 * time.Millisecond)
	hb.handler(rr, req, nil)
	time.Sleep(150 * time.Millisecond)
	hb.handler(rr, req, nil)
	time.Sleep(150 * time.Millisecond)

	select {
	case <-hb.channel:
		t.Fatal("callback should have not been called")
	default:
	}
}
