// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package status_helper

import (
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extconversion"
)

type StatusResultModifier func(result *action_kit_api.StatusResult)

func WithArtifacts(artifacts *action_kit_api.Artifacts) StatusResultModifier {
	return func(result *action_kit_api.StatusResult) {
		result.Artifacts = artifacts
	}
}
func WithError(error *action_kit_api.ActionKitError) StatusResultModifier {
	return func(result *action_kit_api.StatusResult) {
		result.Error = error
	}
}
func WithMessages(messages *action_kit_api.Messages) StatusResultModifier {
	return func(result *action_kit_api.StatusResult) {
		result.Messages = messages
	}
}
func WithMetrics(metrics *action_kit_api.Metrics) StatusResultModifier {
	return func(result *action_kit_api.StatusResult) {
		result.Metrics = metrics
	}
}

func NewStatusResult(state interface{}, modifier ...StatusResultModifier) (*action_kit_api.StatusResult, error) {
	var result action_kit_api.StatusResult
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
