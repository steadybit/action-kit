// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"errors"
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	ametav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"strings"
	"testing"
	"time"
)

type Nginx struct {
	Minikube *Minikube
	Pod      metav1.Object
	Service  metav1.Object
}

func (n *Nginx) Deploy(podName string, opts ...func(c *acorev1.PodApplyConfiguration)) error {
	cfg := &acorev1.PodApplyConfiguration{
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
					Image: extutil.Ptr("docker.io/library/nginx:stable-alpine"),
					Ports: []acorev1.ContainerPortApplyConfiguration{
						{
							ContainerPort: extutil.Ptr(int32(80)),
						},
					},
				},
			},
		},
		Status: nil,
	}

	for _, fn := range opts {
		fn(cfg)
	}

	pod, err := n.Minikube.CreatePod(cfg)
	if err != nil {
		return err
	}
	n.Pod = pod

	service, err := n.Minikube.CreateService(&acorev1.ServiceApplyConfiguration{
		TypeMetaApplyConfiguration: ametav1.TypeMetaApplyConfiguration{
			Kind:       extutil.Ptr("Service"),
			APIVersion: extutil.Ptr("v1"),
		},
		ObjectMetaApplyConfiguration: &ametav1.ObjectMetaApplyConfiguration{
			Name:   extutil.Ptr("nginx"),
			Labels: map[string]string{"app": podName},
		},
		Spec: &acorev1.ServiceSpecApplyConfiguration{
			Type:     extutil.Ptr(corev1.ServiceTypeLoadBalancer),
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
	return NewContainerTarget(n.Minikube, n.Pod, "nginx")
}

func (n *Nginx) IsReachable() error {
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

func (n *Nginx) AssertIsReachable(t *testing.T, expected bool) {
	t.Helper()

	client, err := n.Minikube.NewRestClientForService(n.Service)
	require.NoError(t, err)
	defer client.Close()

	Retry(t, 8, 500*time.Millisecond, func(r *R) {
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

func (n *Nginx) CanReach(url string) error {
	out, err := n.Minikube.PodExec(n.Pod, "nginx", "curl", "--max-time", "2", url)
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}

func (n *Nginx) AssertCanReach(t *testing.T, url string, expected bool) {
	t.Helper()

	Retry(t, 8, 500*time.Millisecond, func(r *R) {
		err := n.CanReach(url)
		if expected && err != nil {
			r.Failed = true
			_, _ = fmt.Fprintf(r.Log, "expected '%s' to be reachble from nginx, but was not: %s", url, err)
		} else if !expected && err == nil {
			r.Failed = true
			_, _ = fmt.Fprintf(r.Log, "expecte '%s' not to be reachble from nginx, but was", url)
		}
	})
}

func (n *Nginx) AssertCannotReach(t *testing.T, url string, errContains string) {
	t.Helper()

	Retry(t, 8, 500*time.Millisecond, func(r *R) {
		err := n.CanReach(url)
		if err == nil {
			r.Failed = true
			_, _ = fmt.Fprintf(r.Log, "expected '%s' not to be reachble from nginx, but was", url)
		} else if !strings.Contains(err.Error(), errContains) {
			r.Failed = true
			_, _ = fmt.Fprintf(r.Log, "expected '%s' not to be reachble from nginx, with error containing '%s', but was '%s'", url, errContains, err)
		}
	})
}

func (n *Nginx) ContainerStatus() (*corev1.ContainerStatus, error) {
	return GetContainerStatus(n.Minikube, n.Pod, "nginx")
}

func (n *Nginx) Delete() error {
	return errors.Join(
		n.Minikube.DeletePod(n.Pod),
		n.Minikube.DeleteService(n.Service),
	)
}
