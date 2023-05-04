// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"errors"
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	ametav1 "k8s.io/client-go/applyconfigurations/meta/v1"
)

type Nginx struct {
	minikube *Minikube
	Pod      metav1.Object
	Service  metav1.Object
}

func (n *Nginx) Deploy(podName string) error {
	pod, err := n.minikube.CreatePod(&acorev1.PodApplyConfiguration{
		TypeMetaApplyConfiguration: ametav1.TypeMetaApplyConfiguration{
			Kind:       extutil.Ptr("Pod"),
			APIVersion: extutil.Ptr("v1"),
		},
		ObjectMetaApplyConfiguration: &ametav1.ObjectMetaApplyConfiguration{
			Name:   &podName,
			Labels: map[string]string{"app": podName},
		},
		Spec: &acorev1.PodSpecApplyConfiguration{
			RestartPolicy: extutil.Ptr(corev1.RestartPolicyNever),
			Containers: []acorev1.ContainerApplyConfiguration{
				{
					Name:  extutil.Ptr("nginx"),
					Image: extutil.Ptr("nginx:stable-alpine"),
					Ports: []acorev1.ContainerPortApplyConfiguration{
						{
							ContainerPort: extutil.Ptr(int32(80)),
						},
					},
				},
			},
		},
		Status: nil,
	})
	if err != nil {
		return err
	}
	n.Pod = pod

	service, err := n.minikube.CreateService(&acorev1.ServiceApplyConfiguration{
		TypeMetaApplyConfiguration: ametav1.TypeMetaApplyConfiguration{
			Kind:       extutil.Ptr("Service"),
			APIVersion: extutil.Ptr("v1"),
		},
		ObjectMetaApplyConfiguration: &ametav1.ObjectMetaApplyConfiguration{
			Name:   extutil.Ptr("nginx"),
			Labels: map[string]string{"app": podName},
		},
		Spec: &acorev1.ServiceSpecApplyConfiguration{
			Selector: n.Pod.GetLabels(),
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

func (n *Nginx) Target() (*action_kit_api.Target, error) {
	return NewContainerTarget(n.minikube, n.Pod, "nginx")
}

func (n *Nginx) IsReachable() error {
	client, err := n.minikube.NewRestClientForService(n.Service)
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

func (n *Nginx) CanReach(url string) error {
	out, err := n.minikube.Exec(n.Pod, "nginx", "curl", "--max-time", "2", url)
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}

func (n *Nginx) ContainerStatus() (*corev1.ContainerStatus, error) {
	return GetContainerStatus(n.minikube, n.Pod, "nginx")
}

func (n *Nginx) Delete() error {
	return errors.Join(
		n.minikube.DeletePod(n.Pod),
		n.minikube.DeleteService(n.Service),
	)
}
