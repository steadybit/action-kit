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
	"strconv"
	"strings"
	"time"
)

type Netperf struct {
	Minikube  *Minikube
	ServerPod metav1.Object
	ClientPod metav1.Object
	ServerIp  string
}

func (n *Netperf) Deploy(name string) error {
	serverPodName := fmt.Sprintf("%s-server", name)
	pod, err := n.Minikube.CreatePod(&acorev1.PodApplyConfiguration{
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
					Name:  extutil.Ptr("netserver"),
					Image: extutil.Ptr("networkstatic/netserver:latest"),
					Args:  []string{"-D"},
					Ports: []acorev1.ContainerPortApplyConfiguration{
						{
							Name:          extutil.Ptr("control"),
							ContainerPort: extutil.Ptr(int32(12865)),
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

	describe, err := n.Minikube.GetPod(pod)
	if err != nil {
		return err
	}
	n.ServerPod = pod
	n.ServerIp = describe.Status.PodIP

	clientPodName := fmt.Sprintf("%s-client", name)
	pod, err = n.Minikube.CreatePod(&acorev1.PodApplyConfiguration{
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
					Name:    extutil.Ptr("netperf"),
					Image:   extutil.Ptr("networkstatic/netperf:latest"),
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

func (n *Netperf) Target() (*action_kit_api.Target, error) {
	return NewContainerTarget(n.Minikube, n.ServerPod, "netserver")
}

func (n *Netperf) MeasureLatency() (time.Duration, error) {
	out, err := n.run("TCP_RR", "-P5000", "-r", "1,1", "-o", "mean_latency")
	if err != nil {
		return 0, fmt.Errorf("%s: %s", err, out)
	}

	lines := strings.Split(out, "\n")
	if len(lines) < 3 {
		return 0, fmt.Errorf("unexpected output: %s", out)
	}

	latency, err := strconv.ParseFloat(strings.TrimSpace(lines[2]), 64)
	if err != nil {
		return 0, fmt.Errorf("unexpected output: %s", out)
	}
	duration := time.Duration(latency) * time.Microsecond
	return duration, nil
}

func (n *Netperf) run(test string, args ...string) (string, error) {
	var out string
	var err error
	cmd := append([]string{"netperf", "-H", n.ServerIp, "-l2", "-t", test, "--"}, args...)
	for attempt := 0; attempt < 5; attempt++ {
		out, err = n.Minikube.Exec(n.ClientPod, "netperf", cmd...)
		if err == nil {
			break
		} else {
			if !strings.Contains(out, "Cannot assign requested address") {
				return "", fmt.Errorf("%s: %s", err, out)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	return out, err
}
func (n *Netperf) Delete() error {
	return errors.Join(
		n.Minikube.DeletePod(n.ServerPod),
		n.Minikube.DeletePod(n.ClientPod),
	)

}
