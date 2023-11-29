# Changelog

## 1.2.5

- WaitForDeploymentPhase checks all pods for reaching the wanted phase

## 1.2.4

- wait on coredns to be ready before running tests

## 1.2.3

- fix livelock introduced in 1.2.2

## 1.2.2

- don't limit extension log to 10s

## 1.2.1

- fix data race

## 1.2.0

- Add schema validation to action endpoints

## 1.1.1

- reference image name using full repository reference

## 1.1.0

- refactor hooks in WithMinikube
- refactor to use the test discovery client

## 1.0.15

- fix: ActionExecution.Cancel may hang if action failed in preparation

## 1.0.14

- don't swallow errors from action cancel

## 1.0.13

- added support for `enrichmentData` in discoveries

## 1.0.12

## 1.0.11

- added function to get metrics of status calls

## 1.0.9

- Update to action-kit api 2.7.0

## 1.0.8

- add method iperf testing
- 
## 1.0.7

- get logs of failing pods during startup (don't use `--wait` for `helm install`, instead poll pods and grab logs manually)

## 1.0.6

- add e2e before and after hooks

## 1.0.5

- export method for executing ssh command
- incorrect handling of failure from status

## 1.0.4

- convert coverage data to support sonarqube

## 1.0.3

- try to download coverage data from extensions after e2e tests

## 1.0.2

- fix: AssertLogContains produces endless loop if timeout reached
- added AssertLogContainsWithTimeout

## 1.0.1

- print response body on error

## 1.0.0

- Initial release

