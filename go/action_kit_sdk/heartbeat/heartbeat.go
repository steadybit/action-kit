// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package heartbeat

import (
	"github.com/rs/zerolog/log"
	"sync"
	"time"
)

type Monitor struct {
	mu     sync.Mutex
	pulse  chan time.Time
	closed bool
}

func Notify(ch chan<- time.Time, interval, timeout time.Duration) *Monitor {
	pulse := make(chan time.Time, 10)

	go func(pulse <-chan time.Time, signal chan<- time.Time) {
		last := time.Now()
		log.Debug().
			Dur("interval", interval).
			Dur("timeout", timeout).
			Time("last", last).
			Msg("starting heartbeat")
		for {
			select {
			case ts, ok := <-pulse:
				if ok {
					log.Trace().
						Dur("interval", interval).
						Dur("timeout", timeout).
						Time("current", ts).
						Time("last", last).
						Msg("received heartbeat")
					last = ts
				} else {
					log.Debug().
						Dur("interval", interval).
						Dur("timeout", timeout).
						Time("last", last).
						Msg("heartbeat stopped")
					close(signal)
					return
				}
			case <-time.After(interval):
				log.Debug().
					Dur("interval", interval).
					Dur("timeout", timeout).
					Time("last", last).
					Msg("missing timeout")
				if time.Since(last) > timeout {
					log.Warn().
						Dur("interval", interval).
						Dur("timeout", timeout).
						Time("last", last).
						Msg("no heartbeat received")
					signal <- time.Now()
					close(signal)
					return
				} else {
					log.Trace().Msg("missed heartbeat")
				}
			}
		}
	}(pulse, ch)

	return &Monitor{
		pulse: pulse,
	}
}

func (h *Monitor) RecordHeartbeat() {
	log.Trace().Msg("received heartbeat")
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	// Non-blocking send: once Stop has closed the channel we must not send (that panics),
	// and if the buffer is full the reader only needs to see recent activity — so dropping
	// a beat is fine and must never block the caller (an HTTP status handler goroutine).
	select {
	case h.pulse <- time.Now():
	default:
	}
}

// Stop is idempotent: concurrent or repeated Stop calls (e.g. the HTTP stop handler and
// the heartbeat-timeout goroutine both stopping the same execution) close the channel once.
func (h *Monitor) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	log.Debug().Msg("closing heartbeat channel")
	h.closed = true
	close(h.pulse)
}
