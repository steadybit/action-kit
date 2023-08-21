// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"errors"
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	ametav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"testing"
	"time"
)

type NginxDeployment struct {
	Minikube   *Minikube
	Deployment metav1.Object
	Service    metav1.Object
	Pods       []corev1.Pod
}

func (n *NginxDeployment) Deploy(deploymentName string) error {
	desiredCount := int32(2)
	deploymentConfig := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			Labels: map[string]string{
				"app": deploymentName,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &desiredCount,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": deploymentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: deploymentName,
					Labels: map[string]string{
						"app": deploymentName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:stable-alpine",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									Protocol:      corev1.ProtocolTCP,
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}

	deployment, pods, err := n.Minikube.CreateDeployment(deploymentConfig)
	if err != nil {
		return err
	}
	n.Deployment = deployment
	n.Pods = pods

	service, err := n.Minikube.CreateService(&acorev1.ServiceApplyConfiguration{
		TypeMetaApplyConfiguration: ametav1.TypeMetaApplyConfiguration{
			Kind:       extutil.Ptr("Service"),
			APIVersion: extutil.Ptr("v1"),
		},
		ObjectMetaApplyConfiguration: &ametav1.ObjectMetaApplyConfiguration{
			Name:   extutil.Ptr("nginx"),
			Labels: map[string]string{"app": deploymentName},
		},
		Spec: &acorev1.ServiceSpecApplyConfiguration{
			Type:     extutil.Ptr(corev1.ServiceTypeLoadBalancer),
			Selector: n.Deployment.GetLabels(),
			Ports: []acorev1.ServicePortApplyConfiguration{
				{
					Port:     extutil.Ptr(int32(80)),
					Protocol: extutil.Ptr(corev1.ProtocolTCP),
				},
			},
		},
	})
	if err != nil {
		return err
	}
	n.Service = service

	return nil
}

func (n *NginxDeployment) Target() (*action_kit_api.Target, error) {
	return NewContainerTarget(n.Minikube, n.Deployment, "nginx")
}

func (n *NginxDeployment) IsReachable() error {
	client, err := n.Minikube.NewRestClientForService(n.Service)
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = client.R().Get("/")
	if err != nil {
		return err
	}

	return nil
}

func (n *NginxDeployment) AssertIsReachable(t *testing.T, expected bool) {
	t.Helper()

	client, err := n.Minikube.NewRestClientForService(n.Service)
	require.NoError(t, err)
	defer client.Close()

	Retry(t, 20, 500*time.Millisecond, func(r *R) {
		_, err = client.R().Get("/")
		if expected && err != nil {
			r.Failed = true
			_, _ = fmt.Fprintf(r.Log, "expected nginx to be reachble, but was not: %s", err)
		} else if !expected && err == nil {
			r.Failed = true
			_, _ = fmt.Fprintf(r.Log, "expected nginx not to be reachble, but was")
		}
	})
}

func (n *NginxDeployment) ContainerStatus() (*corev1.ContainerStatus, error) {
	return GetContainerStatus(n.Minikube, n.Deployment, "nginx")
}

func (n *NginxDeployment) Delete() error {
	return errors.Join(
		n.Minikube.DeleteDeployment(n.Deployment),
		n.Minikube.DeleteService(n.Service),
	)
}
