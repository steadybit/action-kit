// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package main

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extlogging"
	"net/http"
)

func main() {
	extlogging.InitZeroLog()

	exthttp.RegisterHttpHandler("/actions", exthttp.GetterAsHandler(getAttackList))
	exthttp.RegisterHttpHandler("/actions/rollout-restart", exthttp.GetterAsHandler(getRolloutRestartDescription))
	exthttp.RegisterHttpHandler("/actions/rollout-restart/prepare", prepareRolloutRestart)
	exthttp.RegisterHttpHandler("/actions/rollout-restart/start", startRolloutRestart)
	exthttp.RegisterHttpHandler("/actions/rollout-restart/status", rolloutRestartStatus)
	exthttp.RegisterHttpHandler("/actions/rollout-restart/stop", stopRolloutRestart)

	port := 8083
	log.Info().Msgf("Starting go-kubectl-attack server on port %d. Get started via /actions", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Err(err).Msg("Failed to start server")
	}
}
