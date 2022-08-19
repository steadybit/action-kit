# How To Write An Attack Extension

This how-to article will teach you how to write an extension using ActionKit that adds new attack capabilities. We will look closely at existing extensions to learn about semantic conventions, best practices, expected behavior and necessary boilerplate.

The article assumes that you have read the [overview documentation](../action-api.md#overview) for the Action API and possibly skimmed over the expected API endpoints. We are leveraging the Go programming language within the examples, but you can use every other language as long as you adhere to the expected API.

## Necessary Boilerplate

Every extension needs boilerplate code to start an HTTP server, initialize logging and register the HTTP handlers that comply with the expected API. The following excerpt shows how the go-kubectl example extension is doing this.

https://github.com/steadybit/action-kit/blob/128d8c05bdadb54e8b001391ead530e22d2d17a3/examples/go-kubectl/main.go#L14-L30

The excerpt above shows an extension leveraging our ExtensionKit, e.g., to register HTTP handlers or initialize the logging system. ExtensionKit makes authoring Steadybit extensions easier through utilities that help you comply with the expected behavior of extensions.

Note the HTTP endpoints' paths. You can choose these paths freely. The Steadybit agent only needs to know about the entry point into the extension. In this case, that would be {{origin}}/actions.

## Action List

Let us start with the first API implementation: The list of supported actions. This endpoint is expected to provide a list of all actions that the extension supports. Note that an attack is a special kind of action.

<p align="center">
    <img src="./img/action-attack.excalidraw.png" width="150" alt="UML class diagram depicting that an Attack is also an Action (Attack inherits from Action)">
</p>

The attack list API endpoint's response body needs to be a JSON encoded list of HTTP endpoints that the Steadybit agent can call to learn more about each action.

https://github.com/steadybit/action-kit/blob/128d8c05bdadb54e8b001391ead530e22d2d17a3/examples/go-kubectl/handlers.go#L20-L29

## Action Description

This is where the fun begins! The action description HTTP endpoint needs to expose information about the UI presentation of the attack, how end-users can configure it and which endpoints to call to prepare/start/stop the attack.

https://github.com/steadybit/action-kit/blob/128d8c05bdadb54e8b001391ead530e22d2d17a3/examples/go-kubectl/handlers.go#L31-L41

The excerpt above shows fundamental options for every action. You can learn more about these within the [action API documentation page](../action-api.md#action-description). Within this document, we are focussing on best practices specific to attacks.

- `kind`: Must be set to `attack`. This option controls the visual appearance, grouping and labeling within the Steadybit user interface.
- `targetType`: Attacks typically operate on a target. So you almost certainly want to specify a target type in here. You can learn about available target types through the Steadybit user interface via `Settings -> Extensions -> Target Types`.

https://github.com/steadybit/action-kit/blob/128d8c05bdadb54e8b001391ead530e22d2d17a3/examples/go-kubectl/handlers.go#L42-L50

There are no special parameter contracts for attacks. So within this area, you will define them just like any other action. Also, refer to our [parameter types documentation](../parameter-types.md) to learn more about the supported parameter types.

https://github.com/steadybit/action-kit/blob/128d8c05bdadb54e8b001391ead530e22d2d17a3/examples/go-kubectl/handlers.go#L51-L66