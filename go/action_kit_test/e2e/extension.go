// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
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
	corev1 "k8s.io/api/core/v1"
	"os"
	"os/exec"
	"time"
)

type Extension struct {
	client *resty.Client
	stop   func() error
}

func (e *Extension) DiscoverTargets(targetId string) ([]discovery_kit_api.Target, error) {
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

func (e *Extension) RunAction(actionId string, target action_kit_api.Target, config interface{}) (ActionExecution, error) {
	actions, err := e.describeActions()
	if err != nil {
		return ActionExecution{}, fmt.Errorf("failed to get action descriptions: %w", err)
	}
	for _, action := range actions {
		if action.Id == actionId {
			return e.execAction(action, target, config)
		}
	}
	return ActionExecution{}, fmt.Errorf("action not found: %s", actionId)
}

func (e *Extension) listDiscoveries() (discovery_kit_api.DiscoveryList, error) {
	var list discovery_kit_api.DiscoveryList
	res, err := e.client.R().SetResult(&list).Get("/")
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
	res, err := e.client.R().SetResult(&description).Execute(string(endpoint.Method), endpoint.Path)
	if err != nil {
		return description, fmt.Errorf("failed to get discovery description: %w", err)
	}
	if !res.IsSuccess() {
		return description, fmt.Errorf("failed to get discovery description: %d", res.StatusCode())
	}
	return description, nil
}

func (e *Extension) discoverTargets(discovery discovery_kit_api.DiscoveryDescription) ([]discovery_kit_api.Target, error) {
	var targets discovery_kit_api.DiscoveredTargets
	res, err := e.client.R().SetResult(&targets).Execute(string(discovery.Discover.Method), discovery.Discover.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to discover targets: %w", err)
	}
	if !res.IsSuccess() {
		return nil, fmt.Errorf("failed to discover targets: %d", res.StatusCode())
	}
	return targets.Targets, nil
}

func (e *Extension) listActions() (action_kit_api.ActionList, error) {
	var list action_kit_api.ActionList
	res, err := e.client.R().SetResult(&list).Get("/")
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
	res, err := e.client.R().SetResult(&description).Execute(string(action.Method), action.Path)
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
	return nil
}

func (e *Extension) execAction(action action_kit_api.ActionDescription, target action_kit_api.Target, config interface{}) (ActionExecution, error) {
	executionId := uuid.New()

	state, duration, err := e.prepareAction(action, target, config, executionId)
	if err != nil {
		return ActionExecution{}, err
	}
	log.Info().Str("actionId", action.Id).
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

		var err error
		if action.Status != nil {
			state, err = e.actionStatus(ctx, action, executionId, state)
		} else {
			<-ctx.Done()
		}

		if action.Stop != nil {
			stopErr := e.stopAction(action, executionId, state)
			if stopErr != nil {
				ch <- errors.Join(err, stopErr)
			} else {
				log.Info().Str("actionId", action.Id).Msg("Action stopped")
			}
		} else {
			ch <- err
		}

		log.Info().Str("actionId", action.Id).Msg("Action ended")
		close(ch)
	}()

	return ActionExecution{
		ch:     ch,
		cancel: cancel,
	}, nil
}

func (e *Extension) prepareAction(action action_kit_api.ActionDescription, target action_kit_api.Target, config interface{}, executionId uuid.UUID) (action_kit_api.ActionState, time.Duration, error) {
	var duration time.Duration
	prepareBody := action_kit_api.PrepareActionRequestBody{
		ExecutionId: executionId,
		Target:      &target,
	}
	if err := extconversion.Convert(config, &prepareBody.Config); err != nil {
		return nil, duration, fmt.Errorf("failed to convert config: %w", err)
	}

	if action.TimeControl == action_kit_api.External {
		duration = time.Duration(prepareBody.Config["duration"].(float64) * float64(time.Millisecond))
	}

	var prepareResult action_kit_api.PrepareResult
	res, err := e.client.R().SetBody(prepareBody).SetResult(&prepareResult).Execute(string(action.Prepare.Method), action.Prepare.Path)
	if err != nil {
		return nil, duration, fmt.Errorf("failed to prepare action: %w", err)
	}
	logMessages(prepareResult.Messages)
	if !res.IsSuccess() {
		return nil, duration, fmt.Errorf("failed to prepare action: %d", res.StatusCode())
	}
	if prepareResult.Error != nil {
		return nil, duration, fmt.Errorf("action failed: %v", *prepareResult.Error)
	}

	return prepareResult.State, duration, nil
}

func (e *Extension) startAction(action action_kit_api.ActionDescription, executionId uuid.UUID, state action_kit_api.ActionState) (action_kit_api.ActionState, error) {
	startBody := action_kit_api.StartActionRequestBody{
		ExecutionId: executionId,
		State:       state,
	}
	var startResult action_kit_api.StartResult
	res, err := e.client.R().SetBody(startBody).SetResult(&startResult).Execute(string(action.Start.Method), action.Start.Path)
	if err != nil {
		return state, fmt.Errorf("failed to start action: %w", err)
	}
	logMessages(startResult.Messages)
	if !res.IsSuccess() {
		return state, fmt.Errorf("failed to start action: %d", res.StatusCode())
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
			res, err := e.client.R().SetBody(statusBody).SetResult(&statusResult).Execute(string(action.Status.Method), action.Status.Path)
			if err != nil {
				return state, fmt.Errorf("failed to get action status: %w", err)
			}
			logMessages(statusResult.Messages)
			if !res.IsSuccess() {
				return state, fmt.Errorf("failed to get action status: %d", res.StatusCode())
			}
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

func (e *Extension) stopAction(action action_kit_api.ActionDescription, executionId uuid.UUID, state action_kit_api.ActionState) error {
	stopBody := action_kit_api.StopActionRequestBody{
		ExecutionId: executionId,
		State:       state,
	}
	var stopResult action_kit_api.StopResult
	res, err := e.client.R().SetBody(stopBody).SetResult(&stopResult).Execute(string(action.Stop.Method), action.Stop.Path)
	if err != nil {
		return fmt.Errorf("failed to stop action: %w", err)
	}
	logMessages(stopResult.Messages)
	if !res.IsSuccess() {
		return fmt.Errorf("failed to stop action: %d", res.StatusCode())
	}
	if stopResult.Error != nil {
		return fmt.Errorf("action failed: %v", *stopResult.Error)
	}
	return nil
}

func logMessages(messages *action_kit_api.Messages) {
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

		event := log.WithLevel(level)
		if msg.Fields != nil {
			event.Fields(*msg.Fields)
		}
		if msg.Type != nil {
			event.Str("type", *msg.Type)
		}
		event.Msg(msg.Message)
	}
}

func startExtension(minikube *Minikube, image string) (*Extension, error) {
	if err := minikube.LoadImage(image); err != nil {
		return nil, err
	}

	ctx := context.Background()
	out, err := exec.CommandContext(ctx,
		"helm",
		"install",
		"--kube-context", minikube.Profile,
		"--namespace=default",
		"--wait",
		"--set", fmt.Sprintf("container.runtime=%s", minikube.Runtime),
		"--set", fmt.Sprintf("image.name=%s", image),
		"--set", "image.pullPolicy=Never",
		"extension-container",
		"../charts/steadybit-extension-container",
	).CombinedOutput()

	if err != nil {
		return nil, fmt.Errorf("failed to install helm chart: %s: %s", err, out)
	}

	tailCtx, tailCancel := context.WithCancel(context.Background())
	stopFwdCh := make(chan struct{})
	stop := func() error {
		tailCancel()
		close(stopFwdCh)
		out, err := exec.Command("helm", "uninstall", "--namespace=default", "--kube-context", minikube.Profile, "extension-container").CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to uninstall helm chart: %s: %s", err, out)
		}
		return nil
	}

	ctx, waitCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer waitCancel()
	var pods []corev1.Pod
	for {
		select {
		case <-ctx.Done():
			_ = stop()
			return nil, fmt.Errorf("extension pods did not start in time")
		case <-time.After(200 * time.Millisecond):
		}

		pods, err = minikube.ListPods(ctx, "default", "app.kubernetes.io/name=steadybit-extension-container")
		if err != nil {
			_ = stop()
			return nil, err
		}

		for _, pod := range pods {
			if err = minikube.WaitForPodPhase(pod.GetObjectMeta(), corev1.PodRunning, 30*time.Second); err != nil {
				_ = stop()
				return nil, err
			}
			go minikube.TailLog(tailCtx, pod.GetObjectMeta())
		}
		if len(pods) > 0 {
			break
		}
	}

	localPort, err := minikube.PortForward(pods[0].GetObjectMeta(), 8080, stopFwdCh)
	if err != nil {
		_ = stop()
		return nil, err
	}

	address := fmt.Sprintf("http://127.0.0.1:%d", localPort)
	client := resty.New().SetBaseURL(address)
	log.Info().Msgf("extension is available at %s", address)
	return &Extension{client: client, stop: stop}, nil
}

func createExtensionContainer() (string, error) {
	cmd := exec.Command("make", "container")
	cmd.Dir = ".."
	cmd.Stdout = &prefixWriter{prefix: "⚒️", w: os.Stdout}
	cmd.Stderr = &prefixWriter{prefix: "⚒️", w: os.Stdout}

	if err := cmd.Run(); err != nil {
		return "", err
	}
	return "docker.io/library/extension-container", nil
}
