// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/yalp/jsonpath"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	ametav1 "k8s.io/client-go/applyconfigurations/meta/v1"
)

type iperf struct {
	minikube  *Minikube
	ServerPod metav1.Object
	ClientPod metav1.Object
	ServerIp  string
}

func (n *iperf) Deploy(name string) error {
	serverPodName := fmt.Sprintf("%s-server", name)
	pod, err := n.minikube.CreatePod(&acorev1.PodApplyConfiguration{
		TypeMetaApplyConfiguration: ametav1.TypeMetaApplyConfiguration{
			Kind:       extutil.Ptr("Pod"),
			APIVersion: extutil.Ptr("v1"),
		},
		ObjectMetaApplyConfiguration: &ametav1.ObjectMetaApplyConfiguration{
			Name:   &serverPodName,
			Labels: map[string]string{"app": serverPodName},
		},
		Spec: &acorev1.PodSpecApplyConfiguration{
			RestartPolicy: extutil.Ptr(corev1.RestartPolicyNever),
			Containers: []acorev1.ContainerApplyConfiguration{
				{
					Name:  extutil.Ptr("iperf"),
					Image: extutil.Ptr("networkstatic/iperf3:latest"),
					Args:  []string{"-s", "-p", "5201"},
					Ports: []acorev1.ContainerPortApplyConfiguration{
						{
							Name:          extutil.Ptr("control"),
							ContainerPort: extutil.Ptr(int32(5201)),
						},
						{
							Name:          extutil.Ptr("data-tcp"),
							ContainerPort: extutil.Ptr(int32(5000)),
							Protocol:      extutil.Ptr(corev1.ProtocolTCP),
						}, {
							Name:          extutil.Ptr("data-udp"),
							ContainerPort: extutil.Ptr(int32(5001)),
							Protocol:      extutil.Ptr(corev1.ProtocolUDP),
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

	describe, err := n.minikube.GetPod(pod)
	if err != nil {
		return err
	}
	n.ServerIp = describe.Status.PodIP
	n.ServerPod = pod

	clientPodName := fmt.Sprintf("%s-client", name)
	pod, err = n.minikube.CreatePod(&acorev1.PodApplyConfiguration{
		TypeMetaApplyConfiguration: ametav1.TypeMetaApplyConfiguration{
			Kind:       extutil.Ptr("Pod"),
			APIVersion: extutil.Ptr("v1"),
		},
		ObjectMetaApplyConfiguration: &ametav1.ObjectMetaApplyConfiguration{
			Name:   &clientPodName,
			Labels: map[string]string{"app": clientPodName},
		},
		Spec: &acorev1.PodSpecApplyConfiguration{
			RestartPolicy: extutil.Ptr(corev1.RestartPolicyNever),
			Containers: []acorev1.ContainerApplyConfiguration{
				{
					Name:    extutil.Ptr("iperf"),
					Image:   extutil.Ptr("networkstatic/iperf3:latest"),
					Command: []string{"sleep", "infinity"},
				},
			},
		},
		Status: nil,
	})
	if err != nil {
		return err
	}
	n.ClientPod = pod
	return nil
}

func (n *iperf) Target() (*action_kit_api.Target, error) {
	return NewContainerTarget(n.minikube, n.ServerPod, "iperf")
}

func (n *iperf) MeasurePackageLoss() (float64, error) {
	out, err := n.minikube.Exec(n.ClientPod, "iperf", "iperf3", "--client", n.ServerIp, "--port=5201", "--udp", "--time=2", "--length=1k", "--bind=0.0.0.0", "--reverse", "--cport=5001", "--no-delay", "--zerocopy", "--json")
	if err != nil {
		return 0, fmt.Errorf("%s: %s", err, out)
	}

	var result interface{}
	err = json.Unmarshal([]byte(out), &result)
	if err != nil {
		return 0, fmt.Errorf("failed reading results: %w", err)
	}

	lost, err := jsonpath.Read(result, "$.end.sum.lost_percent")
	if err != nil {
		return 0, fmt.Errorf("failed reading lost_percent: %w", err)
	}
	return lost.(float64), nil
}

func (n *iperf) Delete() error {
	return errors.Join(
		n.minikube.DeletePod(n.ServerPod),
		n.minikube.DeletePod(n.ClientPod),
	)

}

func (n *iperf) MeasureBandwidth() (float64, error) {
	out, err := n.minikube.Exec(n.ClientPod, "iperf", "iperf3", "--client", n.ServerIp, "--port=5201", "--udp", "--time=2", "--bind=0.0.0.0", "--reverse", "--cport=5001", "--bitrate=500M", "--no-delay", "--json")
	if err != nil {
		return 0, fmt.Errorf("%s: %s", err, out)
	}

	var result interface{}
	err = json.Unmarshal([]byte(out), &result)
	if err != nil {
		return 0, fmt.Errorf("failed reading results: %w", err)
	}

	bps, err := jsonpath.Read(result, "$.end.sum.bits_per_second")
	if err != nil {
		return 0, fmt.Errorf("failed reading bits_per_second: %w", err)
	}
	return bps.(float64) / 1_000_000, nil
}
