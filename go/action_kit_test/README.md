# ActionKit Go Test SDK

This module contains helper and interfaces which will help you to test actions using
the [action kit go api](https://github.com/steadybit/action-kit/tree/main/go/action_kit_api).

The module encapsulates the following technical aspects:

- test utilities to test an extension using minikube as kubernetes cluster

## Installation

Add the following to your `go.mod` file:

```
go get github.com/steadybit/action-kit/go/action_kit_test
```

## Usage

````go
func TestWithMinikube(t *testing.T) {
	extFactory := e2e.HelmExtensionFactory{
		Name: "extension-host",
		Port: 8085,
		ExtraArgs: func(m *e2e.Minikube) []string {
			return []string{
				"--set", fmt.Sprintf("container.runtime=%s", m.Runtime),
			}
		},
	}

	mOpts := e2e.DefaultMiniKubeOpts
	e2e.WithMinikube(t, mOpts, &extFactory, []e2e.WithMinikubeTestCase{
		{
			Name: "stress cpu",
			Test: testStressCpu,
		},
    })
}


func testStressCpu(t *testing.T, m *e2e.Minikube, e *e2e.Extension) {
	log.Info().Msg("Starting testStressCpu")
	config := struct {
		Duration int `json:"duration"`
		CpuLoad  int `json:"cpuLoad"`
		Workers  int `json:"workers"`
	}{Duration: 50000, Workers: 0, CpuLoad: 50}
	exec, err := e.RunAction("com.github.steadybit.extension_host.stress-cpu", getTarget(m), config, nil)
	require.NoError(t, err)
    // ...
	e2e.AssertProcessRunningInContainer(t, m, e.Pod, "steadybit-extension-host", "stress-ng", true)
	require.NoError(t, exec.Cancel())
}
````

## Coverage

The module contains a helper to download the coverage data from the extension host and convert it to the required format for sonarqube.

If you like to use this feature, you need to:

- Compile the extension for the e2e test with the following flags: `-cover`
  - For example by adding `ADDITIONAL_BUILD_PARAMS` as an arg to your `Dockerfile`
  - and adding the parameter to the `Makefile`'s `container` target: `docker build --build-arg ADDITIONAL_BUILD_PARAMS="-cover" -t extension-foo:latest .`
- Add endpoints to your extension which allows the e2e-test-runtime to download the coverage data: `action_kit_sdk.RegisterCoverageEndpoints()`
- Add the new coverage-output to the `sonar-project.properties`: Example `sonar.go.coverage.reportPaths=coverage.out,e2e/e2e-coverage-docker.out`
- Add the new coverage-output to `.gitignore`