package action_kit_sdk

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/phayes/freeport"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extsignals"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestCase struct {
	Name string
	Fn   func(t *testing.T, op ActionOperations)
}

func Test_SDK(t *testing.T) {
	defer resetDefaultServeMux()
	defer extsignals.ClearSignalHandlers()
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
		{
			Name: "should return error from prepare and update state",
			Fn:   testCasePrepareWithGenericErrorAndStateUpdate,
		},
		{
			Name: "should return extension error from prepare and update state",
			Fn:   testCasePrepareWithExtensionKitErrorAndStateUpdate,
		},
		{
			Name: "should return error from start and update state",
			Fn:   testCaseStartWithGenericErrorAndStateUpdate,
		},
		{
			Name: "should return extension start from prepare and update state",
			Fn:   testCaseStartWithExtensionKitErrorAndStateUpdate,
		},
		{
			Name: "should return error from status and update state",
			Fn:   testCaseStatusWithGenericErrorAndStateUpdate,
		},
		{
			Name: "should return extension status from prepare and update state",
			Fn:   testCaseStatusWithExtensionKitErrorAndStateUpdate,
		},
	}
	calls := make(chan Call, 1024)
	defer close(calls)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	action := NewExampleAction(calls)
	go func(action *ExampleAction) {
		// extlogging.InitZeroLog()
		RegisterAction(action)
		exthttp.RegisterHttpHandler("/", exthttp.GetterAsHandler(GetActionList))
		extsignals.ActivateSignalHandlerWithContext(ctx)
		exthttp.Listen(exthttp.ListenOpts{Port: serverPort})
	}(action)
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
			action:      action,
		}

		op.resetCalls()
		t.Run(testCase.Name, func(t *testing.T) {
			testCase.Fn(t, op)
		})
	}

	fmt.Println("Yes, IntelliJ, yes, the test is finished.")
}

func testcaseSimple(t *testing.T, op ActionOperations) {
	result, _ := op.prepare(t)
	assertPrepareResult(t, *result)
	op.assertCall(t, "Prepare", ANY_ARG, ANY_ARG)
	state := result.State

	startResult := op.start(t, state)
	assertStartResult(t, *startResult)
	state = *startResult.State
	op.assertCall(t, "Start", toExampleState(state))

	statusResult := op.status(t, state)
	assertStatusResult(t, *statusResult)
	state = *statusResult.State
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

	startResult := op.start(t, state)
	assertStartResult(t, *startResult)
	state = *startResult.State
	op.assertCall(t, "Start", toExampleState(state))

	statusResult := op.status(t, state)
	assertStatusResult(t, *statusResult)
	state = *statusResult.State
	op.assertCall(t, "Status", toExampleState(state))

	op.queryMetrics(t)
	op.assertCall(t, "QueryMetrics")

	op.stop(t, state)
	op.assertCall(t, "Stop", toExampleState(state))

	_, err = os.Stat(filename)
	assert.True(t, os.IsNotExist(err))
}

func testcaseUsr1Signal(t *testing.T, op ActionOperations) {
	extsignals.RemoveSignalHandlersByName("Termination", "StopExtensionHTTP")

	result, _ := op.prepare(t)
	state := result.State

	startResult := op.start(t, state)
	assertStartResult(t, *startResult)
	state = *startResult.State
	op.resetCalls()

	err := extsignals.Kill(os.Getpid())
	fmt.Println("Process killed (catching events).") // Required for windows (for now).
	require.NoError(t, err)
	op.assertCall(t, "Stop", toExampleState(state))

	statusResult := op.status(t, state)
	require.NotNil(t, statusResult.Error)
	assert.Equal(t, action_kit_api.Errored, *statusResult.Error.Status)

	fmt.Println("Linux: " + statusResult.Error.Title)
	assert.Equal(t, "Action was stopped by extension: received signal SIGUSR1", statusResult.Error.Title)
}

func testcaseHeartbeatTimeout(t *testing.T, op ActionOperations) {
	result, _ := op.prepare(t)
	state := result.State

	startResult := op.start(t, state)
	assertStartResult(t, *startResult)
	state = *startResult.State
	op.resetCalls()

	time.Sleep(25 * time.Second)
	op.assertCall(t, "Stop", toExampleState(state))

	statusResult := op.status(t, state)
	require.NotNil(t, statusResult.Error)
	assert.Equal(t, action_kit_api.Errored, *statusResult.Error.Status)
	assert.Equal(t, "Action was stopped by extension: heartbeat timeout", statusResult.Error.Title)
}

func testCasePrepareWithGenericErrorAndStateUpdate(t *testing.T, op ActionOperations) {
	op.action.prepareError = fmt.Errorf("this is a test error")
	prepareResult, actionError := op.prepare(t)
	assert.Nil(t, actionError)
	assert.Equal(t, &action_kit_api.ActionKitError{Title: "Failed to prepare.", Detail: extutil.Ptr("this is a test error")}, prepareResult.Error)
	assert.Equal(t, "PrepareBeforeError", prepareResult.State["TestStep"].(string))
	op.assertCall(t, "Prepare", ANY_ARG, ANY_ARG)
}

func testCasePrepareWithExtensionKitErrorAndStateUpdate(t *testing.T, op ActionOperations) {
	op.action.prepareError = extutil.Ptr(extension_kit.ToError("this is a test error", errors.New("with some details")))
	prepareResult, actionError := op.prepare(t)
	assert.Nil(t, actionError)
	assert.Equal(t, &action_kit_api.ActionKitError{Title: "this is a test error", Detail: extutil.Ptr("with some details")}, prepareResult.Error)
	assert.Equal(t, "PrepareBeforeError", prepareResult.State["TestStep"].(string))
	op.assertCall(t, "Prepare", ANY_ARG, ANY_ARG)
}

func testCaseStartWithGenericErrorAndStateUpdate(t *testing.T, op ActionOperations) {
	op.action.startError = fmt.Errorf("this is a test error")
	var state action_kit_api.ActionState
	startResult := op.start(t, state)
	assert.Equal(t, &action_kit_api.ActionKitError{Title: "Failed to start action.", Detail: extutil.Ptr("this is a test error")}, startResult.Error)
	assert.Equal(t, "StartBeforeError", (*startResult.State)["TestStep"].(string))
	op.assertCall(t, "Start", ANY_ARG)
}

func testCaseStartWithExtensionKitErrorAndStateUpdate(t *testing.T, op ActionOperations) {
	op.action.startError = extutil.Ptr(extension_kit.ToError("this is a test error", errors.New("with some details")))
	var state action_kit_api.ActionState
	startResult := op.start(t, state)
	assert.Equal(t, &action_kit_api.ActionKitError{Title: "this is a test error", Detail: extutil.Ptr("with some details")}, startResult.Error)
	assert.Equal(t, "StartBeforeError", (*startResult.State)["TestStep"].(string))
	op.assertCall(t, "Start", ANY_ARG)
}

func testCaseStatusWithGenericErrorAndStateUpdate(t *testing.T, op ActionOperations) {
	op.action.statusError = fmt.Errorf("this is a test error")
	var state action_kit_api.ActionState
	startResult := op.status(t, state)
	assert.Equal(t, &action_kit_api.ActionKitError{Title: "Failed to read status.", Detail: extutil.Ptr("this is a test error")}, startResult.Error)
	assert.Equal(t, "StatusBeforeError", (*startResult.State)["TestStep"].(string))
	op.assertCall(t, "Status", ANY_ARG)
}

func testCaseStatusWithExtensionKitErrorAndStateUpdate(t *testing.T, op ActionOperations) {
	op.action.statusError = extutil.Ptr(extension_kit.ToError("this is a test error", errors.New("with some details")))
	var state action_kit_api.ActionState
	startResult := op.status(t, state)
	assert.Equal(t, &action_kit_api.ActionKitError{Title: "this is a test error", Detail: extutil.Ptr("with some details")}, startResult.Error)
	assert.Equal(t, "StatusBeforeError", (*startResult.State)["TestStep"].(string))
	op.assertCall(t, "Status", ANY_ARG)
}
