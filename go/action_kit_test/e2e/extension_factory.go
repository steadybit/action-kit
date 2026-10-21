package e2e

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"os/exec"
	"sort"
	"strings"
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
	pods, err := waitForStableReadyPods(minikube, podLabelSelector, 3*time.Minute)
	if err != nil {
		for _, pod := range pods {
			minikube.TailLog(ctx, pod.GetObjectMeta())
		}
		return 0, nil, err
	}
	for _, pod := range pods {
		go minikube.TailLog(ctx, pod.GetObjectMeta())
	}
	log.Info().TimeDiff("duration", time.Now(), start).Int("pods", len(pods)).Msg("extension pods are in running state")

	localPort, err := minikube.PortForward(pods[0].GetObjectMeta(), h.Port, ctx.Done())
	if err != nil {
		return 0, nil, err
	}
	log.Info().TimeDiff("duration", time.Now(), start).Msgf("extension started. available at %d", localPort)

	return localPort, pods[0].GetObjectMeta(), nil
}

// waitForStableReadyPods waits until the extension's pods that are not being deleted are all ready, and
// the same set was seen on two consecutive polls. After a helm upgrade the previous revision's pod is
// still listed, first ready and then terminating; it must not be picked, nor waited on until it is gone.
func waitForStableReadyPods(minikube *Minikube, podLabelSelector string, timeout time.Duration) ([]corev1.Pod, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var previous string
	var pods []corev1.Pod
	for {
		select {
		case <-ctx.Done():
			return pods, fmt.Errorf("extension pods did not become ready in time. last seen: %s", describePods(pods))
		case <-time.After(1 * time.Second):
		}

		listed, err := minikube.ListPods(ctx, "default", podLabelSelector)
		if err != nil {
			if ctx.Err() != nil {
				continue
			}
			return nil, err
		}

		var current string
		var allReady bool
		pods, current, allReady = activePods(listed)

		if len(pods) > 0 && allReady && current == previous {
			return pods, nil
		}
		previous = current
	}
}

// activePods drops pods that are being deleted and returns the rest, a key identifying that set, and whether
// all of them are ready.
func activePods(listed []corev1.Pod) ([]corev1.Pod, string, bool) {
	var pods []corev1.Pod
	var names []string
	allReady := true
	for _, pod := range listed {
		if pod.DeletionTimestamp != nil {
			continue
		}
		pods = append(pods, pod)
		names = append(names, pod.Name)
		allReady = allReady && isPodReady(&pod)
	}
	sort.Strings(names)
	return pods, strings.Join(names, ","), allReady
}

func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func describePods(pods []corev1.Pod) string {
	if len(pods) == 0 {
		return "no pods"
	}
	var parts []string
	for _, pod := range pods {
		parts = append(parts, fmt.Sprintf("%s (phase %s, ready %t)", pod.Name, pod.Status.Phase, isPodReady(&pod)))
	}
	return strings.Join(parts, ", ")
}
