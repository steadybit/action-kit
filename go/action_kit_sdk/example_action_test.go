// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package action_kit_sdk

import (
	"context"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extconversion"
	"github.com/steadybit/extension-kit/extutil"
)

type Call struct {
	Name string
	Args []interface{}
}

type ExampleAction struct {
	calls        chan<- Call
	prepareError error
	startError   error
	statusError  error
}

type ExampleState struct {
	Duration  string
	InputFile string
	TestStep  string
}

type ExampleConfig struct {
	Duration  string
	InputFile string
}

func NewExampleAction(calls chan<- Call) *ExampleAction {
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
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: extutil.Ptr("10s"),
			},
			{
				Name:          "duration",
				Label:         "Duration with custom units",
				DurationUnits: extutil.Ptr([]action_kit_api.DurationUnit{action_kit_api.DurationUnitSeconds, action_kit_api.DurationUnitMinutes}),
				Type:          action_kit_api.ActionParameterTypeDuration,
				DefaultValue:  extutil.Ptr("10s"),
			},
			{
				Name:     "inputFile",
				Label:    "Input File",
				Type:     action_kit_api.ActionParameterTypeFile,
				Required: extutil.Ptr(true),
				AcceptedFileTypes: extutil.Ptr([]string{
					".txt",
				}),
			},
			{
				Name:     "inputFile2",
				Label:    "Input File 2 (optional)",
				Type:     action_kit_api.ActionParameterTypeFile,
				Required: extutil.Ptr(false),
				AcceptedFileTypes: extutil.Ptr([]string{
					".txt",
				}),
			},
		},
		Prepare: action_kit_api.MutatingEndpointReference{},
		Start:   action_kit_api.MutatingEndpointReference{},
		Status: &action_kit_api.MutatingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("1s"),
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
	var config ExampleConfig
	if err := extconversion.Convert(request.Config, &config); err != nil {
		return nil, extension_kit.ToError("Failed to unmarshal the config.", err)
	}

	state.TestStep = "PrepareBeforeError"
	if action.prepareError != nil {
		return nil, action.prepareError
	}

	state.Duration = config.Duration
	state.InputFile = config.InputFile
	state.TestStep = "Prepare"
	return &action_kit_api.PrepareResult{
		Artifacts: &action_kit_api.Artifacts{
			{Data: "test", Label: "artifact-prepare"},
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
	state.TestStep = "StartBeforeError"
	if action.startError != nil {
		return nil, action.startError
	}

	state.TestStep = "Start"
	return &action_kit_api.StartResult{
		Artifacts: &action_kit_api.Artifacts{
			{Data: "test", Label: "artifact-start"},
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
	state.TestStep = "StatusBeforeError"
	if action.statusError != nil {
		return nil, action.statusError
	}

	state.TestStep = "Status"
	return &action_kit_api.StatusResult{
		Artifacts: &action_kit_api.Artifacts{
			{Data: "test", Label: "artifact-status"},
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
			{Data: "test", Label: "artifact-stop"},
		},
		Messages: &action_kit_api.Messages{
			{Level: extutil.Ptr(action_kit_api.Info), Message: "This is a test Message from Stop"},
		},
		Metrics: &action_kit_api.Metrics{
			{Metric: map[string]string{"Test": "stop"}, Name: extutil.Ptr("TestMetric")},
		},
	}, nil
}

func (action *ExampleAction) QueryMetrics(_ context.Context, _ action_kit_api.QueryMetricsRequestBody) (*action_kit_api.QueryMetricsResult, error) {
	action.calls <- Call{"QueryMetrics", nil}
	return &action_kit_api.QueryMetricsResult{
		Artifacts: &action_kit_api.Artifacts{
			{Data: "test", Label: "artifact-query-metrics"},
		},
		Messages: &action_kit_api.Messages{
			{Level: extutil.Ptr(action_kit_api.Info), Message: "This is a test Message from QueryMetrics"},
		},
		Metrics: &action_kit_api.Metrics{
			{Metric: map[string]string{"Test": "query-metrics"}, Name: extutil.Ptr("TestMetric")},
		},
	}, nil
}
