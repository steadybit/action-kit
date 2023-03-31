// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package start_helper

import (
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extconversion"
)

type StartResultModifier func(result *action_kit_api.StartResult)

func WithArtifacts(artifacts *action_kit_api.Artifacts) StartResultModifier {
	return func(result *action_kit_api.StartResult) {
		result.Artifacts = artifacts
	}
}
func WithError(error *action_kit_api.ActionKitError) StartResultModifier {
	return func(result *action_kit_api.StartResult) {
		result.Error = error
	}
}
func WithMessages(messages *action_kit_api.Messages) StartResultModifier {
	return func(result *action_kit_api.StartResult) {
		result.Messages = messages
	}
}
func WithMetrics(metrics *action_kit_api.Metrics) StartResultModifier {
	return func(result *action_kit_api.StartResult) {
		result.Metrics = metrics
	}
}

func NewStartResult(state interface{}, modifier ...StartResultModifier) (*action_kit_api.StartResult, error) {
	var result action_kit_api.StartResult
	if state != nil {
		var convertedState action_kit_api.ActionState
		err := extconversion.Convert(state, &convertedState)
		if err != nil {
			return nil, extension_kit.ToError("Failed to encode action state", err)
		}
		result.State = &convertedState
	}
	for _, resultModifier := range modifier {
		resultModifier(&result)
	}
	return &result, nil
}
