// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-kit/extconversion"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sync"
	"time"
)

type Extension struct {
	Client *resty.Client
	stop   func() error
	Pod    metav1.Object
}

var (
	Metrics = sync.Map{}
)

func (e *Extension) DiscoverTargets(targetId string) (*discovery_kit_api.DiscoveryData, error) {
	discoveries, err := e.describeDiscoveries()
	if err != nil {
		return nil, fmt.Errorf("failed to get discovery descriptions: %w", err)
	}
	for _, discovery := range discoveries {
		if discovery.Id == targetId {
			return e.discoverTargets(discovery)
		}
	}
	return nil, fmt.Errorf("discovery not found: %s", targetId)
}

func (e *Extension) RunAction(actionId string, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext) (ActionExecution, error) {
	return e.RunActionWithFiles(actionId, target, config, executionContext, nil)
}

type File struct {
	ParameterName string
	FileName      string
	Content       []byte
}

func (e *Extension) RunActionWithFiles(actionId string, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext, files []File) (ActionExecution, error) {
	actions, err := e.describeActions()
	if err != nil {
		return ActionExecution{}, fmt.Errorf("failed to get action descriptions: %w", err)
	}
	for _, action := range actions {
		if action.Id == actionId {
			return e.execAction(action, target, config, executionContext, files)
		}
	}
	return ActionExecution{}, fmt.Errorf("action not found: %s", actionId)
}

func (e *Extension) listDiscoveries() (discovery_kit_api.DiscoveryList, error) {
	var list discovery_kit_api.DiscoveryList
	res, err := e.Client.R().SetResult(&list).Get("/")
	if err != nil {
		return list, fmt.Errorf("failed to get discovery list: %w", err)
	}
	if !res.IsSuccess() {
		return list, fmt.Errorf("failed to get discovery list: %d", res.StatusCode())
	}
	return list, nil
}

func (e *Extension) describeDiscoveries() ([]discovery_kit_api.DiscoveryDescription, error) {
	list, err := e.listDiscoveries()
	if err != nil {
		return nil, fmt.Errorf("failed to get discovery descriptions: %w", err)
	}

	discoveries := make([]discovery_kit_api.DiscoveryDescription, 0, len(list.Discoveries))
	for _, discovery := range list.Discoveries {
		description, err := e.describeDiscovery(discovery)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to describe discovery")
		}
		discoveries = append(discoveries, description)
	}
	return discoveries, nil
}

func (e *Extension) describeDiscovery(endpoint discovery_kit_api.DescribingEndpointReference) (discovery_kit_api.DiscoveryDescription, error) {
	var description discovery_kit_api.DiscoveryDescription
	res, err := e.Client.R().SetResult(&description).Execute(cases.Upper(language.English).String(string(endpoint.Method)), endpoint.Path)
	if err != nil {
		return description, fmt.Errorf("failed to get discovery description: %w", err)
	}
	if !res.IsSuccess() {
		return description, fmt.Errorf("failed to get discovery description: %d", res.StatusCode())
	}
	return description, nil
}

func (e *Extension) discoverTargets(discovery discovery_kit_api.DiscoveryDescription) (*discovery_kit_api.DiscoveryData, error) {
	var result discovery_kit_api.DiscoveryData
	res, err := e.Client.R().SetResult(&result).Execute(cases.Upper(language.English).String(string(discovery.Discover.Method)), discovery.Discover.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to discover targets: %w", err)
	}
	if !res.IsSuccess() {
		return nil, fmt.Errorf("failed to discover targets: %d", res.StatusCode())
	}
	return &result, nil
}

func (e *Extension) listActions() (action_kit_api.ActionList, error) {
	var list action_kit_api.ActionList
	res, err := e.Client.R().SetResult(&list).Get("/")
	if err != nil {
		return list, fmt.Errorf("failed to get action list: %w", err)
	}
	if !res.IsSuccess() {
		return list, fmt.Errorf("failed to get action list: %d", res.StatusCode())
	}
	return list, nil
}

func (e *Extension) describeActions() ([]action_kit_api.ActionDescription, error) {
	list, err := e.listActions()
	if err != nil {
		return nil, fmt.Errorf("failed to get action descriptions: %w", err)
	}

	actions := make([]action_kit_api.ActionDescription, 0, len(list.Actions))
	for _, action := range list.Actions {
		description, err := e.describeAction(action)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to describe action")
		}
		actions = append(actions, description)
	}
	return actions, nil
}

func (e *Extension) describeAction(action action_kit_api.DescribingEndpointReference) (action_kit_api.ActionDescription, error) {
	var description action_kit_api.ActionDescription
	res, err := e.Client.R().SetResult(&description).Execute(cases.Upper(language.English).String(string(action.Method)), action.Path)
	if err != nil {
		return description, fmt.Errorf("failed to get action description: %w", err)
	}
	if !res.IsSuccess() {
		return description, fmt.Errorf("failed to get action description: %d", res.StatusCode())
	}
	return description, nil
}

type ActionExecution struct {
	ch     <-chan error
	cancel context.CancelFunc
}

func (a *ActionExecution) Wait() error {
	return <-a.ch
}

func (a *ActionExecution) Cancel() error {
	if a.cancel != nil {
		a.cancel()
	}
	for err := range a.ch {
		return err
	}
	return nil
}

func (e *Extension) execAction(action action_kit_api.ActionDescription, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext, files []File) (ActionExecution, error) {
	executionId := uuid.New()

	state, duration, err := e.prepareAction(action, target, config, executionId, executionContext, files)
	if err != nil {
		return ActionExecution{}, err
	}
	log.Info().Str("actionId", action.Id).
		Stringer("executionId", executionId).
		Interface("config", config).
		Interface("state", state).
		Msg("Action prepared")

	state, err = e.startAction(action, executionId, state)
	if err != nil {
		if action.Stop != nil {
			_ = e.stopAction(action, executionId, state)
		}
		return ActionExecution{}, err
	}
	log.Info().Str("actionId", action.Id).
		Stringer("executionId", executionId).
		Interface("state", state).
		Msg("Action started")

	ch := make(chan error)
	var ctx context.Context
	var cancel context.CancelFunc
	if duration > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), duration)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	go func() {
		defer cancel()
		defer close(ch)

		var err error
		if action.Status != nil {
			state, err = e.actionStatus(ctx, action, executionId, state)
		} else {
			<-ctx.Done()
		}

		if action.Stop != nil {
			stopErr := e.stopAction(action, executionId, state)
			if stopErr != nil {
				err = errors.Join(err, stopErr)
			} else {
				log.Info().Str("actionId", action.Id).Stringer("executionId", executionId).Msg("Action stopped")
			}
		}

		if err != nil {
			log.Warn().Str("actionId", action.Id).Stringer("executionId", executionId).Err(err).Msg("Action ended with error")
			ch <- err
		} else {
			log.Info().Str("actionId", action.Id).Stringer("executionId", executionId).Msg("Action ended")
		}
	}()

	return ActionExecution{
		ch:     ch,
		cancel: cancel,
	}, nil
}

func (e *Extension) prepareAction(action action_kit_api.ActionDescription, target *action_kit_api.Target, config interface{}, executionId uuid.UUID, executionContext *action_kit_api.ExecutionContext, files []File) (action_kit_api.ActionState, time.Duration, error) {
	var duration time.Duration
	prepareBody := action_kit_api.PrepareActionRequestBody{
		ExecutionId:      executionId,
		Target:           target,
		ExecutionContext: executionContext,
	}
	if err := extconversion.Convert(config, &prepareBody.Config); err != nil {
		return nil, duration, fmt.Errorf("failed to convert config: %w", err)
	}

	if action.TimeControl == action_kit_api.TimeControlExternal {
		duration = time.Duration(prepareBody.Config["duration"].(float64) * float64(time.Millisecond))
	}

	var prepareResult action_kit_api.PrepareResult
	var res *resty.Response
	var err error
	if len(files) == 0 {
		res, err = e.Client.R().
			SetBody(prepareBody).
			SetResult(&prepareResult).
			Execute(cases.Upper(language.English).String(string(action.Prepare.Method)), action.Prepare.Path)
	} else {
		var prepareBodyJson []byte
		prepareBodyJson, err = e.Client.JSONMarshal(prepareBody)
		if err != nil {
			return nil, duration, fmt.Errorf("failed to marshall prepare request action: %w", err)
		}
		request := e.Client.R().
			SetMultipartFormData(map[string]string{
				"request": string(prepareBodyJson),
			}).
			SetResult(&prepareResult)
		for _, file := range files {
			request.SetMultipartField(file.ParameterName, file.FileName, "application/octet-stream", bytes.NewReader(file.Content))
		}
		res, err = request.Execute(cases.Upper(language.English).String(string(action.Prepare.Method)), action.Prepare.Path)
	}

	if err != nil {
		return nil, duration, fmt.Errorf("failed to prepare action: %w", err)
	}
	logMessages(executionId, prepareResult.Messages)
	if prepareResult.Error != nil {
		return nil, duration, fmt.Errorf("action failed: %v", *prepareResult.Error)
	}
	if !res.IsSuccess() {
		return nil, duration, fmt.Errorf("failed to prepare action: HTTP %d %s", res.StatusCode(), string(res.Body()))
	}

	return prepareResult.State, duration, nil
}

func (e *Extension) startAction(action action_kit_api.ActionDescription, executionId uuid.UUID, state action_kit_api.ActionState) (action_kit_api.ActionState, error) {
	startBody := action_kit_api.StartActionRequestBody{
		ExecutionId: executionId,
		State:       state,
	}
	var startResult action_kit_api.StartResult
	res, err := e.Client.R().SetBody(startBody).SetResult(&startResult).Execute(cases.Upper(language.English).String(string(action.Start.Method)), action.Start.Path)
	if err != nil {
		return state, fmt.Errorf("failed to start action: %w", err)
	}
	logMessages(executionId, startResult.Messages)
	if !res.IsSuccess() {
		return nil, fmt.Errorf("failed to start action: HTTP %d %s", res.StatusCode(), string(res.Body()))
	}
	if startResult.State != nil {
		state = *startResult.State
	}
	return state, nil
}

func (e *Extension) actionStatus(ctx context.Context, action action_kit_api.ActionDescription, executionId uuid.UUID, state action_kit_api.ActionState) (action_kit_api.ActionState, error) {
	interval, err := time.ParseDuration(*action.Status.CallInterval)
	if err != nil {
		interval = 1 * time.Second
	}

	Metrics.Store(action.Id, nil)
	for {
		select {
		case <-ctx.Done():
			return state, nil
		case <-time.After(interval):
			statusBody := action_kit_api.ActionStatusRequestBody{
				ExecutionId: executionId,
				State:       state,
			}
			var statusResult action_kit_api.StatusResult
			res, err := e.Client.R().SetBody(statusBody).SetResult(&statusResult).Execute(cases.Upper(language.English).String(string(action.Status.Method)), action.Status.Path)
			if err != nil {
				return state, fmt.Errorf("failed to get action status: %w", err)
			}
			if !res.IsSuccess() {
				return nil, fmt.Errorf("failed to get action state: HTTP %d %s", res.StatusCode(), string(res.Body()))
			}

			logMessages(executionId, statusResult.Messages)
			storeLatestMetrics(action.Id, statusResult.Metrics)

			if statusResult.State != nil {
				state = *statusResult.State
			}
			if statusResult.Error != nil {
				return state, fmt.Errorf("action failed: %v", *statusResult.Error)
			}

			log.Info().Str("actionId", action.Id).Bool("completed", statusResult.Completed).Msg("Action status")
			if statusResult.Completed {
				return state, nil
			}
		}
	}
}

func storeLatestMetrics(actionId string, metrics *[]action_kit_api.Metric) {
	if metrics == nil {
		return
	}
	value, ok := Metrics.Load(actionId)
	if !ok || value == nil {
		Metrics.Store(actionId, *metrics)
	} else {
		metricsStored := value.([]action_kit_api.Metric)
		metricsStored = append(metricsStored, *metrics...)
		Metrics.Store(actionId, metricsStored)
	}
}
func (e *Extension) GetMetrics(actionId string) []action_kit_api.Metric {
	value, ok := Metrics.Load(actionId)
	if !ok || value == nil {
		return nil
	}
	var result []action_kit_api.Metric
	result = append(result, value.([]action_kit_api.Metric)...)
	return result
}

func (e *Extension) stopAction(action action_kit_api.ActionDescription, executionId uuid.UUID, state action_kit_api.ActionState) error {
	stopBody := action_kit_api.StopActionRequestBody{
		ExecutionId: executionId,
		State:       state,
	}
	var stopResult action_kit_api.StopResult
	res, err := e.Client.R().SetBody(stopBody).SetResult(&stopResult).Execute(cases.Upper(language.English).String(string(action.Stop.Method)), action.Stop.Path)
	if err != nil {
		return fmt.Errorf("failed to stop action: %w", err)
	}
	logMessages(executionId, stopResult.Messages)
	storeLatestMetrics(action.Id, stopResult.Metrics)
	if !res.IsSuccess() {
		return fmt.Errorf("failed to stop action: HTTP %d %s", res.StatusCode(), string(res.Body()))
	}
	if stopResult.Error != nil {
		return fmt.Errorf("action failed: %v", *stopResult.Error)
	}
	return nil
}

func logMessages(executionId uuid.UUID, messages *action_kit_api.Messages) {
	if messages == nil {
		return
	}
	for _, msg := range *messages {
		level := zerolog.NoLevel
		if msg.Level != nil {
			level, _ = zerolog.ParseLevel(string(*msg.Level))
		}
		if level == zerolog.NoLevel {
			level = zerolog.InfoLevel
		}

		event := log.WithLevel(level).Stringer("executionId", executionId)
		if msg.Fields != nil {
			event.Fields(*msg.Fields)
		}
		if msg.Type != nil {
			event.Str("type", *msg.Type)
		}
		event.Msg(msg.Message)
	}
}
