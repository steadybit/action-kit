# Changelog

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
   - Aligned all enum values to  lower case (but upper case is accepted as well).
 - Added the ability to upload artifacts to Steadybit as part of the `prepare`,
   `start`, `status` and `stop` endpoints.
 - Added `warn` level for `Message`.
