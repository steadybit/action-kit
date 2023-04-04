/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package heartbeat

import (
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-kit/exthttp"
	"net/http"
	"time"
)

type Heartbeat struct {
	pulse   chan time.Time
	channel chan time.Time
}

func StartAndRegisterHandler() *Heartbeat {
	hb := Start(30*time.Second, 15*time.Second)
	hb.RegisterHandler()
	return hb
}

func Start(interval, timeout time.Duration) *Heartbeat {
	pulse := make(chan time.Time)
	signal := make(chan time.Time)

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
				if time.Since(last) > timeout {
					log.Debug().Msgf("no heartbeat received within %s (last: %s)", timeout, last.Format(time.RFC3339))
					signal <- time.Now()
				} else {
					log.Trace().Msg("missed heartbeat")
				}
			}
		}
	}(pulse, signal)

	return &Heartbeat{
		pulse:   pulse,
		channel: signal,
	}
}

func (h *Heartbeat) RegisterHandler() {
	exthttp.RegisterHttpHandler("/heartbeat", h.handler)
}

func (h *Heartbeat) handler(w http.ResponseWriter, _ *http.Request, _ []byte) {
	log.Trace().Msg("received heartbeat")
	h.pulse <- time.Now()
	w.WriteHeader(http.StatusOK)
}

func (h *Heartbeat) Stop() {
	close(h.pulse)
}
func (h *Heartbeat) Channel() chan time.Time {
	return h.channel
}
