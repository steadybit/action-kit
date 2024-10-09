// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package action_kit_sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/phayes/freeport"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extlogging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"
)

var (
	ANY_ARG = struct{}{}
)

type ActionOperations struct {
	executionId uuid.UUID
	basePath    string
	description action_kit_api.ActionDescription
	calls       <-chan Call
}

type TestCase struct {
	Name string
	Fn   func(t *testing.T, op ActionOperations)
}

func Test_SDK(t *testing.T) {
	testCases := []TestCase{
		{
			Name: "should run a simple action",
			Fn:   testcaseSimple,
		},
		{
			Name: "should run an action with file upload",
			Fn:   testcaseWithFileUpload,
		},
		{
			Name: "should stop actions on USR1 signal",
			Fn:   testcaseUsr1Signal,
		},
		{
			Name: "should stop actions on heartbeat timeout",
			Fn:   testcaseHeartbeatTimeout,
		},
	}
	calls := make(chan Call, 1024)
	defer close(calls)

	serverPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	go func(calls chan<- Call) {
		action := NewExampleAction(calls)
		extlogging.InitZeroLog()
		RegisterAction(action)
		exthttp.RegisterHttpHandler("/", exthttp.GetterAsHandler(GetActionList))
		InstallSignalHandler()
		exthttp.Listen(exthttp.ListenOpts{Port: serverPort})
	}(calls)
	time.Sleep(1 * time.Second)

	basePath := fmt.Sprintf("http://localhost:%d", serverPort)
	actionPath := listExtension(t, basePath)
	description := describe(t, fmt.Sprintf("%s%s", basePath, actionPath))

	for _, testCase := range testCases {
		op := ActionOperations{
			basePath:    basePath,
			description: description,
			executionId: uuid.New(),
			calls:       calls,
		}

		op.resetCalls()
		t.Run(testCase.Name, func(t *testing.T) {
			testCase.Fn(t, op)
		})
	}

	fmt.Println("Yes, IntelliJ, yes, the test is finished.")
}

func testcaseSimple(t *testing.T, op ActionOperations) {
	state := op.prepare(t)
	op.assertCall(t, "Prepare", ANY_ARG, ANY_ARG)

	state = op.start(t, state)
	op.assertCall(t, "Start", toExampleState(state))

	state = op.status(t, state)
	op.assertCall(t, "Status", toExampleState(state))

	op.queryMetrics(t)
	op.assertCall(t, "QueryMetrics")

	op.stop(t, state)
	op.assertCall(t, "Stop", toExampleState(state))
}

func testcaseWithFileUpload(t *testing.T, op ActionOperations) {
	state := op.prepareWithFileUpload(t)
	op.assertCall(t, "Prepare", ANY_ARG, ANY_ARG)

	filename := toExampleState(state).InputFile
	fileContent, err := os.ReadFile(filename)
	require.NoError(t, err)
	assert.Equal(t, "This is a test file", string(fileContent[:]))

	state = op.start(t, state)
	op.assertCall(t, "Start", toExampleState(state))

	state = op.status(t, state)
	op.assertCall(t, "Status", toExampleState(state))

	op.queryMetrics(t)
	op.assertCall(t, "QueryMetrics")

	op.stop(t, state)
	op.assertCall(t, "Stop", toExampleState(state))

	_, err = os.Stat(filename)
	assert.True(t, os.IsNotExist(err))
}

func testcaseUsr1Signal(t *testing.T, op ActionOperations) {
	state := op.prepare(t)
	state = op.start(t, state)
	op.resetCalls()

	err := syscall.Kill(os.Getpid(), syscall.SIGUSR1)
	require.NoError(t, err)
	op.assertCall(t, "Stop", toExampleState(state))

	statusResult := op.statusResult(t, state)
	require.NotNil(t, statusResult.Error)
	assert.Equal(t, action_kit_api.Errored, *statusResult.Error.Status)
	assert.Equal(t, "Action was stopped by extension: received signal SIGUSR1", statusResult.Error.Title)
}

func testcaseHeartbeatTimeout(t *testing.T, op ActionOperations) {
	state := op.prepare(t)
	state = op.start(t, state)
	op.resetCalls()

	time.Sleep(25 * time.Second)
	op.assertCall(t, "Stop", toExampleState(state))

	statusResult := op.statusResult(t, state)
	require.NotNil(t, statusResult.Error)
	assert.Equal(t, action_kit_api.Errored, *statusResult.Error.Status)
	assert.Equal(t, "Action was stopped by extension: heartbeat timeout", statusResult.Error.Title)
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

func (op *ActionOperations) prepare(t *testing.T) action_kit_api.ActionState {
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
	bodyReader := bytes.NewReader(jsonBody)
	res, err := http.Post(fmt.Sprintf("%s%s", op.basePath, op.description.Prepare.Path), "application/json", bodyReader)
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
			fmt.Printf("Expected: %v, Actual: %v", &expected, actual)
			assert.EqualValues(t, expected, actual)
		}
	case <-time.After(1 * time.Second):
		assert.Fail(t, "No call to received", "Expected call to %s", name)
	}
}
