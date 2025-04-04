// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build windows

package action_kit_sdk

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/phayes/freeport"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extlogging"
	"github.com/steadybit/extension-kit/extsignals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWindowsSignals(t *testing.T) {
	defer resetDefaultServeMux()
	defer extsignals.ClearSignalHandlers()
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGTERM, os.Interrupt)
	calls := make(chan Call, 1024)
	defer close(signalChannel)
	defer close(calls)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	action := NewExampleAction(calls)
	serverPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	go func(action *ExampleAction) {
		extlogging.InitZeroLog()
		RegisterAction(action)
		exthttp.RegisterHttpHandler("/", exthttp.GetterAsHandler(GetActionList))
		extsignals.ActivateSignalHandlerWithContext(ctx)
		exthttp.Listen(exthttp.ListenOpts{Port: serverPort})
	}(action)
	time.Sleep(1 * time.Second)
	extsignals.RemoveSignalHandlersByName("Termination", "StopExtensionHTTP")

	basePath := fmt.Sprintf("http://localhost:%d", serverPort)
	actionPath := listExtension(t, basePath)
	description := describe(t, fmt.Sprintf("%s%s", basePath, actionPath))

	op := ActionOperations{
		basePath:    basePath,
		description: description,
		executionId: uuid.New(),
		calls:       calls,
		action:      action,
	}

	result, _ := op.prepare(t)
	op.assertCall(t, "Prepare", ANY_ARG, ANY_ARG)
	state := result.State

	state = op.start(t, state)
	op.resetCalls()

	var wg sync.WaitGroup
	wg.Add(1)
	go func(signals <-chan os.Signal, waitgroup *sync.WaitGroup) {
		for s := range signals {
			signalCheck := fmt.Sprintf("Signal %s", extsignals.GetSignalName(s.(syscall.Signal)))
			assert.Equal(t, signalCheck, "Signal CTRL_CLOSE_EVENT")
			wg.Done()
		}
	}(signalChannel, &wg)

	extsignals.Kill(os.Getpid())
	require.NoError(t, err)
	wg.Wait()
	op.assertCall(t, "Stop", toExampleState(state))
	signal.Stop(signalChannel)
	fmt.Println("Done")
	time.Sleep(10 * time.Second)
}
