# Changelog

## 2.6.3

- add Experiment-Kex, Execution-ID and URIs to ExecutionContext

## 2.6.2

- add agentPid to ExecutionContext

## 2.6.1

- add `stressng-worker` to parameter type

- ## 2.6.0

- change `restrictedUrls` to []RestrictedEndpoint

## 2.5.2

- add `restrictedUrls` to ExecutionContext
- add `bitrate` parameter type
- add `AdditionalFlags` for `ActionDescription`

## 2.5.1

- fix enum constants for `QuantityRestriction`

## 2.5.0

- option to define quantity restrictions for target selections
- added more docs
- added `minValue` and `maxValue` to integer and percentage action parameters
- deprecated `ActionDescription.targetType` -> moved to `ActionDescription.targetSelection.targetType`
- deprecated `ActionDescription.targetSelectionTemplates` -> moved to `ActionDescription.targetSelection.selectionTemplates`

## 2.4.6

- add EnvVarOrUri, Header, Separator and TextArea to ParameterTypes

## 2.4.5

- remove the heartbeat endpoint ref to ActionList in favor of status calls

## 2.4.4

- added the heartbeat endpoint ref to ActionList to subscribe to the heartbeat event
- 
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

- **Breaking:** The type `ParameterOption` was renamed to `ExplicitParameterOption`. The following code snippet shows an updated usage:

```go
Options: extutil.Ptr([]action_kit_api.ParameterOption{
  action_kit_api.ExplicitParameterOption{
    Label: "Any",
    Value: "*",
  },
  action_kit_api.ParameterOptionsFromTargetAttribute{
    Attribute: "k8s.namespace",
  },
}),
```

- Ability to auto-generate supported options through the `ParameterOption` type `ParameterOptionsFromTargetAttribute`.
- Support for metric queries and responses.

## 1.0.0

- Initial release

### Differences to the deprecated AttackKit API v0.5.0

- **Breaking:** This version contains breaking changes in the ActionKit API.
    - Various APIs were renamed from `*Attack*` to `*Action*`
    - Attack description `category` is now optional and no longer an enumeration.
    - Attack's `targetType` is now optional.
    - `target` is now optional within prepare request bodies.
    - new required configuration value `kind` in preparation for action, checks load test support.
    - Several types were renamed to make it easier to under when to use them.
    - `action_kit_api.Ptr` was removed from this module to avoid requirement for Go `>1.18`.
    - Enum names for HTTP methods were changed to avoid context specific prefixes.
    - Aligned all enum values to lower case (but upper case is accepted as well).
- Added the ability to upload artifacts to Steadybit as part of the `prepare`,
  `start`, `status` and `stop` endpoints.
- Added `warn` level for `Message`.
