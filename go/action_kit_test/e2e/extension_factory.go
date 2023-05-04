package e2e

import (
	"context"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"os"
	"os/exec"
	"time"
)

type HelmExtensionFactory struct {
	Name             string
	ImageName        string
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

	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (h *HelmExtensionFactory) Start(minikube *Minikube) (*Extension, error) {
	if h.ImageName == "" {
		h.ImageName = fmt.Sprintf("docker.io/library/%s", h.Name)
	}

	if h.ChartPath == "" {
		h.ChartPath = fmt.Sprintf("../charts/steadybit-%s", h.Name)
	}
	if h.PodLabelSelector == "" {
		h.PodLabelSelector = fmt.Sprintf("app.kubernetes.io/name=steadybit-%s", h.Name)
	}

	if err := minikube.LoadImage(h.ImageName); err != nil {
		return nil, err
	}

	args := []string{
		"install",
		"--kube-context", minikube.Profile,
		"--namespace=default",
		"--wait",
		"--set", fmt.Sprintf("image.name=%s", h.ImageName),
		"--set", "image.pullPolicy=Never",
	}

	if h.ExtraArgs != nil {
		args = append(args, h.ExtraArgs(minikube)...)
	}
	args = append(args, h.Name, h.ChartPath)

	ctx := context.Background()
	out, err := exec.CommandContext(ctx, "helm", args...).CombinedOutput()

	if err != nil {
		return nil, fmt.Errorf("failed to install helm chart: %s: %s", err, out)
	}

	tailCtx, tailCancel := context.WithCancel(context.Background())
	stopFwdCh := make(chan struct{})
	stop := func() error {
		tailCancel()
		close(stopFwdCh)
		out, err := exec.Command("helm", "uninstall", "--namespace=default", "--kube-context", minikube.Profile, h.Name).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to uninstall helm chart: %s: %s", err, out)
		}
		return nil
	}

	ctx, waitCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer waitCancel()
	var pods []corev1.Pod
	for {
		select {
		case <-ctx.Done():
			_ = stop()
			return nil, fmt.Errorf("extension pods did not start in time")
		case <-time.After(200 * time.Millisecond):
		}

		pods, err = minikube.ListPods(ctx, "default", h.PodLabelSelector)
		if err != nil {
			_ = stop()
			return nil, err
		}

		for _, pod := range pods {
			if err = minikube.WaitForPodPhase(pod.GetObjectMeta(), corev1.PodRunning, 3*time.Minute); err != nil {
				_ = stop()
				return nil, err
			}
			go minikube.TailLog(tailCtx, pod.GetObjectMeta())
		}
		if len(pods) > 0 {
			break
		}
	}

	localPort, err := minikube.PortForward(pods[0].GetObjectMeta(), h.Port, stopFwdCh)
	if err != nil {
		_ = stop()
		return nil, err
	}

	address := fmt.Sprintf("http://127.0.0.1:%d", localPort)
	client := resty.New().SetBaseURL(address)
	log.Info().Msgf("extension is available at %s", address)
	return &Extension{client: client, stop: stop, Pod: pods[0].GetObjectMeta()}, nil
}
