package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extconversion"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"strings"
	"sync"
	"time"
)

type File struct {
	ParameterName string
	FileName      string
	Content       []byte
}

type ActionAPI interface {
	ListActions() (action_kit_api.ActionList, error)
	DescribeAction(ref action_kit_api.DescribingEndpointReference) (action_kit_api.ActionDescription, error)

	RunAction(actionId string, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext) (ActionExecution, error)
	RunActionWithFiles(actionId string, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext, files []File) (ActionExecution, error)
}

type ActionExecution interface {
	Wait() error
	Cancel() error
	Metrics() []action_kit_api.Metric
	Messages() []action_kit_api.Message
}

type clientImpl struct {
	client   *resty.Client
	rootPath string
	spec     *openapi3.T
}

func NewActionClient(rootPath string, client *resty.Client) ActionAPI {
	spec, _ := action_kit_api.GetSwagger()
	return &clientImpl{
		client:   client,
		rootPath: rootPath,
		spec:     spec,
	}
}

func (c *clientImpl) ListActions() (action_kit_api.ActionList, error) {
	var list action_kit_api.ActionList
	err := c.executeAndValidate(action_kit_api.DescribingEndpointReference{Path: c.rootPath}, &list, "ActionList")
	return list, err
}

func (c *clientImpl) DescribeAction(ref action_kit_api.DescribingEndpointReference) (action_kit_api.ActionDescription, error) {
	var description action_kit_api.ActionDescription
	err := c.executeAndValidate(ref, &description, "ActionDescription")
	return description, err
}

type actionExecutionImpl struct {
	ch            <-chan error
	cancel        context.CancelFunc
	metrics       []action_kit_api.Metric
	metricsMutex  sync.RWMutex
	messages      []action_kit_api.Message
	messagesMutex sync.RWMutex
}

func (a *actionExecutionImpl) Wait() error {
	return <-a.ch
}

func (a *actionExecutionImpl) Cancel() error {
	if a.cancel != nil {
		a.cancel()
	}
	if a.ch != nil {
		for err := range a.ch {
			return err
		}
	}
	return nil
}

func (a *actionExecutionImpl) appendMetrics(metrics []action_kit_api.Metric) {
	a.metricsMutex.Lock()
	a.metrics = append(a.metrics, metrics...)
	a.metricsMutex.Unlock()
}

func (a *actionExecutionImpl) Metrics() []action_kit_api.Metric {
	a.metricsMutex.RLock()
	result := make([]action_kit_api.Metric, len(a.metrics))
	copy(result, a.metrics)
	a.metricsMutex.RUnlock()
	return result
}

func (a *actionExecutionImpl) appendMessages(messages []action_kit_api.Message) {
	a.messagesMutex.Lock()
	a.messages = append(a.messages, messages...)
	a.messagesMutex.Unlock()
}

func (a *actionExecutionImpl) Messages() []action_kit_api.Message {
	a.messagesMutex.RLock()
	result := make([]action_kit_api.Message, len(a.messages))
	copy(result, a.messages)
	a.messagesMutex.RUnlock()
	return result
}

func (c *clientImpl) RunAction(actionId string, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext) (ActionExecution, error) {
	return c.RunActionWithFiles(actionId, target, config, executionContext, nil)
}

func (c *clientImpl) RunActionWithFiles(actionId string, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext, files []File) (ActionExecution, error) {
	actionList, err := c.ListActions()
	if err != nil {
		return &actionExecutionImpl{}, err
	}

	for _, action := range actionList.Actions {
		description, err := c.DescribeAction(action)
		if err != nil {
			return &actionExecutionImpl{}, err
		}

		if description.Id == actionId {
			return c.runAction(description, target, config, executionContext, files)
		}
	}

	return &actionExecutionImpl{}, fmt.Errorf("action with id %s not found", actionId)
}

func (c *clientImpl) runAction(action action_kit_api.ActionDescription, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext, files []File) (ActionExecution, error) {
	executionId := uuid.New()

	state, duration, err := c.prepareAction(action, target, config, executionId, executionContext, files)
	if err != nil {
		return &actionExecutionImpl{}, err
	}
	log.Info().Str("actionId", action.Id).
		Stringer("executionId", executionId).
		Interface("config", config).
		Interface("state", state).
		Msg("Action prepared")

	state, err = c.startAction(action, executionId, state)
	if err != nil {
		if action.Stop != nil {
			_ = c.stopAction(action, executionId, state, nil, nil)
		}
		return &actionExecutionImpl{}, err
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
	actionExecution := &actionExecutionImpl{ch: ch, cancel: cancel, metrics: nil, metricsMutex: sync.RWMutex{}, messages: nil, messagesMutex: sync.RWMutex{}}

	go func() {
		defer cancel()
		defer close(ch)

		var err error
		if action.Status != nil {
			state, err = c.actionStatus(ctx, action, executionId, state, actionExecution.appendMetrics, actionExecution.appendMessages)
		} else {
			<-ctx.Done()
		}

		if action.Stop != nil {
			stopErr := c.stopAction(action, executionId, state, actionExecution.appendMetrics, actionExecution.appendMessages)
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

	return actionExecution, nil
}

func (c *clientImpl) prepareAction(action action_kit_api.ActionDescription, target *action_kit_api.Target, config interface{}, executionId uuid.UUID, executionContext *action_kit_api.ExecutionContext, files []File) (action_kit_api.ActionState, time.Duration, error) {
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
	var err error
	if len(files) == 0 {
		err = c.executeWithBodyAndValidate(action.Prepare, prepareBody, &prepareResult, "PrepareResult")
	} else {
		err = c.executeWithMultipartAndValidate(action.Prepare, prepareBody, files, &prepareResult, "PrepareResult")
	}
	if err != nil {
		return nil, duration, fmt.Errorf("failed to prepare action: %w", err)
	}

	logMessages(executionId, prepareResult.Messages)

	if prepareResult.Error != nil {
		return nil, duration, toError(prepareResult.Error)
	}

	return prepareResult.State, duration, nil
}

func (c *clientImpl) startAction(action action_kit_api.ActionDescription, executionId uuid.UUID, state action_kit_api.ActionState) (action_kit_api.ActionState, error) {
	startBody := action_kit_api.StartActionRequestBody{
		ExecutionId: executionId,
		State:       state,
	}

	var startResult action_kit_api.StartResult
	err := c.executeWithBodyAndValidate(action.Start, startBody, &startResult, "StartResult")
	if err != nil {
		return state, fmt.Errorf("failed to start action: %w", err)
	}

	logMessages(executionId, startResult.Messages)

	if startResult.Error != nil {
		return state, toError(startResult.Error)
	}

	if startResult.State != nil {
		state = *startResult.State
	}
	return state, nil
}

func (c *clientImpl) actionStatus(ctx context.Context, action action_kit_api.ActionDescription, executionId uuid.UUID, state action_kit_api.ActionState, metrics func(metrics []action_kit_api.Metric), messages func(messages []action_kit_api.Message)) (action_kit_api.ActionState, error) {
	interval, err := time.ParseDuration(*action.Status.CallInterval)
	if err != nil {
		interval = 1 * time.Second
	}

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
			err := c.executeWithBodyAndValidate(action_kit_api.MutatingEndpointReference{Method: action.Status.Method, Path: action.Status.Path}, statusBody, &statusResult, "StatusResult")
			if err != nil {
				return state, fmt.Errorf("failed to get action status: %w", err)
			}

			logMessages(executionId, statusResult.Messages)
			if statusResult.Metrics != nil {
				metrics(*statusResult.Metrics)
			}
			if statusResult.Messages != nil {
				messages(*statusResult.Messages)
			}

			if statusResult.State != nil {
				state = *statusResult.State
			}
			if statusResult.Error != nil {
				return state, toError(statusResult.Error)
			}

			log.Info().Str("actionId", action.Id).Bool("completed", statusResult.Completed).Msg("Action status")
			if statusResult.Completed {
				return state, nil
			}
		}
	}
}

func toError(err *action_kit_api.ActionKitError) error {
	if err == nil {
		return nil
	}
	var sb strings.Builder
	if err.Status != nil {
		sb.WriteString("[")
		sb.WriteString(string(*err.Status))
		sb.WriteString("] ")
	}
	sb.WriteString(err.Title)
	if err.Detail != nil {
		sb.WriteString(": ")
		sb.WriteString(*err.Detail)
	}
	return fmt.Errorf(sb.String())
}

func (c *clientImpl) stopAction(action action_kit_api.ActionDescription, executionId uuid.UUID, state action_kit_api.ActionState, metrics func(metrics []action_kit_api.Metric), messages func(messages []action_kit_api.Message)) error {
	stopBody := action_kit_api.StopActionRequestBody{
		ExecutionId: executionId,
		State:       state,
	}
	var stopResult action_kit_api.StopResult
	if err := c.executeWithBodyAndValidate(*action.Stop, stopBody, &stopResult, "StopResult"); err != nil {
		return fmt.Errorf("failed to stop action: %w", err)
	}

	logMessages(executionId, stopResult.Messages)
	if metrics != nil && stopResult.Metrics != nil {
		metrics(*stopResult.Metrics)
	}
	if messages != nil && stopResult.Messages != nil {
		messages(*stopResult.Messages)
	}

	if stopResult.Error != nil {
		return toError(stopResult.Error)
	}
	return nil
}

func (c *clientImpl) executeAndValidate(ref action_kit_api.DescribingEndpointReference, result interface{}, schemaName string) error {
	method, path := getMethodAndPath(ref)
	res, err := c.client.R().SetResult(result).Execute(method, path)
	if err != nil {
		return fmt.Errorf("%s %s failed: %w", method, path, err)
	}
	if !res.IsSuccess() {
		return fmt.Errorf("%s %s failed: %d", method, path, res.StatusCode())
	}
	return c.validateResponseBody(schemaName, res)
}

func (c *clientImpl) executeWithBodyAndValidate(ref action_kit_api.MutatingEndpointReference, body, result interface{}, schemaName string) error {
	method, path := getMethodAndPath2(ref)
	res, err := c.client.R().SetBody(body).SetResult(result).Execute(method, path)
	if err != nil {
		return fmt.Errorf("%s %s failed: %w", method, path, err)
	}
	if !res.IsSuccess() {
		return fmt.Errorf("%s %s failed: %d %s", method, path, res.StatusCode(), res.Body())
	}
	return c.validateResponseBody(schemaName, res)
}

func (c *clientImpl) executeWithMultipartAndValidate(ref action_kit_api.MutatingEndpointReference, body interface{}, files []File, result interface{}, schemaName string) error {
	prepareBodyJson, err := c.client.JSONMarshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshall prepare request action: %w", err)
	}
	request := c.client.R().
		SetMultipartFormData(map[string]string{
			"request": string(prepareBodyJson),
		}).
		SetResult(&result)
	for _, file := range files {
		request.SetMultipartField(file.ParameterName, file.FileName, "application/octet-stream", bytes.NewReader(file.Content))
	}

	method, path := getMethodAndPath2(ref)
	res, err := request.Execute(method, path)
	if err != nil {
		return fmt.Errorf("%s %s failed: %w", method, path, err)
	}
	if !res.IsSuccess() {
		return fmt.Errorf("%s %s failed: %d %s", method, path, res.StatusCode(), res.Body())
	}
	return c.validateResponseBody(schemaName, res)
}

func (c *clientImpl) validateResponseBody(name string, res *resty.Response) error {
	if c.spec == nil || name == "" {
		return nil
	}

	schema, ok := c.spec.Components.Schemas[name]
	if !ok {
		return fmt.Errorf("component schema '%s' not found", name)
	}

	var decoded interface{}
	dec := json.NewDecoder(bytes.NewReader(res.Body()))
	dec.UseNumber()
	err := dec.Decode(&decoded)
	if err != nil {
		return fmt.Errorf("error decoding response body: %w", err)
	}

	err = schema.Value.VisitJSON(decoded, openapi3.VisitAsResponse())
	if err != nil {
		return fmt.Errorf("invalid response for %s using schema '%s': %w", res.Request.URL, name, err)
	}
	return nil
}

func getMethodAndPath(ref action_kit_api.DescribingEndpointReference) (string, string) {
	method := "GET"
	if len(ref.Method) > 0 {
		method = cases.Upper(language.English).String(string(ref.Method))
	}
	return method, ref.Path
}

func getMethodAndPath2(ref action_kit_api.MutatingEndpointReference) (string, string) {
	method := "GET"
	if len(ref.Method) > 0 {
		method = cases.Upper(language.English).String(string(ref.Method))
	}
	return method, ref.Path
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
