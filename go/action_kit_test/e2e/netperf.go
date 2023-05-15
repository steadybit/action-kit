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
	"testing"
	"time"
)

type Netperf struct {
	Minikube  *Minikube
	ServerPod metav1.Object
	ClientPod metav1.Object
	ServerIp  string
}

func (n *Netperf) Deploy(name string, opts ...func(server *acorev1.PodApplyConfiguration, client *acorev1.PodApplyConfiguration)) error {
	serverPodName := fmt.Sprintf("%s-server", name)
	serverCfg := &acorev1.PodApplyConfiguration{
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
	}

	clientPodName := fmt.Sprintf("%s-client", name)
	clientCfg := &acorev1.PodApplyConfiguration{
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
	}

	for _, fn := range opts {
		fn(serverCfg, clientCfg)
	}

	serverPod, err := n.Minikube.CreatePod(serverCfg)
	if err != nil {
		return err
	}

	describe, err := n.Minikube.GetPod(serverPod)
	if err != nil {
		return err
	}
	n.ServerPod = serverPod
	n.ServerIp = describe.Status.PodIP

	clientPod, err := n.Minikube.CreatePod(clientCfg)
	if err != nil {
		return err
	}
	n.ClientPod = clientPod
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

func (n *Netperf) AssertLatency(t *testing.T, min time.Duration, max time.Duration) {
	t.Helper()

	measurements := make([]time.Duration, 0, 5)
	Retry(t, 8, 500*time.Millisecond, func(r *R) {
		latency, err := n.MeasureLatency()
		if err != nil {
			r.failed = true
			_, _ = fmt.Fprintf(r.log, "failed to measure package latency: %s", err)
		}
		if latency < min || latency > max {
			r.failed = true
			measurements = append(measurements, latency)
			_, _ = fmt.Fprintf(r.log, "package latency %v is not in expected range [%s, %s]", measurements, min, max)
		}
	})
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
