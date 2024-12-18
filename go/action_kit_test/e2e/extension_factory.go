package e2e

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	stdout := &prefixWriter{prefix: []byte("⚒️"), w: os.Stdout}
	cmd.Stdout = stdout
	cmd.Stderr = stdout

	start := time.Now()
	if err := cmd.Run(); err != nil {
		return err
	}
	log.Info().TimeDiff("duration", time.Now(), start).Msg("extension image created")
	return nil
}

func (h *HelmExtensionFactory) Start(minikube *Minikube) (*Extension, error) {
	start := time.Now()
	if err := minikube.LoadImage(h.imageName()); err != nil {
		return nil, err
	}
	log.Info().TimeDiff("duration", time.Now(), start).Msg("extension image loaded")

	ext := &Extension{
		install: func(values map[string]string) error {
			return h.upgrade(minikube, values)
		},
		uninstall: func() error {
			return h.uninstall(minikube)
		},
		connect: func(ctx context.Context) (uint16, metav1.Object, error) {
			return h.connect(ctx, minikube)
		},
	}

	if err := ext.ResetConfig(); err != nil {
		return nil, err
	}
	return ext, nil
}

func (h *HelmExtensionFactory) upgrade(minikube *Minikube, values map[string]string) error {
	chartPath := fmt.Sprintf("../charts/steadybit-%s", h.Name)
	if h.ChartPath != "" {
		chartPath = h.ChartPath
	}

	args := []string{
		"upgrade",
		"--install",
		"--kube-context", minikube.Profile,
		"--namespace=default",
		"--set", fmt.Sprintf("image.name=%s", h.imageName()),
		"--set", fmt.Sprintf("image.tag=%s", h.imageTag()),
		"--set", "image.pullPolicy=Never",
		"--wait",
	}

	for key, value := range values {
		args = append(args, "--set", fmt.Sprintf("%s=%s", key, value))
	}

	if h.ExtraArgs != nil {
		args = append(args, h.ExtraArgs(minikube)...)
	}

	args = append(args, h.Name, chartPath)

	start := time.Now()
	if out, err := exec.CommandContext(context.Background(), "helm", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install helm chart: %w: %s", err, out)
	}
	log.Info().TimeDiff("duration", time.Now(), start).Msg("extension helm release installed")
	return nil
}

func (h *HelmExtensionFactory) imageName() string {
	imageName := fmt.Sprintf("docker.io/library/%s", h.Name)
	if h.ImageName != "" {
		imageName = h.ImageName
	}
	return imageName
}

func (h *HelmExtensionFactory) imageTag() string {
	imageTag := "latest"
	if h.ImageTag != "" {
		imageTag = h.ImageTag
	}
	return imageTag
}

func (h *HelmExtensionFactory) uninstall(minikube *Minikube) error {
	start := time.Now()
	if out, err := exec.Command("helm", "uninstall", "--namespace=default", "--kube-context", minikube.Profile, h.Name).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to uninstall helm chart: %w: %s", err, out)
	}
	log.Info().TimeDiff("duration", time.Now(), start).Msg("extension helm release uninstalled")
	return nil
}

func (h *HelmExtensionFactory) connect(ctx context.Context, minikube *Minikube) (uint16, metav1.Object, error) {
	podLabelSelector := fmt.Sprintf("app.kubernetes.io/name=steadybit-%s", h.Name)
	if h.PodLabelSelector != "" {
		podLabelSelector = h.PodLabelSelector
	}

	start := time.Now()
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer waitCancel()
	var pods []corev1.Pod
	for {
		select {
		case <-waitCtx.Done():
			return 0, nil, fmt.Errorf("extension pods did not start in time")
		case <-time.After(1 * time.Second):
		}

		var err error
		pods, err = minikube.ListPods(waitCtx, "default", podLabelSelector)
		if err != nil {
			return 0, nil, err
		}

		if len(pods) > 0 {
			break
		}
	}
	log.Info().TimeDiff("duration", time.Now(), start).Int("pods", len(pods)).Msg("pods for extension created")

	start = time.Now()
	for _, pod := range pods {
		if err := minikube.WaitForPodReady(pod.GetObjectMeta(), 3*time.Minute); err != nil {
			minikube.TailLog(ctx, pod.GetObjectMeta())
			return 0, nil, err
		} else {
			go minikube.TailLog(ctx, pod.GetObjectMeta())
		}
	}
	log.Info().TimeDiff("duration", time.Now(), start).Msg("extension pods are in running state")

	localPort, err := minikube.PortForward(pods[0].GetObjectMeta(), h.Port, ctx.Done())
	if err != nil {
		return 0, nil, err
	}
	log.Info().TimeDiff("duration", time.Now(), start).Msgf("extension started. available at %d", localPort)

	return localPort, pods[0].GetObjectMeta(), nil
}
