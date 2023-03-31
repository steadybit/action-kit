// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package stop_helper

import (
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
)

type StopResultModifier func(result *action_kit_api.StopResult)

func WithArtifacts(artifacts *action_kit_api.Artifacts) StopResultModifier {
	return func(result *action_kit_api.StopResult) {
		result.Artifacts = artifacts
	}
}
func WithError(error *action_kit_api.ActionKitError) StopResultModifier {
	return func(result *action_kit_api.StopResult) {
		result.Error = error
	}
}
func WithMessages(messages *action_kit_api.Messages) StopResultModifier {
	return func(result *action_kit_api.StopResult) {
		result.Messages = messages
	}
}
func WithMetrics(metrics *action_kit_api.Metrics) StopResultModifier {
	return func(result *action_kit_api.StopResult) {
		result.Metrics = metrics
	}
}

func NewStopResult(modifier ...StopResultModifier) (*action_kit_api.StopResult, error) {
	var result action_kit_api.StopResult
	for _, resultModifier := range modifier {
		resultModifier(&result)
	}
	return &result, nil
}
