# ActionKit Go SDK

This module contains helper and interfaces which will help you to implement actions using
the [action kit go api](https://github.com/steadybit/action-kit/tree/main/go/action_kit_api).

The module encapsulates the following technical aspects:

- JSON marshalling and unmarshalling of action inputs and outputs
- The sdk will wrap around your `describe` call and will provide some meaningful defaults for your endpoint definitions.
- An additional layer of rollback stability. The SDK will keep a copy of your action state in memory to be able to roll back to the previous state in case
  of connections issues.
- Automatic handling of `file` parameters. The SDK will automatically download the file, store it in a temporary directory and delete the file after the action
  has stopped. The `Config`-map in `action_kit_api.PrepareActionRequestBody` will contain the path to the downloaded file.

## Installation

Add the following to your `go.mod` file:

```
go get github.com/steadybit/action-kit/go/action_kit_sdk
```

## Usage

1. Implement at least the `action_kit_sdk.Action` interface:
    - Examples:
        - [go/action_kit_sdk/example_action_test.go](./example_action_test.go)

2. Implement other interfaces if you need them:
    - `action_kit_sdk.ActionWithStatus`
    - `action_kit_sdk.ActionWithStop`
    - `action_kit_sdk.ActionWithMetricQuery`

3. Register your action:
   ```go
   action_kit_sdk.RegisterAction(NewRolloutRestartAction())
   ```

4. Add your registered actions to the index endpoint of your extension:
   ```go
   exthttp.RegisterHttpHandler("/actions", exthttp.GetterAsHandler(action_kit_sdk.GetActionList))
   ```