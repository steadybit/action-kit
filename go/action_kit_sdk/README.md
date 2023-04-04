# ActionKit Go SDK

This module contains helper and interfaces which will help you to implement actions using
the [action kit go api](https://github.com/steadybit/action-kit/tree/main/go/action_kit_api).

The module encapsulates the following technical aspects:

- JSON marshalling and unmarshalling of action inputs and outputs
- The sdk will wrap around your `describe` call and will provide some meaningful defaults for your endpoint definitions.
- An additional layer of rollback stability. The SDK will keep a copy of your action state in memory to be able to rollback to the previous state in case
  of connections issues.

## Installation

Add the following to your `go.mod` file:

```
go get github.com/steadybit/action-kit/go/action_kit_sdk
```

## Usage

1. Implement at least the `action_kit_sdk.Action` interface:
    - Examples:
        - [examples/go-kubectl-using-sdk/action.go](../../examples/go-kubectl-using-sdk/action.go)
        - [go/action_kit_sdk/test/example_action.go](./test/example_action.go)

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
   exthttp.RegisterHttpHandler("/actions", exthttp.GetterAsHandler(func() action_kit_api.ActionList {
     return action_kit_api.ActionList{
         Actions: action_kit_sdk.RegisteredActionsEndpoints(),
     }
   }))
   ```