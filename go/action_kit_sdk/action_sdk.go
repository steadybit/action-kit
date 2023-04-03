// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package action_kit_sdk

import (
	"context"
	"encoding/json"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extconversion"
	"github.com/steadybit/extension-kit/exthttp"
	"net/http"
)

type Action[T any] interface {
	NewEmptyState() T
	Describe() action_kit_api.ActionDescription
	Prepare(ctx context.Context, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error)
	Start(ctx context.Context, state T) (*action_kit_api.StartResult, error)
}
type ActionWithStatus[T any] interface {
	Action[T]
	Status(ctx context.Context, state T) (*action_kit_api.StatusResult, error)
}
type ActionWithStop[T any] interface {
	Action[T]
	Stop(ctx context.Context, state T) (*action_kit_api.StopResult, error)
}

func RegisterHttpHandlers[T any](a Action[T], basePath string) {
	actionDescription := a.Describe()

	exthttp.RegisterHttpHandler(basePath, exthttp.GetterAsHandler(a.Describe))
	exthttp.RegisterHttpHandler(actionDescription.Prepare.Path, wrapPrepare(a))
	exthttp.RegisterHttpHandler(actionDescription.Start.Path, wrapStart(a))
	if actionWithStatus, ok := a.(ActionWithStatus[T]); ok {
		if actionDescription.Status == nil {
			log.Fatal().Msgf("ActionWithStatus is implemented but actionDescription.Status is nil")
		}
		exthttp.RegisterHttpHandler(actionDescription.Status.Path, wrapStatus(actionWithStatus))
	}
	if actionWithStop, ok := a.(ActionWithStop[T]); ok {
		if actionDescription.Stop == nil {
			panic("ActionWithStop is implemented but actionDescription.Stop is nil")
		}
		exthttp.RegisterHttpHandler(actionDescription.Stop.Path, wrapStop(actionWithStop))
	}
}

func wrapPrepare[T any](action Action[T]) func(w http.ResponseWriter, r *http.Request, body []byte) {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {
		var parsedBody action_kit_api.PrepareActionRequestBody
		err := json.Unmarshal(body, &parsedBody)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to parse request body", err))
			return
		}
		result, err := action.Prepare(r.Context(), parsedBody)
		if err != nil {
			extensionError, isExtensionError := err.(extension_kit.ExtensionError)
			if isExtensionError {
				exthttp.WriteError(w, extensionError)
			} else {
				exthttp.WriteError(w, extension_kit.ToError("Failed to prepare.", err))
			}
			return
		}
		exthttp.WriteBody(w, result)
	}
}

func wrapStart[T any](action Action[T]) func(w http.ResponseWriter, r *http.Request, body []byte) {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {
		var parsedBody action_kit_api.StartActionRequestBody
		err := json.Unmarshal(body, &parsedBody)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to parse request body", err))
			return
		}
		state := action.NewEmptyState()
		err = extconversion.Convert(parsedBody.State, &state)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to parse state.", err))
			return
		}

		result, err := action.Start(r.Context(), state)
		if err != nil {
			extensionError, isExtensionError := err.(extension_kit.ExtensionError)
			if isExtensionError {
				exthttp.WriteError(w, extensionError)
			} else {
				exthttp.WriteError(w, extension_kit.ToError("Failed to start.", err))
			}
			return
		}
		exthttp.WriteBody(w, result)
	}
}

func wrapStatus[T any](action ActionWithStatus[T]) func(w http.ResponseWriter, r *http.Request, body []byte) {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {
		var parsedBody action_kit_api.ActionStatusRequestBody
		err := json.Unmarshal(body, &parsedBody)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to parse request body", err))
			return
		}
		state := action.NewEmptyState()
		err = extconversion.Convert(parsedBody.State, &state)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to parse state.", err))
			return
		}

		result, err := action.Status(r.Context(), state)
		if err != nil {
			extensionError, isExtensionError := err.(extension_kit.ExtensionError)
			if isExtensionError {
				exthttp.WriteError(w, extensionError)
			} else {
				exthttp.WriteError(w, extension_kit.ToError("Failed to read status.", err))
			}
			return
		}
		exthttp.WriteBody(w, result)
	}
}

func wrapStop[T any](action ActionWithStop[T]) func(w http.ResponseWriter, r *http.Request, body []byte) {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {
		var parsedBody action_kit_api.StopActionRequestBody
		err := json.Unmarshal(body, &parsedBody)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to parse request body", err))
			return
		}
		state := action.NewEmptyState()
		err = extconversion.Convert(parsedBody.State, &state)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to parse state.", err))
			return
		}

		result, err := action.Stop(r.Context(), state)
		if err != nil {
			extensionError, isExtensionError := err.(extension_kit.ExtensionError)
			if isExtensionError {
				exthttp.WriteError(w, extensionError)
			} else {
				exthttp.WriteError(w, extension_kit.ToError("Failed to stop.", err))
			}
			return
		}
		exthttp.WriteBody(w, result)
	}
}
