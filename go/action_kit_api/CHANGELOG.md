# Changelog

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
