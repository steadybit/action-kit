# Changelog

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

