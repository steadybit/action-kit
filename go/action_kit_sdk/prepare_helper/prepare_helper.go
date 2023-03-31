// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package prepare_helper

import (
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extconversion"
)

type PrepareResultModifier func(result *action_kit_api.PrepareResult)

func WithArtifacts(artifacts *action_kit_api.Artifacts) PrepareResultModifier {
	return func(result *action_kit_api.PrepareResult) {
		result.Artifacts = artifacts
	}
}
func WithError(error *action_kit_api.ActionKitError) PrepareResultModifier {
	return func(result *action_kit_api.PrepareResult) {
		result.Error = error
	}
}
func WithMessages(messages *action_kit_api.Messages) PrepareResultModifier {
	return func(result *action_kit_api.PrepareResult) {
		result.Messages = messages
	}
}
func WithMetrics(metrics *action_kit_api.Metrics) PrepareResultModifier {
	return func(result *action_kit_api.PrepareResult) {
		result.Metrics = metrics
	}
}

func NewPrepareResult(state interface{}, modifier ...PrepareResultModifier) (*action_kit_api.PrepareResult, error) {
	var convertedState action_kit_api.ActionState
	err := extconversion.Convert(state, &convertedState)
	if err != nil {
		return nil, extension_kit.ToError("Failed to encode action state", err)
	}
	result := action_kit_api.PrepareResult{
		State: convertedState,
	}
	for _, resultModifier := range modifier {
		resultModifier(&result)
	}
	return &result, nil
}
