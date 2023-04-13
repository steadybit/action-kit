// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package action_kit_sdk

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk/heartbeat"
	"github.com/steadybit/action-kit/go/action_kit_sdk/state_persister"
	"github.com/steadybit/extension-kit/extconversion"
	"github.com/steadybit/extension-kit/exthttp"
	"golang.org/x/sys/unix"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"
)

var (
	registeredActions = make(map[string]interface{})
	statePersister    = state_persister.NewInmemoryStatePersister()
	stopEvents        = make([]stopEvent, 0, 10)
	heartbeatMonitors = make(map[uuid.UUID]*heartbeat.Monitor)
)

type stopEvent struct {
	timestamp   time.Time
	reason      string
	executionId uuid.UUID
}

type Action[T any] interface {
	// NewEmptyState creates a new empty state. A pointer to this state is passed to the other methods.
	NewEmptyState() T
	// Describe returns the action description.
	Describe() action_kit_api.ActionDescription
	// Prepare is called before the action is actually started. It is used to validate the action configuration and to prepare the action state.
	// [Details](https://github.com/steadybit/action-kit/blob/main/docs/action-api.md#preparation)
	Prepare(ctx context.Context, state *T, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error)
	// Start is called when the action should actually happen.
	// [Details](https://github.com/steadybit/action-kit/blob/main/docs/action-api.md#start)
	Start(ctx context.Context, state *T) (*action_kit_api.StartResult, error)
}
type ActionWithStatus[T any] interface {
	Action[T]
	// Status is used to observe the current status of the action. This is called periodically by the action-kit if time control [action_kit_api.Internal] or [action_kit_api.External] is used.
	// [Details](https://github.com/steadybit/action-kit/blob/main/docs/action-api.md#status)
	Status(ctx context.Context, state *T) (*action_kit_api.StatusResult, error)
}
type ActionWithStop[T any] interface {
	Action[T]
	// Stop is used to revert system modification or clean up any leftovers. This method is optional.
	// [Details](https://github.com/steadybit/action-kit/blob/main/docs/action-api.md#stop)
	Stop(ctx context.Context, state *T) (*action_kit_api.StopResult, error)
}
type ActionWithMetricQuery[T any] interface {
	Action[T]
	// QueryMetrics is used to fetch metrics from the action. This method is required if the action supports a metric endpoint defined by [action_kit_api.MetricsConfiguration] in the [action_kit_api.ActionDe scription].
	QueryMetrics(ctx context.Context) (*action_kit_api.QueryMetricsResult, error)
}

// InstallSignalHandler registers a signal handler that stops all active actions on SIGINT, SIGTERM and SIGUSR1.
func InstallSignalHandler() {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	go func(signals <-chan os.Signal) {
		for s := range signals {
			signalName := unix.SignalName(s.(syscall.Signal))

			log.Debug().Str("signal", signalName).Msg("received signal - stopping all active actions")
			StopAllActiveActions(fmt.Sprintf("received signal %s", signalName))

			switch s {
			case syscall.SIGINT:
				fmt.Println()
				os.Exit(128 + int(s.(syscall.Signal)))

			case syscall.SIGTERM:
				fmt.Printf("Terminated: %d\n", int(s.(syscall.Signal)))
				os.Exit(128 + int(s.(syscall.Signal)))
			}
		}
	}(signalChannel)
}

func StopAllActiveActions(reason string) {
	ctx := context.Background()
	executionIds, err := statePersister.GetExecutionIds(ctx)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to load active action states")
	}
	if len(executionIds) > 0 {
		log.Warn().Str("reason", reason).Msg("stopping active actions")
	}
	for _, executionId := range executionIds {
		StopAction(ctx, executionId, reason)
	}
}

func StopAction(ctx context.Context, executionId uuid.UUID, reason string) {
	persistedState, err := statePersister.GetState(ctx, executionId)
	if err != nil {
		log.Error().
			Err(err).
			Str("executionId", executionId.String()).
			Msgf("state cannot be loaded, cannot stop active action")
		return
	}

	action, ok := registeredActions[persistedState.ActionId]
	if !ok {
		log.Error().
			Str("actionId", persistedState.ActionId).
			Str("executionId", persistedState.ExecutionId.String()).
			Msgf("action is not registered, cannot stop active action")
		return
	}

	actionType := reflect.ValueOf(action)
	if stopMethod := actionType.MethodByName("Stop"); !stopMethod.IsNil() {
		rState := actionType.MethodByName("NewEmptyState").Call(nil)[0]
		state := reflect.New(rState.Type()).Interface()

		if err := extconversion.Convert(persistedState.State, &state); err != nil {
			log.Error().
				Str("actionId", persistedState.ActionId).
				Str("executionId", persistedState.ExecutionId.String()).
				Err(err).
				Msg("failed to convert state, cannot stop active action")
			return
		}

		log.Info().
			Str("actionId", persistedState.ActionId).
			Str("executionId", persistedState.ExecutionId.String()).
			Str("reason", reason).
			Msg("stopping active action")

		markAsStopped(persistedState.ExecutionId, reason)

		if err := stopMethod.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(state)})[1].Interface(); err != nil {
			log.Warn().
				Str("actionId", persistedState.ActionId).
				Str("executionId", persistedState.ExecutionId.String()).
				Err(err.(error)).
				Msg("failed stopping active action")
			return
		}

		stopMonitorHeartbeat(persistedState.ExecutionId)
		if err := statePersister.DeleteState(ctx, persistedState.ExecutionId); err != nil {
			log.Debug().
				Str("actionId", persistedState.ActionId).
				Str("executionId", persistedState.ExecutionId.String()).
				Err(err).
				Msg("failed deleting persisted state")
		}
	}
}

func RegisterAction[T any](a Action[T]) {
	adapter := NewActionHttpAdapter(a)
	registeredActions[adapter.description.Id] = a

	exthttp.RegisterHttpHandler(adapter.rootPath, adapter.HandleGetDescription)
	exthttp.RegisterHttpHandler(adapter.description.Prepare.Path, adapter.HandlePrepare)
	exthttp.RegisterHttpHandler(adapter.description.Start.Path, adapter.HandleStart)
	if adapter.HasStatus() {
		exthttp.RegisterHttpHandler(adapter.description.Status.Path, adapter.HandleStatus)
	}
	if adapter.HasStop() {
		exthttp.RegisterHttpHandler(adapter.description.Stop.Path, adapter.HandleStop)
	}
	if adapter.HasQueryMetric() {
		exthttp.RegisterHttpHandler(adapter.description.Metrics.Query.Endpoint.Path, adapter.HandleQueryMetric)
	}
}

// GetActionList returns a list of all root endpoints of registered actions.
func GetActionList() action_kit_api.ActionList {
	var result []action_kit_api.DescribingEndpointReference
	for actionId := range registeredActions {
		result = append(result, action_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   fmt.Sprintf("/%s", actionId),
		})
	}

	return action_kit_api.ActionList{
		Actions: result,
	}
}

func monitorHeartbeat(executionId uuid.UUID, interval, timeout time.Duration) {
	ch := make(chan time.Time, 1)
	monitor := heartbeat.Notify(ch, interval, timeout)
	heartbeatMonitors[executionId] = monitor
	go func() {
		for range ch {
			StopAction(context.Background(), executionId, "heartbeat timeout")
		}
	}()
}

func recordHeartbeat(executionId uuid.UUID) {
	monitor := heartbeatMonitors[executionId]
	if monitor != nil {
		monitor.RecordHeartbeat()
	}
}

func stopMonitorHeartbeat(executionId uuid.UUID) {
	monitor := heartbeatMonitors[executionId]
	if monitor != nil {
		monitor.Stop()
		delete(heartbeatMonitors, executionId)
	}
}

func markAsStopped(executionId uuid.UUID, reason string) {
	if len(stopEvents) > 100 {
		stopEvents = stopEvents[1:]
	}
	stopEvents = append(stopEvents, stopEvent{
		executionId: executionId,
		reason:      reason,
		timestamp:   time.Now(),
	})
}

func getStopEvent(executionId uuid.UUID) *stopEvent {
	for _, event := range stopEvents {
		if event.executionId == executionId {
			return &event
		}
	}
	return nil
}
