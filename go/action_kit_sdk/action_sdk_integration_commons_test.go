// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package action_kit_sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	ANY_ARG = struct{}{}
)

type ActionOperations struct {
	executionId uuid.UUID
	basePath    string
	description action_kit_api.ActionDescription
	calls       <-chan Call
	action      *ExampleAction
}

func listExtension(t *testing.T, path string) string {
	res, err := http.Get(path)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	response := action_kit_api.ActionList{}
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

func (op *ActionOperations) prepare(t *testing.T) (*action_kit_api.PrepareResult, *action_kit_api.ActionKitError) {
	prepareBody := action_kit_api.PrepareActionRequestBody{
		ExecutionId: op.executionId,
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
	res, err := http.Post(fmt.Sprintf("%s%s", op.basePath, op.description.Prepare.Path), "application/json", bytes.NewReader(jsonBody))

	if res.StatusCode != http.StatusOK {
		var response action_kit_api.ActionKitError
		err = json.NewDecoder(res.Body).Decode(&response)
		require.NoError(t, err)
		return nil, &response
	}

	require.NoError(t, err)
	var response action_kit_api.PrepareResult
	err = json.NewDecoder(res.Body).Decode(&response)
	require.NoError(t, err)
	return &response, nil
}

func assertPrepareResult(t *testing.T, response action_kit_api.PrepareResult) {
	assert.Equal(t, "This is a test Message from Prepare", (*response.Messages)[0].Message)
	assert.Equal(t, "10s", response.State["Duration"])
	assert.Equal(t, "Prepare", response.State["TestStep"])
	assert.Len(t, *response.Metrics, 1)
	assert.Len(t, *response.Artifacts, 1)

	executionIds, err := statePersister.GetExecutionIds(context.Background())
	require.NoError(t, err)
	assert.Len(t, executionIds, 1)

	state, err := statePersister.GetState(context.Background(), executionIds[0])
	require.NoError(t, err)
	assert.Equal(t, "Prepare", (*state).State["TestStep"])
}

func (op *ActionOperations) prepareWithFileUpload(t *testing.T) action_kit_api.ActionState {
	prepareBody := action_kit_api.PrepareActionRequestBody{
		ExecutionId: op.executionId,
		Target: &action_kit_api.Target{
			Name: "bookinfo",
			Attributes: map[string][]string{
				"k8s.namespace":    {"default"},
				"k8s.cluster-name": {"minikube"},
			},
		},
		Config: map[string]interface{}{
			"duration":  "10s",
			"inputFile": "file::1234567890",
		},
	}
	jsonBody, err := json.Marshal(prepareBody)
	require.NoError(t, err)

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	partRequest, err := writer.CreateFormField("request")
	require.NoError(t, err)
	_, _ = partRequest.Write(jsonBody)
	partFile, err := writer.CreateFormFile("inputFile", "test.txt")
	require.NoError(t, err)
	_, _ = partFile.Write([]byte("This is a test file"))
	_ = writer.Close()
	res, err := http.Post(fmt.Sprintf("%s%s", op.basePath, op.description.Prepare.Path), writer.FormDataContentType(), &buffer)
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

	executionIds, err := statePersister.GetExecutionIds(context.Background())
	require.NoError(t, err)
	assert.Len(t, executionIds, 1)

	state, err := statePersister.GetState(context.Background(), executionIds[0])
	require.NoError(t, err)
	assert.Equal(t, "Prepare", (*state).State["TestStep"])

	return response.State
}

func (op *ActionOperations) start(t *testing.T, state action_kit_api.ActionState) action_kit_api.ActionState {
	startBody := action_kit_api.StartActionRequestBody{State: state, ExecutionId: op.executionId}
	jsonBody, err := json.Marshal(startBody)
	require.NoError(t, err)
	bodyReader := bytes.NewReader(jsonBody)
	res, err := http.Post(fmt.Sprintf("%s%s", op.basePath, op.description.Start.Path), "application/json", bodyReader)
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

	executionIds, err := statePersister.GetExecutionIds(context.Background())
	require.NoError(t, err)
	assert.Len(t, executionIds, 1)

	pState, err := statePersister.GetState(context.Background(), executionIds[0])
	require.NoError(t, err)
	assert.Equal(t, "Start", (*pState).State["TestStep"])

	return *response.State
}

func (op *ActionOperations) status(t *testing.T, state action_kit_api.ActionState) action_kit_api.ActionState {
	response := op.statusResult(t, state)

	assert.Equal(t, "This is a test Message from Status", (*response.Messages)[0].Message)
	assert.Equal(t, "10s", (*response.State)["Duration"])
	assert.Equal(t, "Status", (*response.State)["TestStep"])
	assert.Len(t, *response.Metrics, 1)
	assert.Len(t, *response.Artifacts, 1)

	executionIds, err := statePersister.GetExecutionIds(context.Background())
	require.NoError(t, err)
	assert.Len(t, executionIds, 1)

	pState, err := statePersister.GetState(context.Background(), executionIds[0])
	require.NoError(t, err)
	assert.Equal(t, "Status", (*pState).State["TestStep"])

	return *response.State
}

func (op *ActionOperations) statusResult(t *testing.T, state action_kit_api.ActionState) action_kit_api.StatusResult {
	statusBody := action_kit_api.ActionStatusRequestBody{State: state, ExecutionId: op.executionId}
	jsonBody, err := json.Marshal(statusBody)
	require.NoError(t, err)
	bodyReader := bytes.NewReader(jsonBody)
	res, err := http.Post(fmt.Sprintf("%s%s", op.basePath, op.description.Status.Path), "application/json", bodyReader)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	var response action_kit_api.StatusResult
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)
	return response
}

func (op *ActionOperations) queryMetrics(t *testing.T) {
	statusBody := action_kit_api.QueryMetricsRequestBody{
		ExecutionId: op.executionId,
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
	res, err := http.Post(fmt.Sprintf("%s%s", op.basePath, op.description.Metrics.Query.Endpoint.Path), "application/json", bodyReader)
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

func (op *ActionOperations) stop(t *testing.T, state action_kit_api.ActionState) {
	statusBody := action_kit_api.ActionStatusRequestBody{State: state, ExecutionId: op.executionId}
	jsonBody, err := json.Marshal(statusBody)
	require.NoError(t, err)
	bodyReader := bytes.NewReader(jsonBody)
	res, err := http.Post(fmt.Sprintf("%s%s", op.basePath, op.description.Stop.Path), "application/json", bodyReader)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	var response action_kit_api.StopResult
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, "This is a test Message from Stop", (*response.Messages)[0].Message)
	assert.Len(t, *response.Metrics, 1)
	assert.Len(t, *response.Artifacts, 1)

	executionIds, err := statePersister.GetExecutionIds(context.Background())
	require.NoError(t, err)
	assert.Len(t, executionIds, 0)
}

func (op *ActionOperations) resetCalls() {
	for len(op.calls) > 0 {
		<-op.calls
	}
}

func (op *ActionOperations) assertCall(t *testing.T, name string, args ...interface{}) {
	select {
	case call := <-op.calls:
		assert.Equal(t, name, call.Name)
		assert.Equal(t, len(args), len(call.Args), "Arguments differ in length")
		for i, expected := range args {
			if expected == ANY_ARG {
				continue
			}
			actual := call.Args[i]
			fmt.Printf("Expected: %v, Actual: %v", expected, actual)
			assert.EqualValues(t, expected, actual)
		}
	case <-time.After(1 * time.Second):
		assert.Fail(t, "No call to received", "Expected call to %s", name)
	}
}

func resetDefaultServeMux() {
	mux := http.DefaultServeMux
	v := reflect.ValueOf(mux).Elem()
	v.Set(reflect.Zero(v.Type()))
}
