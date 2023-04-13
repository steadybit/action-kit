# Changelog

## 2.4.6

- add EnvVarOrUri, Header, Separator and TextArea to ParameterTypes
-
## 2.4.5

- remove the heartbeat endpoint ref to ActionList in favor of status calls

## 2.4.4

- added the heartbeat endpoint ref to ActionList to subscribe to the heartbeat event

## 2.4.3

- added the execution id to the StartRequest, StatusRequest and StopRequest

## 2.4.2

- added ExecutonContext to PrepareRequest
  The execution context of the action.
- moved agentAwsAccountId to ExecutonContext
  The AWS account ID of the agent that is executing the action.
  This is only set when the action is executed on an AWS
  account.

## 2.4.1
- added agentAwsAccountId to PrepareRequest
  The AWS account ID of the agent that is executing the action.
  This is only set when the action is executed on an AWS
  account.

## 2.4.0

- added support for action hints
- added support for action parameter hints

## 2.3.0

- added support for execution view widget `log`
- added support for predefined widgets in the platform

## 2.2.0

- added support for execution view widget `state over time`

## 2.1.0

 - added support for target selection templates

## 2.0.1

 - Improved error-handling of extensions. Extensions are now able end with `failed` or `errored`. Extensions can also return an error within any response without
  the need to use a http status code != `200`. You can find more detailed descriptions about extension error handling in the docs.

## 2.0.0

 - **Breaking:** The type `ParameterOption` was renamed to `ExplicitParameterOption`.
 - Ability to auto-generate supported options through the `ParameterOption` type `ParameterOptionsFromTargetAttribute`.
 - Support for metric queries and responses.

## 1.0.0

 - Initial release
