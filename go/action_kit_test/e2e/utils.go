// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"testing"
	"time"
)

func AssertProcessRunningInContainer(t *testing.T, m *Minikube, pod metav1.Object, containername string, comm string, showAll bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lastOutput := ""
	for {
		select {
		case <-ctx.Done():
			assert.Failf(t, "process not found", "process %s not found in container %s/%s.\n%s", comm, pod.GetName(), containername, lastOutput)
			return

		case <-time.After(200 * time.Millisecond):
			var out string
			var err error
			if showAll {
				out, err = m.PodExec(pod, containername, "ps", "-opid,comm", "-A")
			} else {
				out, err = m.PodExec(pod, containername, "ps", "-opid,comm")
			}
			require.NoError(t, err, "failed to exec ps -o=pid,comm: %s", out)

			for _, line := range strings.Split(out, "\n") {
				fields := strings.Fields(line)
				if len(fields) >= 2 && fields[1] == comm {
					return
				}
			}
			lastOutput = out
		}
	}
}

func AssertProcessNOTRunningInContainer(t *testing.T, m *Minikube, pod metav1.Object, containername string, comm string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lastOutput := ""
	for {
		select {
		case <-ctx.Done():
			return

		case <-time.After(200 * time.Millisecond):
			out, err := m.PodExec(pod, containername, "ps", "-opid,comm", "-A")
			require.NoError(t, err, "failed to exec ps -o=pid,comm: %s", out)

			for _, line := range strings.Split(out, "\n") {
				fields := strings.Fields(line)
				if len(fields) >= 2 && fields[1] == comm {
					assert.Fail(t, "process found", "process %s found in container %s/%s.\n%s", comm, pod.GetName(), containername, lastOutput)
					return
				}
			}
			lastOutput = out
		}
	}
}

func NewContainerTarget(m *Minikube, pod metav1.Object, containername string) (*action_kit_api.Target, error) {
	status, err := GetContainerStatus(m, pod, containername)
	if err != nil {
		return nil, err
	}
	return &action_kit_api.Target{
		Attributes: map[string][]string{
			"container.id": {status.ContainerID},
		},
	}, nil
}

func GetContainerStatus(m *Minikube, pod metav1.Object, containername string) (*corev1.ContainerStatus, error) {
	r, err := m.GetPod(pod)
	if err != nil {
		return nil, err
	}

	for _, status := range r.Status.ContainerStatuses {
		if status.Name == containername {
			return &status, nil
		}
	}
	return nil, errors.New("container not found")
}

func WaitForContainerStatusUsingContainerEngine(m *Minikube, containerId string, wantedStatus string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var lastError error
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("container %s did not reach status %s. last error %w", containerId, wantedStatus, lastError)
		case <-time.After(200 * time.Millisecond):
			status, err := getContainerStatusUsingContainerEngine(m, containerId)
			if err != nil {
				lastError = err
			} else {
				if status == wantedStatus {
					return nil
				}
			}
		}
	}
}

func getContainerStatusUsingContainerEngine(m *Minikube, containerId string) (string, error) {
	if strings.HasPrefix(containerId, string(RuntimeDocker)) {
		var outb bytes.Buffer
		cmd := m.SshExec("sudo docker", "inspect", "-f='{{.State.Status}}'", RemovePrefix(containerId))
		cmd.Stdout = &outb
		if err := cmd.Run(); err != nil {
			return "", err
		}
		return strings.TrimSpace(outb.String()), nil
	}

	if strings.HasPrefix(containerId, string(RuntimeContainerd)) {
		var outb bytes.Buffer
		cmd := m.SshExec("sudo ctr", "--namespace=k8s.io", "tasks", "list")
		cmd.Stdout = &outb
		if err := cmd.Run(); err != nil {
			return "", err
		}

		for _, line := range strings.Split(outb.String(), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 3 && fields[0] == RemovePrefix(containerId) {
				return strings.ToLower(fields[2]), nil
			}
		}
		return "", fmt.Errorf("container not found container runtime")
	}

	return "", fmt.Errorf("unsupported container runtime")
}

func PollForTarget(ctx context.Context, e *Extension, targetId string, predicate func(target discovery_kit_api.Target) bool) (discovery_kit_api.Target, error) {
	var lastErr error
	for {
		select {
		case <-ctx.Done():
			return discovery_kit_api.Target{}, fmt.Errorf("timed out waiting for target. last error: %w", lastErr)
		case <-time.After(200 * time.Millisecond):
			result, err := e.DiscoverTargets(targetId)
			if err != nil {
				lastErr = err
				continue
			}
			for _, target := range result {
				if predicate(target) {
					return target, nil
				}
			}
		}
	}
}

func PollForEnrichmentData(ctx context.Context, e *Extension, targetId string, predicate func(target discovery_kit_api.EnrichmentData) bool) (discovery_kit_api.EnrichmentData, error) {
	var lastErr error
	for {
		select {
		case <-ctx.Done():
			return discovery_kit_api.EnrichmentData{}, fmt.Errorf("timed out waiting for target. last error: %w", lastErr)
		case <-time.After(200 * time.Millisecond):
			result, err := e.DiscoverEnrichmentData(targetId)
			if err != nil {
				lastErr = err
				continue
			}
			for _, enrichmentData := range result {
				if predicate(enrichmentData) {
					return enrichmentData, nil
				}
			}
		}
	}
}

func HasAttribute(target discovery_kit_api.Target, key, value string) bool {
	return ContainsAttribute(target.Attributes, key, value)
}

func ContainsAttribute(attributes map[string][]string, key, value string) bool {
	for _, v := range attributes[key] {
		if v == value {
			return true
		}
	}
	return false
}

func AssertLogContains(t *testing.T, m *Minikube, pod metav1.Object, expectedLog string) {
	t.Helper()
	AssertLogContainsWithTimeout(t, m, pod, expectedLog, 30*time.Second)
}

func AssertLogContainsWithTimeout(t *testing.T, m *Minikube, pod metav1.Object, expectedLog string, timeout time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var sinceSeconds *int64
	sinceSeconds = extutil.Ptr(int64(180))
	for {
		select {
		case <-ctx.Done():
			assert.Fail(t, fmt.Sprintf("Failed to find log '%s'", expectedLog))
			return
		case <-time.After(2000 * time.Millisecond):
			found := findLog(m, pod, expectedLog, sinceSeconds)
			if found {
				return
			}
			//after first try only look for last 5 seconds
			sinceSeconds = extutil.Ptr(int64(5))
		}
	}
}

func findLog(m *Minikube, pod metav1.Object, expectedLog string, seconds *int64) bool {
	logCtx, logCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer logCancel()
	podLogs, err := m.GetClient().CoreV1().Pods(pod.GetNamespace()).GetLogs(pod.GetName(), &corev1.PodLogOptions{SinceSeconds: seconds}).Stream(logCtx)

	if err != nil {
		log.Error().Err(err).Msg("Failed to read logs from pod")
	}
	defer func() { _ = podLogs.Close() }()
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read logs from pod (copy)")
	}
	podLogString := buf.String()
	log.Info().Msgf("Try to find log for '%s' in %d bytes", expectedLog, len(podLogString))
	if strings.Contains(podLogString, expectedLog) {
		log.Info().Msg("Found log!")
		return true
	}
	return false
}
