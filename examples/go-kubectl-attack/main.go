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

	exthttp.RegisterHttpHandler("/attacks", exthttp.GetterAsHandler(getAttackList))
	exthttp.RegisterHttpHandler("/attacks/rollout-restart", exthttp.GetterAsHandler(getRolloutRestartDescription))
	exthttp.RegisterHttpHandler("/attacks/rollout-restart/prepare", prepareRolloutRestart)
	exthttp.RegisterHttpHandler("/attacks/rollout-restart/start", startRolloutRestart)
	exthttp.RegisterHttpHandler("/attacks/rollout-restart/status", rolloutRestartStatus)
	exthttp.RegisterHttpHandler("/attacks/rollout-restart/stop", stopRolloutRestart)

	port := 8083
	log.Info().Msgf("Starting go-kubectl-attack server on port %d. Get started via /attacks", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Err(err).Msg("Failed to start server")
	}
}
