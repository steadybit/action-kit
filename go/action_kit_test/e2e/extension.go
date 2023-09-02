// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"github.com/go-resty/resty/v2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	aclient "github.com/steadybit/action-kit/go/action_kit_test/client"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	dclient "github.com/steadybit/discovery-kit/go/discovery_kit_test/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Extension struct {
	Client    *resty.Client
	stop      func() error
	Pod       metav1.Object
	discovery dclient.DiscoveryAPI
	action    aclient.ActionAPI
}

func (e *Extension) DiscoverTargets(discoveryId string) ([]discovery_kit_api.Target, error) {
	return e.discovery.DiscoverTargets(discoveryId)
}

func (e *Extension) DiscoverEnrichmentData(discoveryId string) ([]discovery_kit_api.EnrichmentData, error) {
	return dclient.NewDiscoveryClient("/", e.Client).DiscoverEnrichmentData(discoveryId)
}

func (e *Extension) RunAction(actionId string, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext) (aclient.ActionExecution, error) {
	return e.action.RunAction(actionId, target, config, executionContext)
}
func (e *Extension) RunActionWithFiles(actionId string, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext, files []aclient.File) (aclient.ActionExecution, error) {
	return e.action.RunActionWithFiles(actionId, target, config, executionContext, files)
}
