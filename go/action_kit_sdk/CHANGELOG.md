# Changelog

## 1.1.5

- Update to action-kit api 2.7.0

## 1.1.4

- report extension shutdown as error and not as failure

## 1.1.3

- Added endpoints to fetch coverage results

## 1.1.2
 
- Fix Status-Endpoint registration and check for Status-Endpoint if TimeControl "Internal" is used

## 1.1.1

- Handle `file` parameters: Download files from the platform, store them in a temporary directory and pass the path to the action in the PrepareActionRequest.

## 1.1.0

- QueryMetricsRequestBody was missing in QueryMetrics-interface

## 1.0.2

- Add signal handler that stops active actions on SIGTERM, SIGINT and SIGUSR1
- Augment ActionWithStop with a status endpoint. To report stops by the extension.
- When status requests are missing 4 times in a row we consider the connection to the agent as broken and stop active actions.

## 1.0.1

- Log heartbeat request on debug level
- Fix bug in heartbeat and signal handling


## 1.0.0

- Initial release

