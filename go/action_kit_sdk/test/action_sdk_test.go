// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extlogging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"testing"
	"time"
)

func Test_SDK(t *testing.T) {
	const serverPort = 3333

	go func() {
		action := NewExampleAction()
		extlogging.InitZeroLog()
		action_kit_sdk.RegisterAction(action, "/example")
		exthttp.Listen(exthttp.ListenOpts{Port: serverPort})
	}()
	time.Sleep(1 * time.Second)

	describe(t, serverPort)
	state := prepare(t, serverPort)
	state = start(t, serverPort, state)
	state = status(t, serverPort, state)
	stop(t, serverPort, state)

	fmt.Println("Yes, IntelliJ, yes, the test is finished.")
}

func describe(t *testing.T, serverPort int) {
	res, err := http.Get(fmt.Sprintf("http://localhost:%d/example", serverPort))
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
}

func prepare(t *testing.T, serverPort int) action_kit_api.ActionState {
	prepareBody := action_kit_api.PrepareActionRequestBody{
		ExecutionId: uuid.New(),
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
	res, err := http.Post(fmt.Sprintf("http://localhost:%d/example/prepare", serverPort), "application/json", bodyReader)
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
	return response.State
}

func start(t *testing.T, serverPort int, state action_kit_api.ActionState) action_kit_api.ActionState {
	startBody := action_kit_api.StartActionRequestBody{State: state}
	jsonBody, err := json.Marshal(startBody)
	require.NoError(t, err)
	bodyReader := bytes.NewReader(jsonBody)
	res, err := http.Post(fmt.Sprintf("http://localhost:%d/example/start", serverPort), "application/json", bodyReader)
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
	return *response.State
}

func status(t *testing.T, serverPort int, state action_kit_api.ActionState) action_kit_api.ActionState {
	statusBody := action_kit_api.ActionStatusRequestBody{State: state}
	jsonBody, err := json.Marshal(statusBody)
	require.NoError(t, err)
	bodyReader := bytes.NewReader(jsonBody)
	res, err := http.Post(fmt.Sprintf("http://localhost:%d/example/status", serverPort), "application/json", bodyReader)
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
	return *response.State
}

func stop(t *testing.T, serverPort int, state action_kit_api.ActionState) {
	statusBody := action_kit_api.ActionStatusRequestBody{State: state}
	jsonBody, err := json.Marshal(statusBody)
	require.NoError(t, err)
	bodyReader := bytes.NewReader(jsonBody)
	res, err := http.Post(fmt.Sprintf("http://localhost:%d/example/stop", serverPort), "application/json", bodyReader)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	var response action_kit_api.StopResult
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, "This is a test Message from Stop", (*response.Messages)[0].Message)
	assert.Len(t, *response.Metrics, 1)
	assert.Len(t, *response.Artifacts, 1)
}
