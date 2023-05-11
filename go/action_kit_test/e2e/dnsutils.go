// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"errors"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	ametav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"strings"
)

type DNSUtils struct {
	Minikube *Minikube
	Pod      metav1.Object
}

func (n *DNSUtils) Deploy(podName string) error {
	pod, err := n.Minikube.CreatePod(&acorev1.PodApplyConfiguration{
		TypeMetaApplyConfiguration: ametav1.TypeMetaApplyConfiguration{
			Kind:       extutil.Ptr("Pod"),
			APIVersion: extutil.Ptr("v1"),
		},
		ObjectMetaApplyConfiguration: &ametav1.ObjectMetaApplyConfiguration{
			Name:   &podName,
			Labels: map[string]string{"app": podName},
		},
		Spec: &acorev1.PodSpecApplyConfiguration{
			HostNetwork:   extutil.Ptr(true),
			RestartPolicy: extutil.Ptr(corev1.RestartPolicyNever),
			Containers: []acorev1.ContainerApplyConfiguration{
				{
					Name:    extutil.Ptr("dnsutils"),
					Image:   extutil.Ptr("tutum/dnsutils:latest"),
					Command: []string{"sh", "-c", "while true; do sleep 30; done;"},
					Ports:   []acorev1.ContainerPortApplyConfiguration{},
				},
			},
		},
		Status: nil,
	})
	if err != nil {
		return err
	}
	n.Pod = pod

	if err != nil {
		return err
	}

	return nil
}

func (n *DNSUtils) Target() (*action_kit_api.Target, error) {
	return NewContainerTarget(n.Minikube, n.Pod, "dnsutils")
}

func (n *DNSUtils) IsReachable() error {
	return n.CanReach("github.com")
}

func (n *DNSUtils) CanReach(url string) error {
	out, err := n.Minikube.Exec(n.Pod, "dnsutils", "nslookup", url)
	if strings.Contains(out, "can't find") {
		return errors.New("can't find " + url)
	}
	return err
}

func (n *DNSUtils) ContainerStatus() (*corev1.ContainerStatus, error) {
	return GetContainerStatus(n.Minikube, n.Pod, "dnsutils")
}

func (n *DNSUtils) Delete() error {
	return errors.Join(
		n.Minikube.DeletePod(n.Pod),
	)
}
