// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package heartbeat

import (
	"github.com/rs/zerolog/log"
	"time"
)

type Monitor struct {
	pulse chan time.Time
}

func Notify(ch chan<- time.Time, interval, timeout time.Duration) *Monitor {
	pulse := make(chan time.Time)

	go func(pulse <-chan time.Time, signal chan<- time.Time) {
		last := time.Now()
		for {
			select {
			case ts, ok := <-pulse:
				if ok {
					last = ts
				} else {
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
	select {
	case h.pulse <- time.Now():
	default:
	}
}

func (h *Monitor) Stop() {
	close(h.pulse)
}
