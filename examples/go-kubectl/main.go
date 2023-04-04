/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package main

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extlogging"
	"net/http"
)

func main() {
	extlogging.InitZeroLog()

	action_kit_sdk.RegisterAction(NewRolloutRestartAction())
	exthttp.RegisterHttpHandler("/actions", exthttp.GetterAsHandler(func() action_kit_api.ActionList {
		return action_kit_api.ActionList{
			Actions: action_kit_sdk.RegisteredActionsEndpoints(),
		}
	}))

	stop := action_kit_sdk.Start()
	defer stop()

	port := 8083
	log.Info().Msgf("Starting go-kubectl server on port %d. Get started via /actions", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Err(err).Msg("Failed to start server")
	}
}
