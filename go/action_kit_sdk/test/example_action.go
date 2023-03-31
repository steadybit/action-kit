// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package test

import (
	"context"
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/action-kit/go/action_kit_sdk/prepare_helper"
	"github.com/steadybit/action-kit/go/action_kit_sdk/start_helper"
	"github.com/steadybit/action-kit/go/action_kit_sdk/status_helper"
	"github.com/steadybit/action-kit/go/action_kit_sdk/stop_helper"
	"github.com/steadybit/extension-kit/extutil"
)

type ExampleAction struct {
}

type ExampleState struct {
	Duration string
	TestStep string
}

func (action ExampleAction) NewEmptyState() ExampleState {
	return ExampleState{}
}

func NewExampleAction() action_kit_sdk.Action[ExampleState] {
	return ExampleAction{}
}

func (action ExampleAction) Describe() action_kit_api.ActionDescription {
	fmt.Println("Describe!")
	return action_kit_api.ActionDescription{
		Id:          "ExampleActionId",
		Description: "This is an Example Action",
		Kind:        action_kit_api.Other,
		Prepare: action_kit_api.MutatingEndpointReference{
			Method: action_kit_api.Post,
			Path:   "/example/prepare",
		},
		Start: action_kit_api.MutatingEndpointReference{
			Method: action_kit_api.Post,
			Path:   "/example/start",
		},
		Status: &action_kit_api.MutatingEndpointReferenceWithCallInterval{
			Method:       action_kit_api.Post,
			Path:         "/example/status",
			CallInterval: extutil.Ptr("10s"),
		},
		Stop: &action_kit_api.MutatingEndpointReference{
			Method: action_kit_api.Post,
			Path:   "/example/stop",
		},
	}
}
func (action ExampleAction) Prepare(ctx context.Context, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	fmt.Println("Prepare!")
	state := ExampleState{Duration: request.Config["duration"].(string), TestStep: "Prepare"}
	return prepare_helper.NewPrepareResult(state,
		prepare_helper.WithArtifacts(&action_kit_api.Artifacts{
			{"test", "artifact-prepare"},
		}),
		prepare_helper.WithMessages(&action_kit_api.Messages{
			{Level: extutil.Ptr(action_kit_api.Info), Message: "This is a test Message from Prepare"},
		}),
		prepare_helper.WithMetrics(&action_kit_api.Metrics{
			{Metric: map[string]string{"Test": "prepare"}, Name: extutil.Ptr("TestMetric")},
		}),
	)
}
func (action ExampleAction) Start(ctx context.Context, state ExampleState) (*action_kit_api.StartResult, error) {
	fmt.Println("Start!")
	state.TestStep = "Start"
	return start_helper.NewStartResult(state,
		start_helper.WithArtifacts(&action_kit_api.Artifacts{
			{"test", "artifact-start"},
		}),
		start_helper.WithMessages(&action_kit_api.Messages{
			{Level: extutil.Ptr(action_kit_api.Info), Message: "This is a test Message from Start"},
		}),
		start_helper.WithMetrics(&action_kit_api.Metrics{
			{Metric: map[string]string{"Test": "start"}, Name: extutil.Ptr("TestMetric")},
		}),
	)
}

func (action ExampleAction) Status(ctx context.Context, state ExampleState) (*action_kit_api.StatusResult, error) {
	fmt.Println("Status!!")
	state.TestStep = "Status"
	return status_helper.NewStatusResult(state,
		status_helper.WithArtifacts(&action_kit_api.Artifacts{
			{"test", "artifact-status"},
		}),
		status_helper.WithMessages(&action_kit_api.Messages{
			{Level: extutil.Ptr(action_kit_api.Info), Message: "This is a test Message from Status"},
		}),
		status_helper.WithMetrics(&action_kit_api.Metrics{
			{Metric: map[string]string{"Test": "status"}, Name: extutil.Ptr("TestMetric")},
		}),
	)
}

func (action ExampleAction) Stop(ctx context.Context, state ExampleState) (*action_kit_api.StopResult, error) {
	fmt.Println("Stop!")
	return stop_helper.NewStopResult(
		stop_helper.WithArtifacts(&action_kit_api.Artifacts{
			{"test", "artifact-stop"},
		}),
		stop_helper.WithMessages(&action_kit_api.Messages{
			{Level: extutil.Ptr(action_kit_api.Info), Message: "This is a test Message from Stop"},
		}),
		stop_helper.WithMetrics(&action_kit_api.Metrics{
			{Metric: map[string]string{"Test": "stop"}, Name: extutil.Ptr("TestMetric")},
		}),
	)
}
