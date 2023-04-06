// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package action_kit_sdk

import (
	"context"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extconversion"
	"github.com/steadybit/extension-kit/extutil"
)

type Call struct {
	Name string
	Args []interface{}
}

type ExampleAction struct {
	calls chan<- Call
}

type ExampleState struct {
	Duration string
	TestStep string
}

func NewExampleAction(calls chan<- Call) Action[ExampleState] {
	return &ExampleAction{calls: calls}
}

// Make sure our ExampleAction implements all the interfaces we need
var _ Action[ExampleState] = (*ExampleAction)(nil)
var _ ActionWithStatus[ExampleState] = (*ExampleAction)(nil)
var _ ActionWithStop[ExampleState] = (*ExampleAction)(nil)
var _ ActionWithMetricQuery[ExampleState] = (*ExampleAction)(nil)

func toExampleState(state action_kit_api.ActionState) *ExampleState {
	result := ExampleState{}
	err := extconversion.Convert(state, &result)
	if err != nil {
		panic(err)
	}
	return &result
}

func (action *ExampleAction) NewEmptyState() ExampleState {
	return ExampleState{}
}

func (action *ExampleAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          "ExampleActionId",
		Description: "This is an Example Action",
		Kind:        action_kit_api.Other,
		Prepare:     action_kit_api.MutatingEndpointReference{},
		Start:       action_kit_api.MutatingEndpointReference{},
		Status: &action_kit_api.MutatingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("10s"),
		},
		Stop: &action_kit_api.MutatingEndpointReference{},
		Metrics: &action_kit_api.MetricsConfiguration{
			Query: &action_kit_api.MetricsQueryConfiguration{
				Endpoint: action_kit_api.MutatingEndpointReferenceWithCallInterval{
					CallInterval: extutil.Ptr("10s"),
				},
			},
		},
	}
}
func (action *ExampleAction) Prepare(_ context.Context, state *ExampleState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	action.calls <- Call{"Prepare", []interface{}{state, request}}
	state.Duration = request.Config["duration"].(string)
	state.TestStep = "Prepare"
	return &action_kit_api.PrepareResult{
		Artifacts: &action_kit_api.Artifacts{
			{"test", "artifact-prepare"},
		},
		Messages: &action_kit_api.Messages{
			{Level: extutil.Ptr(action_kit_api.Info), Message: "This is a test Message from Prepare"},
		},
		Metrics: &action_kit_api.Metrics{
			{Metric: map[string]string{"Test": "prepare"}, Name: extutil.Ptr("TestMetric")},
		},
	}, nil
}

func (action *ExampleAction) Start(_ context.Context, state *ExampleState) (*action_kit_api.StartResult, error) {
	action.calls <- Call{"Start", []interface{}{state}}
	state.TestStep = "Start"
	return &action_kit_api.StartResult{
		Artifacts: &action_kit_api.Artifacts{
			{"test", "artifact-start"},
		},
		Messages: &action_kit_api.Messages{
			{Level: extutil.Ptr(action_kit_api.Info), Message: "This is a test Message from Start"},
		},
		Metrics: &action_kit_api.Metrics{
			{Metric: map[string]string{"Test": "start"}, Name: extutil.Ptr("TestMetric")},
		},
	}, nil
}

func (action *ExampleAction) Status(_ context.Context, state *ExampleState) (*action_kit_api.StatusResult, error) {
	action.calls <- Call{"Status", []interface{}{state}}
	state.TestStep = "Status"
	return &action_kit_api.StatusResult{
		Artifacts: &action_kit_api.Artifacts{
			{"test", "artifact-status"},
		},
		Messages: &action_kit_api.Messages{
			{Level: extutil.Ptr(action_kit_api.Info), Message: "This is a test Message from Status"},
		},
		Metrics: &action_kit_api.Metrics{
			{Metric: map[string]string{"Test": "status"}, Name: extutil.Ptr("TestMetric")},
		},
	}, nil
}

func (action *ExampleAction) Stop(_ context.Context, state *ExampleState) (*action_kit_api.StopResult, error) {
	action.calls <- Call{"Stop", []interface{}{state}}
	return &action_kit_api.StopResult{
		Artifacts: &action_kit_api.Artifacts{
			{"test", "artifact-stop"},
		},
		Messages: &action_kit_api.Messages{
			{Level: extutil.Ptr(action_kit_api.Info), Message: "This is a test Message from Stop"},
		},
		Metrics: &action_kit_api.Metrics{
			{Metric: map[string]string{"Test": "stop"}, Name: extutil.Ptr("TestMetric")},
		},
	}, nil
}

func (action *ExampleAction) QueryMetrics(_ context.Context) (*action_kit_api.QueryMetricsResult, error) {
	action.calls <- Call{"QueryMetrics", nil}
	return &action_kit_api.QueryMetricsResult{
		Artifacts: &action_kit_api.Artifacts{
			{"test", "artifact-query-metrics"},
		},
		Messages: &action_kit_api.Messages{
			{Level: extutil.Ptr(action_kit_api.Info), Message: "This is a test Message from QueryMetrics"},
		},
		Metrics: &action_kit_api.Metrics{
			{Metric: map[string]string{"Test": "query-metrics"}, Name: extutil.Ptr("TestMetric")},
		},
	}, nil
}
