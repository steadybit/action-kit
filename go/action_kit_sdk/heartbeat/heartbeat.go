// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package heartbeat

import (
	"github.com/rs/zerolog"
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
	hb := Start(15*time.Second, 60*time.Second)
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
					log.Debug().
						Str("timeout", timeout.String()).
						Str("last", last.Format(time.RFC3339)).
						Msg("no heartbeat received")
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
	http.Handle("/heartbeat", exthttp.PanicRecovery(exthttp.LogRequestWithLevel(h.handler, zerolog.DebugLevel)))
}

func (h *Heartbeat) handler(w http.ResponseWriter, _ *http.Request, _ []byte) {
	log.Trace().Msg("received heartbeat")
	select {
	case h.pulse <- time.Now():
	default:
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Heartbeat) Stop() {
	close(h.pulse)
}
func (h *Heartbeat) Channel() chan time.Time {
	return h.channel
}
