// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package action_kit_sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk/heartbeat"
	"github.com/steadybit/action-kit/go/action_kit_sdk/state_persister"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extconversion"
	"github.com/steadybit/extension-kit/exthttp"
	"net/http"
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
	actionDescription := wrapDescribe(a.Describe())
	actionId := actionDescription.Id
	registeredActions[actionId] = a

	exthttp.RegisterHttpHandler(fmt.Sprintf("/%s", actionId), exthttp.GetterAsHandler(func() action_kit_api.ActionDescription {
		return actionDescription
	}))
	exthttp.RegisterHttpHandler(actionDescription.Prepare.Path, wrapPrepare(a))
	exthttp.RegisterHttpHandler(actionDescription.Start.Path, wrapStart(a))
	if actionWithStatus, ok := a.(ActionWithStatus[T]); ok {
		if actionDescription.Status == nil {
			log.Fatal().Msgf("ActionWithStatus is implemented but actionDescription.Status is nil.")
		}
		exthttp.RegisterHttpHandler(actionDescription.Status.Path, wrapStatus(actionWithStatus))
	}
	if actionWithStop, ok := a.(ActionWithStop[T]); ok {
		if actionDescription.Stop == nil {
			log.Fatal().Msgf("ActionWithStop is implemented but actionDescription.Stop is nil.")
		}
		exthttp.RegisterHttpHandler(actionDescription.Stop.Path, wrapStop(actionWithStop))
	}
	if actionWithMetricQuery, ok := a.(ActionWithMetricQuery[T]); ok {
		if actionDescription.Metrics == nil {
			log.Fatal().Msgf("ActionWithMetricQuery is implemented but actionDescription.Metrics is nil.")
		}
		if actionDescription.Metrics.Query == nil {
			log.Fatal().Msgf("ActionWithMetricQuery is implemented but actionDescription.Metrics.Query is nil.")
		}
		exthttp.RegisterHttpHandler(actionDescription.Metrics.Query.Endpoint.Path, wrapMetricQuery(actionWithMetricQuery))
	}
}

// wrapDescribe wraps the action description and adds default paths and methods for prepare, start, status, stop and metrics.
func wrapDescribe(actionDescription action_kit_api.ActionDescription) action_kit_api.ActionDescription {
	if actionDescription.Prepare.Path == "" {
		actionDescription.Prepare.Path = fmt.Sprintf("/%s/prepare", actionDescription.Id)
	}
	if actionDescription.Prepare.Method == "" {
		actionDescription.Prepare.Method = action_kit_api.Post
	}
	if actionDescription.Start.Path == "" {
		actionDescription.Start.Path = fmt.Sprintf("/%s/start", actionDescription.Id)
	}
	if actionDescription.Start.Method == "" {
		actionDescription.Start.Method = action_kit_api.Post
	}
	if actionDescription.Status != nil {
		if actionDescription.Status.Path == "" {
			actionDescription.Status.Path = fmt.Sprintf("/%s/status", actionDescription.Id)
		}
		if actionDescription.Status.Method == "" {
			actionDescription.Status.Method = action_kit_api.Post
		}
	}
	if actionDescription.Stop != nil {
		if actionDescription.Stop.Path == "" {
			actionDescription.Stop.Path = fmt.Sprintf("/%s/stop", actionDescription.Id)
		}
		if actionDescription.Stop.Method == "" {
			actionDescription.Stop.Method = action_kit_api.Post
		}
	}
	if actionDescription.Metrics != nil && actionDescription.Metrics.Query != nil {
		if actionDescription.Metrics.Query.Endpoint.Path == "" {
			actionDescription.Metrics.Query.Endpoint.Path = fmt.Sprintf("/%s/query", actionDescription.Id)
		}
		if actionDescription.Metrics.Query.Endpoint.Method == "" {
			actionDescription.Metrics.Query.Endpoint.Method = action_kit_api.Post
		}
	}
	return actionDescription
}

func wrapPrepare[T any](action Action[T]) func(w http.ResponseWriter, r *http.Request, body []byte) {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {
		var parsedBody action_kit_api.PrepareActionRequestBody
		err := json.Unmarshal(body, &parsedBody)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to parse request body.", err))
			return
		}
		state := action.NewEmptyState()
		result, err := action.Prepare(r.Context(), &state, parsedBody)
		if err != nil {
			extensionError, isExtensionError := err.(extension_kit.ExtensionError)
			if isExtensionError {
				exthttp.WriteError(w, extensionError)
			} else {
				exthttp.WriteError(w, extension_kit.ToError("Failed to prepare.", err))
			}
			return
		}
		if result == nil {
			result = &action_kit_api.PrepareResult{}
		}
		if result.State != nil {
			exthttp.WriteError(w, extension_kit.ToError(" Please modify the state using the given state pointer.", err))
		}

		var convertedState action_kit_api.ActionState
		err = extconversion.Convert(state, &convertedState)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to encode action state.", err))
			return
		}
		result.State = convertedState

		if action.Describe().Stop != nil {
			err = statePersister.PersistState(r.Context(), &state_persister.PersistedState{ExecutionId: parsedBody.ExecutionId, ActionId: action.Describe().Id, State: convertedState})
			if err != nil {
				exthttp.WriteError(w, extension_kit.ToError("Failed to persist action state.", err))
				return
			}
		}
		exthttp.WriteBody(w, result)
	}
}

func wrapStart[T any](action Action[T]) func(w http.ResponseWriter, r *http.Request, body []byte) {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {
		var parsedBody action_kit_api.StartActionRequestBody
		err := json.Unmarshal(body, &parsedBody)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to parse request body.", err))
			return
		}
		state := action.NewEmptyState()
		err = extconversion.Convert(parsedBody.State, &state)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to parse state.", err))
			return
		}

		result, err := action.Start(r.Context(), &state)
		if result == nil {
			result = &action_kit_api.StartResult{}
		}
		if err != nil {
			extensionError, isExtensionError := err.(extension_kit.ExtensionError)
			if isExtensionError {
				exthttp.WriteError(w, extensionError)
			} else {
				exthttp.WriteError(w, extension_kit.ToError("Failed to start.", err))
			}
			return
		}

		if result.State != nil {
			exthttp.WriteError(w, extension_kit.ToError(" Please modify the state using the given state pointer.", err))
		}

		var convertedState action_kit_api.ActionState
		err = extconversion.Convert(state, &convertedState)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to encode action state.", err))
			return
		}
		result.State = &convertedState

		if action.Describe().Stop != nil {
			err = statePersister.PersistState(r.Context(), &state_persister.PersistedState{ExecutionId: parsedBody.ExecutionId, ActionId: action.Describe().Id, State: convertedState})
			if err != nil {
				exthttp.WriteError(w, extension_kit.ToError("Failed to persist action state.", err))
				return
			}
		}
		exthttp.WriteBody(w, result)
	}
}

func wrapStatus[T any](action ActionWithStatus[T]) func(w http.ResponseWriter, r *http.Request, body []byte) {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {
		var parsedBody action_kit_api.ActionStatusRequestBody
		err := json.Unmarshal(body, &parsedBody)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to parse request body.", err))
			return
		}
		state := action.NewEmptyState()
		err = extconversion.Convert(parsedBody.State, &state)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to parse state.", err))
			return
		}

		result, err := action.Status(r.Context(), &state)
		if result == nil {
			result = &action_kit_api.StatusResult{}
		}
		if err != nil {
			extensionError, isExtensionError := err.(extension_kit.ExtensionError)
			if isExtensionError {
				exthttp.WriteError(w, extensionError)
			} else {
				exthttp.WriteError(w, extension_kit.ToError("Failed to read status.", err))
			}
			return
		}

		if result.State != nil {
			exthttp.WriteError(w, extension_kit.ToError(" Please modify the state using the given state pointer.", err))
		}

		var convertedState action_kit_api.ActionState
		err = extconversion.Convert(state, &convertedState)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to encode action state.", err))
			return
		}
		result.State = &convertedState

		if action.Describe().Stop != nil {
			err = statePersister.PersistState(r.Context(), &state_persister.PersistedState{ExecutionId: parsedBody.ExecutionId, ActionId: action.Describe().Id, State: convertedState})
			if err != nil {
				exthttp.WriteError(w, extension_kit.ToError("Failed to persist action state.", err))
				return
			}
		}
		exthttp.WriteBody(w, result)
	}
}

func wrapStop[T any](action ActionWithStop[T]) func(w http.ResponseWriter, r *http.Request, body []byte) {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {
		var parsedBody action_kit_api.StopActionRequestBody
		err := json.Unmarshal(body, &parsedBody)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to parse request body.", err))
			return
		}
		state := action.NewEmptyState()
		err = extconversion.Convert(parsedBody.State, &state)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to parse state.", err))
			return
		}

		result, err := action.Stop(r.Context(), &state)
		if result == nil {
			result = &action_kit_api.StopResult{}
		}
		if err != nil {
			extensionError, isExtensionError := err.(extension_kit.ExtensionError)
			if isExtensionError {
				exthttp.WriteError(w, extensionError)
			} else {
				exthttp.WriteError(w, extension_kit.ToError("Failed to stop.", err))
			}
			return
		}

		err = statePersister.DeleteState(r.Context(), parsedBody.ExecutionId)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to delete action state.", err))
			return
		}
		exthttp.WriteBody(w, result)
	}
}

func wrapMetricQuery[T any](action ActionWithMetricQuery[T]) func(w http.ResponseWriter, r *http.Request, body []byte) {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {
		var parsedBody action_kit_api.QueryMetricsRequestBody
		err := json.Unmarshal(body, &parsedBody)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to parse request body.", err))
			return
		}

		result, err := action.QueryMetrics(r.Context())
		if result == nil {
			result = &action_kit_api.QueryMetricsResult{}
		}
		if err != nil {
			extensionError, isExtensionError := err.(extension_kit.ExtensionError)
			if isExtensionError {
				exthttp.WriteError(w, extensionError)
			} else {
				exthttp.WriteError(w, extension_kit.ToError("Failed to query metrics.", err))
			}
			return
		}
		exthttp.WriteBody(w, result)
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
