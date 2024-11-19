package action_kit_api

type ParameterOption interface{}
type Widget interface{}
type LineChartWidgetGroupMatcher interface{}

const (
	// Bitrate - Bitrate parameter type.
	// Deprecated: Use action_kit_api.ActionParameterTypeBitrate instead.
	Bitrate ActionParameterType = "bitrate"
	// Boolean - Boolean parameter type.
	// Deprecated: Use action_kit_api.ActionParameterTypeBoolean instead.
	Boolean ActionParameterType = "boolean"
	// Duration - Duration parameter type.
	// Deprecated: Use action_kit_api.ActionParameterTypeDuration instead.
	Duration ActionParameterType = "duration"
	// File - File parameter type.
	// Deprecated: Use action_kit_api.ActionParameterTypeFile instead.
	File ActionParameterType = "file"
	// Header - Header parameter type.
	// Deprecated: Use action_kit_api.ActionParameterTypeHeader instead.
	Header ActionParameterType = "header"
	// Integer - Integer parameter type.
	// Deprecated: Use action_kit_api.ActionParameterTypeInteger instead.
	Integer ActionParameterType = "integer"
	// KeyValue - KeyValue parameter type.
	// Deprecated: Use action_kit_api.ActionParameterTypeKeyValue instead.
	KeyValue ActionParameterType = "key_value"
	// Percentage - Percentage parameter type.
	// Deprecated: Use action_kit_api.ActionParameterTypePercentage instead.
	Percentage ActionParameterType = "percentage"
	// Regex - Regex parameter type.
	// Deprecated: Use action_kit_api.ActionParameterTypeRegex instead.
	Regex ActionParameterType = "regex"
	// Separator - Separator parameter type.
	// Deprecated: Use action_kit_api.ActionParameterTypeSeparator instead.
	Separator ActionParameterType = "separator"
	// StressngWorkers - StressngWorkers parameter type.
	// Deprecated: Use action_kit_api.ActionParameterTypeStressngWorkers instead.
	StressngWorkers ActionParameterType = "stressng-workers"
	// String - String parameter type.
	// Deprecated: Use action_kit_api.ActionParameterTypeString instead.
	String ActionParameterType = "string"
	// String1 - String1 parameter type.
	// Deprecated: Use action_kit_api.ActionParameterTypeString1 instead.
	String1 ActionParameterType = "string[]"
	// StringArray - StringArray parameter type.
	// Deprecated: Use action_kit_api.ActionParameterTypeStringArray instead.
	StringArray ActionParameterType = "string_array"
	// Textarea - StringTextarea parameter type.
	// Deprecated: Use action_kit_api.ActionParameterTypeStringTextarea instead.
	Textarea ActionParameterType = "textarea"
	// Url - Url parameter type.
	// Deprecated: Use action_kit_api.ActionParameterTypeStringUrl instead.
	Url ActionParameterType = "url"
)
