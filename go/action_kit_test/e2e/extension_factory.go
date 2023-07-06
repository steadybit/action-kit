package e2e

import (
	"context"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"os"
	"os/exec"
	"testing"
	"time"
)

type HelmExtensionFactory struct {
	Name             string
	ImageName        string
	Port             uint16
	ChartPath        string
	PodLabelSelector string
	ExtraArgs        func(minikube *Minikube) []string
	BeforeAllFunc    func(t *testing.T, m *Minikube, e *Extension) error
	BeforeEachFunc   func(t *testing.T, m *Minikube, e *Extension) error
	AfterAllFunc     func(t *testing.T, m *Minikube, e *Extension) error
	AfterEachFunc    func(t *testing.T, m *Minikube, e *Extension) error
}

func (h *HelmExtensionFactory) CreateImage() error {
	cmd := exec.Command("make", "container")
	cmd.Dir = ".."
	cmd.Stdout = &prefixWriter{prefix: "⚒️", w: os.Stdout}
	cmd.Stderr = &prefixWriter{prefix: "⚒️", w: os.Stdout}

	start := time.Now()
	if err := cmd.Run(); err != nil {
		return err
	}
	log.Info().TimeDiff("duration", time.Now(), start).Msg("extension image created")
	return nil
}

func (h *HelmExtensionFactory) Start(minikube *Minikube) (*Extension, error) {
	imageName := fmt.Sprintf("docker.io/library/%s", h.Name)
	if h.ImageName != "" {
		imageName = h.ImageName
	}

	chartPath := fmt.Sprintf("../charts/steadybit-%s", h.Name)
	if h.ChartPath != "" {
		chartPath = h.ChartPath
	}
	podLabelSelector := fmt.Sprintf("app.kubernetes.io/name=steadybit-%s", h.Name)
	if h.PodLabelSelector != "" {
		podLabelSelector = h.PodLabelSelector
	}

	start := time.Now()
	if err := minikube.LoadImage(imageName); err != nil {
		return nil, err
	}
	log.Info().TimeDiff("duration", time.Now(), start).Msg("extension image loaded")

	args := []string{
		"install",
		"--kube-context", minikube.Profile,
		"--namespace=default",
		"--set", fmt.Sprintf("image.name=%s", imageName),
		"--set", "image.pullPolicy=Never",
	}

	if h.ExtraArgs != nil {
		args = append(args, h.ExtraArgs(minikube)...)
	}
	args = append(args, h.Name, chartPath)

	start = time.Now()
	ctxHelm := context.Background()
	out, err := exec.CommandContext(ctxHelm, "helm", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to install helm chart: %s: %s", err, out)
	}
	log.Info().TimeDiff("duration", time.Now(), start).Msg("helm chart installed (without waiting for pods to start)")

	stopFwdCh := make(chan struct{})
	stop := func() error {
		close(stopFwdCh)
		out, err := exec.Command("helm", "uninstall", "--namespace=default", "--kube-context", minikube.Profile, h.Name).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to uninstall helm chart: %s: %s", err, out)
		}
		return nil
	}

	start = time.Now()
	ctxWaitForPods, waitForPodsCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer waitForPodsCancel()
	var pods []corev1.Pod
	for {
		select {
		case <-ctxWaitForPods.Done():
			_ = stop()
			return nil, fmt.Errorf("extension pods did not start in time")
		case <-time.After(1 * time.Second):
		}

		pods, err = minikube.ListPods(ctxWaitForPods, "default", podLabelSelector)
		if err != nil {
			_ = stop()
			return nil, err
		}

		if len(pods) > 0 {
			break
		}
	}
	log.Info().TimeDiff("duration", time.Now(), start).Int("pods", len(pods)).Msg("got list of pods")

	start = time.Now()
	tailCtx, tailCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer tailCancel()
	for _, pod := range pods {
		if err = minikube.WaitForPodPhase(pod.GetObjectMeta(), corev1.PodRunning, 3*time.Minute); err != nil {
			minikube.TailLog(tailCtx, pod.GetObjectMeta())
			_ = stop()
			return nil, err
		}
		minikube.TailLog(tailCtx, pod.GetObjectMeta())
	}
	log.Info().TimeDiff("duration", time.Now(), start).Msg("pods are in running state")

	localPort, err := minikube.PortForward(pods[0].GetObjectMeta(), h.Port, stopFwdCh)
	if err != nil {
		_ = stop()
		return nil, err
	}
	address := fmt.Sprintf("http://127.0.0.1:%d", localPort)
	client := resty.New().SetBaseURL(address)
	log.Info().TimeDiff("duration", time.Now(), start).Msgf("extension started. available at %s", address)

	return &Extension{Client: client, stop: stop, Pod: pods[0].GetObjectMeta()}, nil
}

func (h *HelmExtensionFactory) BeforeAll(t *testing.T, m *Minikube, e *Extension) error {
	if h.BeforeAllFunc == nil {
		return nil
	}
	return h.BeforeAllFunc(t, m, e)
}

func (h *HelmExtensionFactory) BeforeEach(t *testing.T, m *Minikube, e *Extension) error {
	if h.BeforeEachFunc == nil {
		return nil
	}
	return h.BeforeEachFunc(t, m, e)
}

func (h *HelmExtensionFactory) AfterAll(t *testing.T, m *Minikube, e *Extension) error {
	if h.AfterAllFunc == nil {
		return nil
	}
	return h.AfterAllFunc(t, m, e)
}

func (h *HelmExtensionFactory) AfterEach(t *testing.T, m *Minikube, e *Extension) error {
	if h.AfterEachFunc == nil {
		return nil
	}
	return h.AfterEachFunc(t, m, e)
}
