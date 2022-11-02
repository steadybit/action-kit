# Changelog

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
