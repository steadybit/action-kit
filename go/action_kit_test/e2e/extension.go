// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"context"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	aclient "github.com/steadybit/action-kit/go/action_kit_test/client"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	dclient "github.com/steadybit/discovery-kit/go/discovery_kit_test/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Extension struct {
	Client *resty.Client
	Pod    metav1.Object

	install             func(values map[string]string) error
	uninstall           func() error
	terminateConnection func()
	connect             func(ctx context.Context) (uint16, metav1.Object, error)
}

func (e *Extension) stop() error {
	if e.terminateConnection != nil {
		e.terminateConnection()
	}
	return e.uninstall()
}

func (e *Extension) ResetConfig() error {
	return e.Reconfigure(nil)
}

func (e *Extension) Reconfigure(values map[string]string) error {
	if e.terminateConnection != nil {
		e.terminateConnection()
	}
	e.Client = nil
	e.Pod = nil

	success := false
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if !success {
			cancel()
		}
	}()

	if err := e.install(values); err != nil {
		return err
	}

	port, pod, err := e.connect(ctx)
	if err != nil {
		return err
	}

	success = true
	e.terminateConnection = cancel
	e.Client = resty.New().SetBaseURL(fmt.Sprintf("http://127.0.0.1:%d", port))
	e.Pod = pod
	return nil
}

func (e *Extension) DiscoverTargets(discoveryId string) ([]discovery_kit_api.Target, error) {
	return dclient.NewDiscoveryClient("/", e.Client).DiscoverTargets(discoveryId)
}

func (e *Extension) DiscoverEnrichmentData(discoveryId string) ([]discovery_kit_api.EnrichmentData, error) {
	return dclient.NewDiscoveryClient("/", e.Client).DiscoverEnrichmentData(discoveryId)
}

func (e *Extension) RunAction(actionId string, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext) (aclient.ActionExecution, error) {
	return aclient.NewActionClient("/", e.Client).RunAction(actionId, target, config, executionContext)
}
func (e *Extension) RunActionWithFiles(actionId string, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext, files []aclient.File) (aclient.ActionExecution, error) {
	return aclient.NewActionClient("/", e.Client).RunActionWithFiles(actionId, target, config, executionContext, files)
}
