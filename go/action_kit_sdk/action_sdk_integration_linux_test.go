//go:build !windows

package action_kit_sdk

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/phayes/freeport"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extsignals"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"runtime"
	"testing"
	"time"
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
			Name: "should return error from prepare",
			Fn:   testCasePrepareWithGenericError,
		},
		{
			Name: "should return extension error from prepare",
			Fn:   testCasePrepareWithExtensionKitError,
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
	extsignals.RemoveSignalHandlersByName("Termination", "StopExtensionHTTP")

	result, _ := op.prepare(t)
	state := result.State

	state = op.start(t, state)
	op.resetCalls()

	err := extsignals.Kill(os.Getpid())
	fmt.Println("Process killed (catching events).") // Required for windows (for now).
	require.NoError(t, err)
	op.assertCall(t, "Stop", toExampleState(state))

	statusResult := op.statusResult(t, state)
	require.NotNil(t, statusResult.Error)
	assert.Equal(t, action_kit_api.Errored, *statusResult.Error.Status)

	if runtime.GOOS == "windows" {
		fmt.Println("Windows: " + statusResult.Error.Title)
		assert.Equal(t, "Action was stopped by extension: received signal CTRL_CLOSE_EVENT", statusResult.Error.Title)
	} else {
		fmt.Println("Linux: " + statusResult.Error.Title)
		assert.Equal(t, "Action was stopped by extension: received signal SIGUSR1", statusResult.Error.Title)
	}
}

func testcaseHeartbeatTimeout(t *testing.T, op ActionOperations) {
	result, _ := op.prepare(t)
	state := result.State

	state = op.start(t, state)
	op.resetCalls()

	time.Sleep(25 * time.Second)
	op.assertCall(t, "Stop", toExampleState(state))

	statusResult := op.statusResult(t, state)
	require.NotNil(t, statusResult.Error)
	assert.Equal(t, action_kit_api.Errored, *statusResult.Error.Status)
	assert.Equal(t, "Action was stopped by extension: heartbeat timeout", statusResult.Error.Title)
}

func testCasePrepareWithGenericError(t *testing.T, op ActionOperations) {
	op.action.prepareError = fmt.Errorf("this is a test error")
	_, response := op.prepare(t)
	assert.Equal(t, &action_kit_api.ActionKitError{Title: "Failed to prepare.", Detail: extutil.Ptr("this is a test error")}, response)
	op.assertCall(t, "Prepare", ANY_ARG, ANY_ARG)
}

func testCasePrepareWithExtensionKitError(t *testing.T, op ActionOperations) {
	op.action.prepareError = extutil.Ptr(extension_kit.ToError("this is a test error", errors.New("with some setails")))
	_, response := op.prepare(t)
	assert.Equal(t, &action_kit_api.ActionKitError{Title: "this is a test error", Detail: extutil.Ptr("with some setails")}, response)
	op.assertCall(t, "Prepare", ANY_ARG, ANY_ARG)
}
