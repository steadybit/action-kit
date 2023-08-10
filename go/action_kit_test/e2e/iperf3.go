// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/yalp/jsonpath"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	ametav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"testing"
	"time"
)

type Iperf struct {
	Minikube  *Minikube
	ServerPod metav1.Object
	ClientPod metav1.Object
	ServerIp  string
}

func (n *Iperf) Deploy(name string, opts ...func(server *acorev1.PodApplyConfiguration, client *acorev1.PodApplyConfiguration)) error {
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
					Name:    extutil.Ptr("iperf"),
					Image:   extutil.Ptr("networkstatic/iperf3:latest"),
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
	n.ServerIp = describe.Status.PodIP
	n.ServerPod = serverPod

	clientPod, err := n.Minikube.CreatePod(clientCfg)
	if err != nil {
		return err
	}
	n.ClientPod = clientPod
	return nil
}

func (n *Iperf) Target() (*action_kit_api.Target, error) {
	return NewContainerTarget(n.Minikube, n.ServerPod, "iperf")
}

func (n *Iperf) Delete() error {
	return errors.Join(
		n.Minikube.DeletePod(n.ServerPod),
		n.Minikube.DeletePod(n.ClientPod),
	)
}

func (n *Iperf) MeasurePackageLoss() (float64, error) {
	out, err := n.Minikube.PodExec(n.ClientPod, "iperf", "iperf3", "--client", n.ServerIp, "--port=5201", "--udp", "--time=2", "--length=1k", "--bind=0.0.0.0", "--reverse", "--cport=5001", "--no-delay", "--zerocopy", "--json")
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

func (n *Iperf) AssertPackageLoss(t *testing.T, min float64, max float64) {
	t.Helper()

	measurements := make([]float64, 0, 5)
	Retry(t, 8, 500*time.Millisecond, func(r *R) {
		loss, err := n.MeasurePackageLoss()
		if err != nil {
			r.Failed = true
			_, _ = fmt.Fprintf(r.Log, "failed to measure package loss: %s", err)
		}
		if loss < min || loss > max {
			r.Failed = true
			measurements = append(measurements, loss)
			_, _ = fmt.Fprintf(r.Log, "package loss %v is not in expected range [%f, %f]", measurements, min, max)
		}
	})
}

func (n *Iperf) AssertPackageLossWithRetry(min float64, max float64, maxRetries int) bool {

	measurements := make([]float64, 0, 5)
	success := false
	for i := 0; i < maxRetries; i++ {
		loss, err := n.MeasurePackageLoss()
		if err != nil {
			success = false
			log.Err(err).Msg("failed to measure package loss")
			break
		}
		if loss < min || loss > max {
			success = false
			measurements = append(measurements, loss)
		} else {
			success = true
			break
		}
	}
	if !success {
		log.Info().Msgf("package loss %v is not in expected range [%f, %f]", measurements, min, max)
	}
	return success
}

func (n *Iperf) MeasureBandwidth() (float64, error) {
	out, err := n.Minikube.PodExec(n.ClientPod, "iperf", "iperf3", "--client", n.ServerIp, "--port=5201", "--udp", "--time=3", "--bind=0.0.0.0", "--reverse", "--cport=5001", "--bitrate=500M", "--no-delay", "--json")
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

func (n *Iperf) AssertBandwidth(t *testing.T, min float64, max float64) {
	t.Helper()

	measurements := make([]float64, 0, 5)
	Retry(t, 8, 500*time.Millisecond, func(r *R) {
		bandwidth, err := n.MeasureBandwidth()
		if err != nil {
			r.Failed = true
			_, _ = fmt.Fprintf(r.Log, "failed to measure bandwidth bandwidth: %s", err)
		}
		if bandwidth < min || bandwidth > max {
			r.Failed = true
			measurements = append(measurements, bandwidth)
			_, _ = fmt.Fprintf(r.Log, "bandwidth %f is not in expected range [%f, %f]", measurements, min, max)
		}
	})
}
