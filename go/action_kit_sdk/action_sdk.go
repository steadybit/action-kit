// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package action_kit_sdk

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk/heartbeat"
	"github.com/steadybit/action-kit/go/action_kit_sdk/state_persister"
	"github.com/steadybit/extension-kit/extconversion"
	"github.com/steadybit/extension-kit/exthttp"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"
)

var (
	registeredActions = make(map[string]interface{})
	statePersister    = state_persister.NewInmemoryStatePersister()
)

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
	// Status is used to observe the current status of the action. This is called periodically by the action-kit if time control [action_kit_api.Internal] is used.
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

// Start starts the safety nets of the action-kit sdk. A heartbeat will constantly check the connection to the agent. The method returns a function that needs to be deferred to close the required resources.
func Start() func() {
	hb := heartbeat.StartAndRegisterHandler()
	go func(heartbeats <-chan time.Time) {
		for range heartbeats {
			StopAllActiveActions("heartbeat failed")
		}
	}(hb.Channel())

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM, syscall.SIGUSR1)
	go func(signals <-chan os.Signal) {
		for s := range signals {
			log.Debug().Str("signal", s.String()).Msg("received signal")
			StopAllActiveActions(fmt.Sprintf("received signal %s", s))
		}
	}(signalChannel)

	return func() {
		close(signalChannel)
		hb.Stop()
	}
}

func StopAllActiveActions(reason string) {
	ctx := context.Background()
	states, err := statePersister.GetStates(ctx)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to load active action states")
	}
	if len(states) > 0 {
		log.Warn().Str("reason", reason).Msg("stopping active actions")
	}
	for _, persistedState := range states {
		action, ok := registeredActions[persistedState.ActionId]
		if !ok {
			log.Info().
				Str("actionId", persistedState.ActionId).
				Str("executionId", persistedState.ExecutionId.String()).
				Msgf("action is not registered, cannot stop active action")
			continue
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
				continue
			}

			log.Info().
				Str("actionId", persistedState.ActionId).
				Str("executionId", persistedState.ExecutionId.String()).
				Msg("stopping active action")

			if err := stopMethod.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(state)})[1].Interface(); err != nil {
				log.Warn().
					Str("actionId", persistedState.ActionId).
					Str("executionId", persistedState.ExecutionId.String()).
					Err(err.(error)).
					Msg("failed stopping active action")
			}
		}
	}
}

func RegisterAction[T any](a Action[T]) {
	adapter := NewActionHttpAdapter(a)
	registeredActions[adapter.description.Id] = a

	exthttp.RegisterHttpHandler(adapter.rootPath, adapter.GetDescription)
	exthttp.RegisterHttpHandler(adapter.description.Prepare.Path, adapter.Prepare)
	exthttp.RegisterHttpHandler(adapter.description.Start.Path, adapter.Start)
	if adapter.HasStatus() {
		exthttp.RegisterHttpHandler(adapter.description.Status.Path, adapter.Status)
	}
	if adapter.HasStop() {
		exthttp.RegisterHttpHandler(adapter.description.Stop.Path, adapter.Stop)
	}
	if adapter.HasQueryMetric() {
		exthttp.RegisterHttpHandler(adapter.description.Metrics.Query.Endpoint.Path, adapter.QueryMetric)
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
		Heartbeat: &action_kit_api.DescribingEndpointReference{
			Method: "POST",
			Path:   "/heartbeat",
		},
		Actions: result,
	}
}
