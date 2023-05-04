// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-container/pkg/container/types"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/require"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/client-go/util/homedir"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	globalMinikubeMutex sync.Mutex
)

type Minikube struct {
	runtime types.Runtime
	profile string
	stdout  io.Writer
	stderr  io.Writer

	clientOnce   sync.Once
	client       *kubernetes.Clientset
	clientConfig *rest.Config
}

func newMinikube(runtime types.Runtime) *Minikube {
	profile := "e2e-" + string(runtime)
	stdout := prefixWriter{prefix: "🧊", w: os.Stdout}
	stderr := prefixWriter{prefix: "🧊", w: os.Stderr}

	return &Minikube{
		runtime: runtime,
		profile: profile,
		stdout:  &stdout,
		stderr:  &stderr,
	}
}

func (m *Minikube) start() error {
	globalMinikubeMutex.Lock()
	defer globalMinikubeMutex.Unlock()

	args := []string{"start", "--keep-context", fmt.Sprintf("--container-runtime=%s", string(m.runtime)), "--ports=8080"}
	if m.runtime == "cri-o" {
		args = append(args, "--cni=bridge")
	}

	if err := m.command(args...).Run(); err != nil {
		return err
	}

	return nil
}

func (m *Minikube) Client() *kubernetes.Clientset {
	if m.client == nil {
		m.clientOnce.Do(func() {
			client, config, err := createKubernetesClient(m.profile)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to create kubernetes client")
			}
			m.client = client
			m.clientConfig = config
		})
	}
	return m.client
}

func (m *Minikube) Config() *rest.Config {
	if m.clientConfig == nil {
		m.Client()
	}
	return m.clientConfig
}

func (m *Minikube) Runtime() types.Runtime {
	return m.runtime
}

func (m *Minikube) waitForDefaultServiceaccount() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return errors.New("the serviceaccount 'default' was not created")
		case <-time.After(200 * time.Millisecond):
			if _, err := m.Client().CoreV1().ServiceAccounts("default").Get(context.Background(), "default", metav1.GetOptions{}); err == nil {
				return nil
			}
		}
	}
}

func (m *Minikube) delete() error {
	globalMinikubeMutex.Lock()
	defer globalMinikubeMutex.Unlock()
	return m.command("delete").Run()
}

func (m *Minikube) cp(src, dst string) error {
	return m.command(m.profile, "cp", src, dst).Run()
}

func (m *Minikube) exec(arg ...string) *exec.Cmd {
	return m.command(append([]string{"ssh", "--"}, arg...)...)
}

func (m *Minikube) LoadImage(image string) error {
	return m.command("image", "load", image).Run()
}

func (m *Minikube) command(arg ...string) *exec.Cmd {
	return m.commandContext(context.Background(), arg...)
}

func (m *Minikube) commandContext(ctx context.Context, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "minikube", append([]string{"-p", m.profile}, arg...)...)
	cmd.Stdout = m.stdout
	cmd.Stderr = m.stderr
	return cmd
}

type WithMinikubeTestCase struct {
	Name string
	Test func(t *testing.T, minikube *Minikube, e *Extension)
}

func WithMinikube(t *testing.T, runtimes []types.Runtime, testCases []WithMinikubeTestCase) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	imageName := ""
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		s, err := createExtensionContainer()
		if err != nil {
			log.Fatal().Msgf("failed to create extension executable: %v", err)
		}
		imageName = s
		wg.Done()
	}()

	for _, runtime := range runtimes {
		t.Run(string(runtime), func(t *testing.T) {

			minikube := newMinikube(runtime)
			_ = minikube.delete()
			err := minikube.start()
			if err != nil {
				log.Fatal().Msgf("failed to start minikube: %v", err)
			}
			defer func() { _ = minikube.delete() }()

			t.Parallel()

			if err := minikube.waitForDefaultServiceaccount(); err != nil {
				t.Fatal("Serviceaccount didn't show up", err)
			}

			wg.Wait()
			extension, err := startExtension(minikube, imageName)
			require.NoError(t, err)
			defer func() { _ = extension.stop() }()

			for _, tc := range testCases {
				t.Run(tc.Name, func(t *testing.T) {
					tc.Test(t, minikube, extension)
				})
			}
		})
	}
}

func createKubernetesClient(context string) (*kubernetes.Clientset, *rest.Config, error) {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: filepath.Join(homedir.HomeDir(), ".kube", "config")},
		&clientcmd.ConfigOverrides{CurrentContext: context},
	).ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	return client, config, err
}

type prefixWriter struct {
	prefix string
	w      io.Writer
}

func (w *prefixWriter) Write(p []byte) (n int, err error) {
	lines := strings.Split(strings.TrimSuffix(string(p), "\n"), "\n")
	count := 0
	for _, line := range lines {
		c, err := fmt.Fprintf(w.w, "%s%s\n", w.prefix, line)
		count += c
		if err != nil {
			return count, err
		}
	}
	return len(p), nil
}

type ServiceClient struct {
	resty.Client
	close func()
}

func (c *ServiceClient) Close() {
	c.close()
}

func (m *Minikube) NewRestClientForService(service metav1.Object) (*ServiceClient, error) {
	url, cancel, err := m.TunnelService(service)
	if err != nil {
		return nil, err
	}

	client := resty.New()
	client.SetBaseURL(url)
	client.SetTimeout(3 * time.Second)

	return &ServiceClient{
		Client: *client,
		close:  cancel,
	}, nil
}

func (m *Minikube) TunnelService(service metav1.Object) (string, func(), error) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := m.commandContext(ctx, "service", "--namespace", service.GetNamespace(), service.GetName(), "--url")
	cmd.Stdout = nil
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return "", nil, err
	}

	chUrl := make(chan string)
	go func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		for {
			if !scanner.Scan() {
				return
			}
			line := scanner.Text()
			_, _ = m.stdout.Write([]byte(line))
			if strings.HasPrefix(line, "http") {
				chUrl <- line
				return
			}
		}
	}(stdout)

	err = cmd.Start()
	if err != nil {
		cancel()
		return "", nil, err
	}

	chErr := make(chan error)
	go func() { chErr <- cmd.Wait() }()

	select {
	case url := <-chUrl:
		return url, cancel, nil
	case <-time.After(10 * time.Second):
		cancel()
		return "", nil, fmt.Errorf("timed out to tunnel service")
	case err = <-chErr:
		cancel()
		return "", nil, fmt.Errorf("failed to tunnel service: %w", err)
	}
}

func (m *Minikube) CreatePod(pod *acorev1.PodApplyConfiguration) (metav1.Object, error) {
	applied, err := m.Client().CoreV1().Pods("default").Apply(context.Background(), pod, metav1.ApplyOptions{FieldManager: "application/apply-patch"})
	if err != nil {
		return nil, err
	}
	if err = m.WaitForPodPhase(applied.GetObjectMeta(), corev1.PodRunning, 30*time.Second); err != nil {
		return nil, err
	}
	return applied.GetObjectMeta(), nil
}

func (m *Minikube) GetPod(pod metav1.Object) (*corev1.Pod, error) {
	return m.Client().CoreV1().Pods(pod.GetNamespace()).Get(context.Background(), pod.GetName(), metav1.GetOptions{})
}

func (m *Minikube) DeletePod(pod metav1.Object) error {
	if pod == nil {
		return nil
	}
	return m.Client().CoreV1().Pods(pod.GetNamespace()).Delete(context.Background(), pod.GetName(), metav1.DeleteOptions{GracePeriodSeconds: extutil.Ptr(int64(0))})
}

func (m *Minikube) WaitForPodPhase(pod metav1.Object, phase corev1.PodPhase, duration time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var lastStatus corev1.PodPhase
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("pod %s/%s did not reach phase %s. last status %s", pod.GetNamespace(), pod.GetName(), phase, lastStatus)
		case <-time.After(200 * time.Millisecond):
			p, err := m.Client().CoreV1().Pods(pod.GetNamespace()).Get(context.Background(), pod.GetName(), metav1.GetOptions{})
			if err == nil && p.Status.Phase == phase {
				return nil
			}
			lastStatus = p.Status.Phase
		}
	}
}

func (m *Minikube) CreateService(service *acorev1.ServiceApplyConfiguration) (metav1.Object, error) {
	applied, err := m.Client().CoreV1().Services("default").Apply(context.Background(), service, metav1.ApplyOptions{FieldManager: "application/apply-patch"})
	if err != nil {
		return nil, err
	}
	return applied.GetObjectMeta(), nil
}

func (m *Minikube) DeleteService(service metav1.Object) error {
	if service == nil {
		return nil
	}
	return m.Client().CoreV1().Services(service.GetNamespace()).Delete(context.Background(), service.GetName(), metav1.DeleteOptions{GracePeriodSeconds: extutil.Ptr(int64(0))})
}

func (m *Minikube) Exec(pod metav1.Object, containername string, cmd ...string) (string, error) {
	req := m.Client().CoreV1().RESTClient().Post().
		Namespace(pod.GetNamespace()).
		Resource("pods").
		Name(pod.GetName()).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containername,
			Command:   cmd,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(m.Config(), "POST", req.URL())
	if err != nil {
		return "", err
	}

	var outb bytes.Buffer
	err = executor.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: &outb,
		Stderr: &outb,
		Tty:    true,
	})
	return outb.String(), err
}

func (m *Minikube) PortForward(pod metav1.Object, remotePort uint16, stopCh <-chan struct{}) (uint16, error) {
	req := m.Client().CoreV1().RESTClient().Post().
		Namespace(pod.GetNamespace()).
		Resource("pods").
		Name(pod.GetName()).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(m.Config())
	if err != nil {
		return 0, err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req.URL())

	readyCh := make(chan struct{})
	forwarder, err := portforward.New(dialer, []string{fmt.Sprintf("0:%d", remotePort)}, stopCh, readyCh, m.stdout, m.stderr)
	if err != nil {
		return 0, err
	}

	chErr := make(chan error)
	go func() {
		err = forwarder.ForwardPorts()
		if err != nil {
			chErr <- err
		}
	}()

	select {
	case <-readyCh:
	case err := <-chErr:
		if err != nil {
			return 0, err
		}
	}

	ports, err := forwarder.GetPorts()
	if err != nil {
		return 0, err
	}

	for _, port := range ports {
		if port.Remote == remotePort {
			return port.Local, nil
		}
	}

	return 0, fmt.Errorf("port %d not forwarded", remotePort)
}

func (m *Minikube) ListPods(ctx context.Context, namespace, matchLabels string) ([]corev1.Pod, error) {
	list, err := m.Client().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: matchLabels})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (m *Minikube) TailLog(ctx context.Context, pod metav1.Object) {
	reader, err := m.Client().CoreV1().Pods(pod.GetNamespace()).GetLogs(pod.GetName(), &corev1.PodLogOptions{Follow: true}).Stream(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to tail logs")
	}
	defer func() { _ = reader.Close() }()
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fmt.Printf("📦%s\n", scanner.Text())
	}
}
