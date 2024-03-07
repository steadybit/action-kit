// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package action_kit_sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk/state_persister"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extconversion"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extruntime"
	"github.com/steadybit/extension-kit/extutil"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultCallInterval  = "5s"
	minHeartbeatInterval = 5 * time.Second
)

type actionHttpAdapter[T any] struct {
	description action_kit_api.ActionDescription
	action      Action[T]
	rootPath    string
}

func newActionHttpAdapter[T any](action Action[T]) *actionHttpAdapter[T] {
	description := getDescriptionWithDefaults(action)
	adapter := &actionHttpAdapter[T]{
		description: description,
		action:      action,
		rootPath:    fmt.Sprintf("/%s", description.Id),
	}
	if adapter.hasQueryMetric() {
		if adapter.description.Metrics == nil {
			log.Fatal().Msgf("ActionWithMetricQuery is implemented but description.Metrics is nil.")
		}
		if adapter.description.Metrics.Query == nil {
			log.Fatal().Msgf("ActionWithMetricQuery is implemented but description.Metrics.Query is nil.")
		}
	}
	var hasFileParameter bool
	for _, parameter := range description.Parameters {
		if parameter.Type == action_kit_api.File {
			hasFileParameter = true
			break
		}
	}
	if hasFileParameter && !adapter.hasStop() {
		log.Fatal().Msgf("Actions using a parameter of type 'file' need to implement ActionWithStop.")
	}
	if description.TimeControl == action_kit_api.TimeControlInternal && !adapter.hasStatus() {
		log.Fatal().Msgf("Actions using TimeControl 'Internal' need to implement ActionWithStatus.")
	}
	return adapter
}

func (a *actionHttpAdapter[T]) handleGetDescription(w http.ResponseWriter, _ *http.Request, _ []byte) {
	exthttp.WriteBody(w, a.description)
}

func (a *actionHttpAdapter[T]) handlePrepare(w http.ResponseWriter, r *http.Request, body []byte) {
	prepareActionRequestBody := parseRequestAndHandleFiles(w, r, body)
	if prepareActionRequestBody == nil {
		return
	}
	state := a.action.NewEmptyState()
	result, err := a.action.Prepare(r.Context(), &state, *prepareActionRequestBody)
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
		exthttp.WriteError(w, extension_kit.ToError("Please modify the state using the given state pointer.", err))
	}

	unameInformation := extruntime.GetUnameInformation()
	if unameInformation != "" {
		if result.Messages == nil {
			result.Messages = extutil.Ptr([]action_kit_api.Message{})
		}
		*result.Messages = append(*result.Messages, action_kit_api.Message{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: unameInformation,
		})
	}


	var convertedState action_kit_api.ActionState
	err = extconversion.Convert(state, &convertedState)
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to encode action state.", err))
		return
	}
	result.State = convertedState

	if a.description.Stop != nil {
		err = statePersister.PersistState(r.Context(), &state_persister.PersistedState{ExecutionId: prepareActionRequestBody.ExecutionId, ActionId: a.description.Id, State: convertedState})
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to persist action state.", err))
			return
		}
	}
	exthttp.WriteBody(w, result)
}

func parseRequestAndHandleFiles(w http.ResponseWriter, r *http.Request, body []byte) *action_kit_api.PrepareActionRequestBody {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to parse multipart request body.", err))
			return nil
		}
		prepareActionRequest := parsePrepareActionRequestBody(w, []byte(r.MultipartForm.Value["request"][0]))
		folder := fmt.Sprintf("/tmp/steadybit/%v", prepareActionRequest.ExecutionId)
		err = os.MkdirAll(folder, 0755)
		for parameterName, fileHeaders := range r.MultipartForm.File {
			if len(fileHeaders) > 1 {
				exthttp.WriteError(w, extension_kit.ToError(fmt.Sprintf("Too many Fileheaders for parameter %s.", parameterName), err))
				return nil
			}
			filename := folder + "/" + filepath.Base(fileHeaders[0].Filename)
			log.Debug().Msgf("Save File: Parameter %s, File %s", parameterName, filename)
			err := saveFile(filename, fileHeaders[0])
			if err != nil {
				exthttp.WriteError(w, extension_kit.ToError(fmt.Sprintf("Failed to save file %s.", filename), err))
				return nil
			}
			prepareActionRequest.Config[parameterName] = filename
		}
		return prepareActionRequest
	} else {
		return parsePrepareActionRequestBody(w, body)
	}
}

func saveFile(filename string, fileHeader *multipart.FileHeader) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Error().Err(err).Msgf("Failed to close file %s", file.Name())
		}
	}(file)

	open, err := fileHeader.Open()
	if err != nil {
		return err
	}
	buffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(buffer, open); err != nil {
		return err
	}
	if _, err = file.Write(buffer.Bytes()); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	return nil
}

func parsePrepareActionRequestBody(w http.ResponseWriter, request []byte) *action_kit_api.PrepareActionRequestBody {
	var parsedBody action_kit_api.PrepareActionRequestBody
	err := json.Unmarshal(request, &parsedBody)
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to parse request body.", err))
		return nil
	}
	return &parsedBody
}

func (a *actionHttpAdapter[T]) handleStart(w http.ResponseWriter, r *http.Request, body []byte) {
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
			exthttp.WriteError(w, extension_kit.ToError("Failed to start action.", err))
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
			if interval < minHeartbeatInterval {
				interval = minHeartbeatInterval
			}
			if err == nil {
				monitorHeartbeat(parsedBody.ExecutionId, interval, interval*4)
			}
		}
	}
	exthttp.WriteBody(w, result)
}

func (a *actionHttpAdapter[T]) hasStatus() bool {
	_, ok := a.action.(ActionWithStatus[T])
	return ok
}

func (a *actionHttpAdapter[T]) handleStatus(w http.ResponseWriter, r *http.Request, body []byte) {
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
				Status: extutil.Ptr(action_kit_api.Errored),
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

func (a *actionHttpAdapter[T]) hasStop() bool {
	_, ok := a.action.(ActionWithStop[T])
	return ok
}

func (a *actionHttpAdapter[T]) handleStop(w http.ResponseWriter, r *http.Request, body []byte) {
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
			exthttp.WriteError(w, extension_kit.ToError("Failed to stop action.", err))
		}
		return
	}

	folder := fmt.Sprintf("/tmp/steadybit/%v", parsedBody.ExecutionId)
	_, err = os.Stat(folder)
	if !os.IsNotExist(err) {
		err = os.RemoveAll(folder)
		if err != nil {
			log.Error().Msgf("Could not remove directory '%s'", folder)
		} else {
			log.Debug().Msgf("Directory '%s' removed successfully", folder)
		}
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

func (a *actionHttpAdapter[T]) hasQueryMetric() bool {
	_, ok := a.action.(ActionWithMetricQuery[T])
	return ok
}

func (a *actionHttpAdapter[T]) handleQueryMetric(w http.ResponseWriter, r *http.Request, body []byte) {
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

func (a *actionHttpAdapter[T]) registerHandlers() {

	exthttp.RegisterHttpHandler(a.rootPath, a.handleGetDescription)
	exthttp.RegisterHttpHandler(a.description.Prepare.Path, a.handlePrepare)
	exthttp.RegisterHttpHandler(a.description.Start.Path, a.handleStart)
	if a.hasStatus() || a.hasStop() {
		// If the action has a stop,  we augment a status endpoint. It is used to report stops by extension.
		exthttp.RegisterHttpHandler(a.description.Status.Path, a.handleStatus)
	}
	if a.hasStop() {
		exthttp.RegisterHttpHandler(a.description.Stop.Path, a.handleStop)
	}
	if a.hasQueryMetric() {
		exthttp.RegisterHttpHandler(a.description.Metrics.Query.Endpoint.Path, a.handleQueryMetric)
	}
}

// getDescriptionWithDefaults wraps the action description and adds default paths and methods for prepare, start, status, stop and metrics.
func getDescriptionWithDefaults[T any](action Action[T]) action_kit_api.ActionDescription {
	description := action.Describe()
	if description.Prepare.Path == "" {
		description.Prepare.Path = fmt.Sprintf("/%s/prepare", description.Id)
	}
	if description.Prepare.Method == "" {
		description.Prepare.Method = action_kit_api.POST
	}
	if description.Start.Path == "" {
		description.Start.Path = fmt.Sprintf("/%s/start", description.Id)
	}
	if description.Start.Method == "" {
		description.Start.Method = action_kit_api.POST
	}
	if _, ok := action.(ActionWithStop[T]); ok && description.Stop == nil {
		description.Stop = &action_kit_api.MutatingEndpointReference{}
	}

	if description.Stop != nil {
		if description.Stop.Path == "" {
			description.Stop.Path = fmt.Sprintf("/%s/stop", description.Id)
		}
		if description.Stop.Method == "" {
			description.Stop.Method = action_kit_api.POST
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
			description.Status.Method = action_kit_api.POST
		}
		if description.Status.CallInterval == nil || *description.Status.CallInterval == "" {
			description.Status.CallInterval = extutil.Ptr(defaultCallInterval)
		}
	}

	if description.Metrics != nil && description.Metrics.Query != nil {
		if description.Metrics.Query.Endpoint.Path == "" {
			description.Metrics.Query.Endpoint.Path = fmt.Sprintf("/%s/query", description.Id)
		}
		if description.Metrics.Query.Endpoint.Method == "" {
			description.Metrics.Query.Endpoint.Method = action_kit_api.POST
		}
	}
	return description
}
