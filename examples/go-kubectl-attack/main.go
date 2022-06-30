package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

func loggingMiddleware(next func(w http.ResponseWriter, r *http.Request, body []byte)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, bodyReadErr := ioutil.ReadAll(r.Body)
		if bodyReadErr != nil {
			http.Error(w, bodyReadErr.Error(), http.StatusBadRequest)
			return
		}

		if len(body) > 0 {
			InfoLogger.Printf("%s %s with body %s", r.Method, r.URL, body)
		} else {
			InfoLogger.Printf("%s %s", r.Method, r.URL)
		}

		next(w, r, body)
	})
}

func main() {
	http.Handle("/attacks", loggingMiddleware(getAttackList))
	http.Handle("/attacks/rollout-restart", loggingMiddleware(getRolloutRestartDescription))
	http.Handle("/attacks/rollout-restart/prepare", loggingMiddleware(prepareRolloutRestart))
	http.Handle("/attacks/rollout-restart/start", loggingMiddleware(startRolloutRestart))
	http.Handle("/attacks/rollout-restart/state", loggingMiddleware(rolloutRestartState))
	http.Handle("/attacks/rollout-restart/stop", loggingMiddleware(stopRolloutRestart))

	port := 8083
	InfoLogger.Printf("Starting kubectl attack server on port %d. Get started via /attacks\n", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
