package e2e

import (
	"context"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	aclient "github.com/steadybit/action-kit/go/action_kit_test/client"
	dclient "github.com/steadybit/discovery-kit/go/discovery_kit_test/client"
	corev1 "k8s.io/api/core/v1"
	"os"
	"os/exec"
	"time"
)

type HelmExtensionFactory struct {
	Name             string
	ImageName        string
	ImageTag         string
	Port             uint16
	ChartPath        string
	PodLabelSelector string
	ExtraArgs        func(minikube *Minikube) []string
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

	imageTag := "latest"
	if h.ImageName != "" {
		imageTag = h.ImageTag
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
		"--set", fmt.Sprintf("image.tag=%s", imageTag),
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
		return nil, fmt.Errorf("failed to install helm chart: %w: %s", err, out)
	}
	log.Info().TimeDiff("duration", time.Now(), start).Msg("helm chart installed (without waiting for pods to start)")

	stopFwdCh := make(chan struct{})
	tailCtx, tailCancel := context.WithCancel(context.Background())
	success := false

	stop := func() error {
		defer tailCancel()
		defer close(stopFwdCh)
		out, err := exec.Command("helm", "uninstall", "--namespace=default", "--kube-context", minikube.Profile, h.Name).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to uninstall helm chart: %w: %s", err, out)
		}
		return nil
	}

	defer func() {
		if !success {
			_ = stop()
		}
	}()

	start = time.Now()
	waitForPodsCtx, waitForPodsCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer waitForPodsCancel()
	var pods []corev1.Pod
	for {
		select {
		case <-waitForPodsCtx.Done():
			return nil, fmt.Errorf("extension pods did not start in time")
		case <-time.After(1 * time.Second):
		}

		pods, err = minikube.ListPods(waitForPodsCtx, "default", podLabelSelector)
		if err != nil {
			return nil, err
		}

		if len(pods) > 0 {
			break
		}
	}
	log.Info().TimeDiff("duration", time.Now(), start).Int("pods", len(pods)).Msg("pod(s) for extension created")

	start = time.Now()
	for _, pod := range pods {
		if err = minikube.WaitForPodReady(pod.GetObjectMeta(), 3*time.Minute); err != nil {
			minikube.TailLog(tailCtx, pod.GetObjectMeta())
			return nil, err
		} else {
			go minikube.TailLog(tailCtx, pod.GetObjectMeta())
		}
	}
	log.Info().TimeDiff("duration", time.Now(), start).Msg("pods are in running state")

	localPort, err := minikube.PortForward(pods[0].GetObjectMeta(), h.Port, stopFwdCh)
	if err != nil {
		return nil, err
	}
	address := fmt.Sprintf("http://127.0.0.1:%d", localPort)
	client := resty.New().SetBaseURL(address)
	log.Info().TimeDiff("duration", time.Now(), start).Msgf("extension started. available at %s", address)

	success = true
	return &Extension{
		Client:    client,
		discovery: dclient.NewDiscoveryClient("/", client),
		action:    aclient.NewActionClient("/", client),
		stop:      stop,
		Pod:       pods[0].GetObjectMeta(),
	}, nil
}
