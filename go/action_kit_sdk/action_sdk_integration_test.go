// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package action_kit_sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extlogging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"testing"
	"time"
)

type ExtensionListResponse struct {
	Actions []action_kit_api.DescribingEndpointReference `json:"attacks"`
}

func Test_SDK(t *testing.T) {
	const serverPort = 3333

	go func() {
		action := NewExampleAction()
		extlogging.InitZeroLog()
		RegisterAction(action)
		exthttp.RegisterHttpHandler("/", exthttp.GetterAsHandler(func() ExtensionListResponse {
			return ExtensionListResponse{
				Actions: RegisteredActionsEndpoints(),
			}
		}))
		exthttp.Listen(exthttp.ListenOpts{Port: serverPort})
	}()
	time.Sleep(1 * time.Second)

	basePath := fmt.Sprintf("http://localhost:%d", serverPort)
	actionPath := listExtension(t, basePath)
	actionDescription := describe(t, fmt.Sprintf("%s%s", basePath, actionPath))
	executionId := uuid.New()

	state := prepare(t, executionId, fmt.Sprintf("%s%s", basePath, actionDescription.Prepare.Path))
	state = start(t, executionId, fmt.Sprintf("%s%s", basePath, actionDescription.Start.Path), state)
	state = status(t, executionId, fmt.Sprintf("%s%s", basePath, actionDescription.Status.Path), state)
	queryMetrics(t, executionId, fmt.Sprintf("%s%s", basePath, actionDescription.Metrics.Query.Endpoint.Path))
	stop(t, executionId, fmt.Sprintf("%s%s", basePath, actionDescription.Stop.Path), state)

	fmt.Println("Yes, IntelliJ, yes, the test is finished.")
}

func listExtension(t *testing.T, path string) string {
	res, err := http.Get(path)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	var response ExtensionListResponse
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)
	assert.NotEmpty(t, response.Actions)
	return response.Actions[0].Path
}

func describe(t *testing.T, actionPath string) action_kit_api.ActionDescription {
	res, err := http.Get(actionPath)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	var response action_kit_api.ActionDescription
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)
	assert.Equal(t, "ExampleActionId", response.Id)
	assert.NotNil(t, response.Prepare)
	assert.NotNil(t, response.Start)
	assert.NotNil(t, response.Status)
	assert.NotNil(t, response.Stop)
	return response
}

func prepare(t *testing.T, executionId uuid.UUID, path string) action_kit_api.ActionState {
	prepareBody := action_kit_api.PrepareActionRequestBody{
		ExecutionId: executionId,
		Target: &action_kit_api.Target{
			Name: "bookinfo",
			Attributes: map[string][]string{
				"k8s.namespace":    {"default"},
				"k8s.cluster-name": {"minikube"},
			},
		},
		Config: map[string]interface{}{
			"duration": "10s",
		},
	}
	jsonBody, err := json.Marshal(prepareBody)
	require.NoError(t, err)
	bodyReader := bytes.NewReader(jsonBody)
	res, err := http.Post(path, "application/json", bodyReader)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	var response action_kit_api.PrepareResult
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, "This is a test Message from Prepare", (*response.Messages)[0].Message)
	assert.Equal(t, "10s", response.State["Duration"])
	assert.Equal(t, "Prepare", response.State["TestStep"])
	assert.Len(t, *response.Metrics, 1)
	assert.Len(t, *response.Artifacts, 1)

	states, err := statePersister.GetStates(context.Background())
	require.NoError(t, err)
	assert.Len(t, states, 1)
	assert.Equal(t, "Prepare", (*states[0]).State.(ExampleState).TestStep)

	return response.State
}

func start(t *testing.T, executionId uuid.UUID, path string, state action_kit_api.ActionState) action_kit_api.ActionState {
	startBody := action_kit_api.StartActionRequestBody{State: state, ExecutionId: executionId}
	jsonBody, err := json.Marshal(startBody)
	require.NoError(t, err)
	bodyReader := bytes.NewReader(jsonBody)
	res, err := http.Post(path, "application/json", bodyReader)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	var response action_kit_api.StartResult
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, "This is a test Message from Start", (*response.Messages)[0].Message)
	assert.Equal(t, "10s", (*response.State)["Duration"])
	assert.Equal(t, "Start", (*response.State)["TestStep"])
	assert.Len(t, *response.Metrics, 1)
	assert.Len(t, *response.Artifacts, 1)

	states, err := statePersister.GetStates(context.Background())
	require.NoError(t, err)
	assert.Len(t, states, 1)
	assert.Equal(t, "Start", (*states[0]).State.(ExampleState).TestStep)

	return *response.State
}

func status(t *testing.T, executionId uuid.UUID, path string, state action_kit_api.ActionState) action_kit_api.ActionState {
	statusBody := action_kit_api.ActionStatusRequestBody{State: state, ExecutionId: executionId}
	jsonBody, err := json.Marshal(statusBody)
	require.NoError(t, err)
	bodyReader := bytes.NewReader(jsonBody)
	res, err := http.Post(path, "application/json", bodyReader)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	var response action_kit_api.StatusResult
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, "This is a test Message from Status", (*response.Messages)[0].Message)
	assert.Equal(t, "10s", (*response.State)["Duration"])
	assert.Equal(t, "Status", (*response.State)["TestStep"])
	assert.Len(t, *response.Metrics, 1)
	assert.Len(t, *response.Artifacts, 1)

	states, err := statePersister.GetStates(context.Background())
	require.NoError(t, err)
	assert.Len(t, states, 1)
	assert.Equal(t, "Status", (*states[0]).State.(ExampleState).TestStep)

	return *response.State
}

func queryMetrics(t *testing.T, executionId uuid.UUID, path string) {
	statusBody := action_kit_api.QueryMetricsRequestBody{
		ExecutionId: executionId,
		Target: &action_kit_api.Target{
			Name: "bookinfo",
			Attributes: map[string][]string{
				"k8s.namespace":    {"default"},
				"k8s.cluster-name": {"minikube"},
			},
		},
		Config: map[string]interface{}{
			"duration": "10s",
		},
		Timestamp: time.Now(),
	}
	jsonBody, err := json.Marshal(statusBody)
	require.NoError(t, err)
	bodyReader := bytes.NewReader(jsonBody)
	res, err := http.Post(path, "application/json", bodyReader)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	var response action_kit_api.QueryMetricsResult
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, "This is a test Message from QueryMetrics", (*response.Messages)[0].Message)
	assert.Len(t, *response.Metrics, 1)
	assert.Len(t, *response.Artifacts, 1)
}

func stop(t *testing.T, executionId uuid.UUID, path string, state action_kit_api.ActionState) {
	statusBody := action_kit_api.ActionStatusRequestBody{State: state, ExecutionId: executionId}
	jsonBody, err := json.Marshal(statusBody)
	require.NoError(t, err)
	bodyReader := bytes.NewReader(jsonBody)
	res, err := http.Post(path, "application/json", bodyReader)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	var response action_kit_api.StopResult
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, "This is a test Message from Stop", (*response.Messages)[0].Message)
	assert.Len(t, *response.Metrics, 1)
	assert.Len(t, *response.Artifacts, 1)

	states, err := statePersister.GetStates(context.Background())
	require.NoError(t, err)
	assert.Len(t, states, 0)
}
