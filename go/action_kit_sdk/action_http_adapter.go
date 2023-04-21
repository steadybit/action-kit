// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package action_kit_sdk

import (
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk/state_persister"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extconversion"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
	"time"
)

type ActionHttpAdapter[T any] struct {
	description action_kit_api.ActionDescription
	action      Action[T]
	rootPath    string
}

func NewActionHttpAdapter[T any](action Action[T]) *ActionHttpAdapter[T] {
	description := getDescriptionWithDefaults(action)
	adapter := &ActionHttpAdapter[T]{
		description: description,
		action:      action,
		rootPath:    fmt.Sprintf("/%s", description.Id),
	}
	if adapter.HasQueryMetric() {
		if adapter.description.Metrics == nil {
			log.Fatal().Msgf("ActionWithMetricQuery is implemented but description.Metrics is nil.")
		}
		if adapter.description.Metrics.Query == nil {
			log.Fatal().Msgf("ActionWithMetricQuery is implemented but description.Metrics.Query is nil.")
		}
	}
	return adapter
}

func (a *ActionHttpAdapter[T]) HandleGetDescription(w http.ResponseWriter, _ *http.Request, _ []byte) {
	exthttp.WriteBody(w, a.description)
}

func (a *ActionHttpAdapter[T]) HandlePrepare(w http.ResponseWriter, r *http.Request, body []byte) {
	var parsedBody action_kit_api.PrepareActionRequestBody
	err := json.Unmarshal(body, &parsedBody)
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to parse request body.", err))
		return
	}
	state := a.action.NewEmptyState()
	result, err := a.action.Prepare(r.Context(), &state, parsedBody)
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

	if a.description.Stop != nil {
		err = statePersister.PersistState(r.Context(), &state_persister.PersistedState{ExecutionId: parsedBody.ExecutionId, ActionId: a.description.Id, State: convertedState})
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to persist action state.", err))
			return
		}
	}
	exthttp.WriteBody(w, result)
}

func (a *ActionHttpAdapter[T]) HandleStart(w http.ResponseWriter, r *http.Request, body []byte) {
	var parsedBody action_kit_api.StartActionRequestBody
	err := json.Unmarshal(body, &parsedBody)
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to parse request body.", err))
		return
	}
	state := a.action.NewEmptyState()
	err = extconversion.Convert(parsedBody.State, &state)
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to parse state.", err))
		return
	}

	result, err := a.action.Start(r.Context(), &state)
	if result == nil {
		result = &action_kit_api.StartResult{}
	}
	if err != nil {
		if extensionError, ok := err.(extension_kit.ExtensionError); ok {
			exthttp.WriteError(w, extensionError)
		} else {
			exthttp.WriteError(w, extension_kit.ToError("Failed to start.", err))
		}
		return
	}

	if result.State != nil {
		exthttp.WriteError(w, extension_kit.ToError("Please modify the state using the given state pointer.", err))
	}

	var convertedState action_kit_api.ActionState
	err = extconversion.Convert(state, &convertedState)
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to encode action state.", err))
		return
	}
	result.State = &convertedState

	if a.description.Stop != nil {
		err = statePersister.PersistState(r.Context(), &state_persister.PersistedState{ExecutionId: parsedBody.ExecutionId, ActionId: a.description.Id, State: convertedState})
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to persist action state.", err))
			return
		}

		if (a.description.Status != nil) && (a.description.Status.CallInterval != nil) {
			interval, err := time.ParseDuration(*a.description.Status.CallInterval)
			if err == nil {
				monitorHeartbeat(parsedBody.ExecutionId, interval, interval*4)
			}
		}
	}
	exthttp.WriteBody(w, result)
}

func (a *ActionHttpAdapter[T]) HasStatus() bool {
	// If the action has a stop,  we augment a status endpoint. It is used to report stops by extension.
	_, ok := a.action.(ActionWithStop[T])
	return ok || a.HasStop()
}

func (a *ActionHttpAdapter[T]) HandleStatus(w http.ResponseWriter, r *http.Request, body []byte) {
	var parsedBody action_kit_api.ActionStatusRequestBody
	err := json.Unmarshal(body, &parsedBody)
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to parse request body.", err))
		return
	}

	recordHeartbeat(parsedBody.ExecutionId)

	if stopEvent := getStopEvent(parsedBody.ExecutionId); stopEvent != nil {
		exthttp.WriteBody(w, action_kit_api.StatusResult{
			Completed: true,
			Error: &action_kit_api.ActionKitError{
				Title:  fmt.Sprintf("Action was stopped by extension: %s", stopEvent.reason),
				Status: extutil.Ptr(action_kit_api.Failed),
			},
		})
		return
	}

	action, ok := a.action.(ActionWithStatus[T])
	if !ok {
		exthttp.WriteBody(w, action_kit_api.StatusResult{
			Completed: false,
		})
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
		exthttp.WriteError(w, extension_kit.ToError("Please modify the state using the given state pointer.", err))
	}

	var convertedState action_kit_api.ActionState
	err = extconversion.Convert(state, &convertedState)
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to encode action state.", err))
		return
	}
	result.State = &convertedState

	if a.description.Stop != nil {
		err = statePersister.PersistState(r.Context(), &state_persister.PersistedState{ExecutionId: parsedBody.ExecutionId, ActionId: a.description.Id, State: convertedState})
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to persist action state.", err))
			return
		}
	}
	exthttp.WriteBody(w, result)
}

func (a *ActionHttpAdapter[T]) HasStop() bool {
	_, ok := a.action.(ActionWithStop[T])
	return ok
}

func (a *ActionHttpAdapter[T]) HandleStop(w http.ResponseWriter, r *http.Request, body []byte) {
	action := a.action.(ActionWithStop[T])

	var parsedBody action_kit_api.StopActionRequestBody
	err := json.Unmarshal(body, &parsedBody)
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to parse request body.", err))
		return
	}

	stopMonitorHeartbeat(parsedBody.ExecutionId)

	if stopEvent := getStopEvent(parsedBody.ExecutionId); stopEvent != nil {
		exthttp.WriteBody(w, action_kit_api.StopResult{
			Error: &action_kit_api.ActionKitError{
				Title: fmt.Sprintf("Action was stopped by extension %s", stopEvent.reason),
			},
		})
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
		log.Warn().
			Err(err).
			Str("actionId", a.description.Id).
			Str("executionId", parsedBody.ExecutionId.String()).
			Msg("Failed to delete action state.")
		return
	}
	exthttp.WriteBody(w, result)
}

func (a *ActionHttpAdapter[T]) HasQueryMetric() bool {
	_, ok := a.action.(ActionWithMetricQuery[T])
	return ok
}

func (a *ActionHttpAdapter[T]) HandleQueryMetric(w http.ResponseWriter, r *http.Request, body []byte) {
	action := a.action.(ActionWithMetricQuery[T])

	var parsedBody action_kit_api.QueryMetricsRequestBody
	err := json.Unmarshal(body, &parsedBody)
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to parse request body.", err))
		return
	}

	result, err := action.QueryMetrics(r.Context(), parsedBody)
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

// getDescriptionWithDefaults wraps the action description and adds default paths and methods for prepare, start, status, stop and metrics.
func getDescriptionWithDefaults[T any](action Action[T]) action_kit_api.ActionDescription {
	description := action.Describe()
	if description.Prepare.Path == "" {
		description.Prepare.Path = fmt.Sprintf("/%s/prepare", description.Id)
	}
	if description.Prepare.Method == "" {
		description.Prepare.Method = action_kit_api.Post
	}
	if description.Start.Path == "" {
		description.Start.Path = fmt.Sprintf("/%s/start", description.Id)
	}
	if description.Start.Method == "" {
		description.Start.Method = action_kit_api.Post
	}
	if _, ok := action.(ActionWithStop[T]); ok && description.Stop == nil {
		description.Stop = &action_kit_api.MutatingEndpointReference{}
	}

	if description.Stop != nil {
		if description.Stop.Path == "" {
			description.Stop.Path = fmt.Sprintf("/%s/stop", description.Id)
		}
		if description.Stop.Method == "" {
			description.Stop.Method = action_kit_api.Post
		}
	}

	if _, ok := action.(ActionWithStatus[T]); ok && description.Status == nil {
		description.Status = &action_kit_api.MutatingEndpointReferenceWithCallInterval{}
	}
	// If the action has a stop, we augment a status endpoint. It is used to check for agent heartbeats and to report extraordinary stops.
	if description.Stop != nil && description.Status == nil {
		description.Status = &action_kit_api.MutatingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("15s"),
		}
	}
	if description.Status != nil {
		if description.Status.Path == "" {
			description.Status.Path = fmt.Sprintf("/%s/status", description.Id)
		}
		if description.Status.Method == "" {
			description.Status.Method = action_kit_api.Post
		}
		if description.Status.CallInterval == nil || *description.Status.CallInterval == "" {
			description.Status.CallInterval = extutil.Ptr("5s")
		}
	}

	if description.Metrics != nil && description.Metrics.Query != nil {
		if description.Metrics.Query.Endpoint.Path == "" {
			description.Metrics.Query.Endpoint.Path = fmt.Sprintf("/%s/query", description.Id)
		}
		if description.Metrics.Query.Endpoint.Method == "" {
			description.Metrics.Query.Endpoint.Method = action_kit_api.Post
		}
	}
	return description
}
