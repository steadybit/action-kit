# Changelog

## 1.0.2

- Add signal handler that stops active actions on SIGTERM, SIGINT and SIGUSR1
- Augment ActionWithStop with a status endpoint. To report stops by the extension.
- When status requests are missing 4 times in a row we consider the connection to the agent as broken and stop active actions.

## 1.0.1

- Log heartbeat request on debug level
- Fix bug in heartbeat and signal handling


## 1.0.0

- Initial release

