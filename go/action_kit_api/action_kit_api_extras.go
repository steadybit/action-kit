package action_kit_api

type ParameterOption interface{}
type Widget interface{}
type LineChartWidgetGroupMatcher interface{}
type ExecutionModification interface{}

const (
	// Deprecated: Use action_kit_api.ActionParameterTypeBitrate instead.
	Bitrate ActionParameterType = "bitrate"
	// Deprecated: Use action_kit_api.ActionParameterTypeBoolean instead.
	Boolean ActionParameterType = "boolean"
	// Deprecated: Use action_kit_api.ActionParameterTypeDuration instead.
	Duration ActionParameterType = "duration"
	// Deprecated: Use action_kit_api.ActionParameterTypeFile instead.
	File ActionParameterType = "file"
	// Deprecated: Use action_kit_api.ActionParameterTypeHeader instead.
	Header ActionParameterType = "header"
	// Deprecated: Use action_kit_api.ActionParameterTypeInteger instead.
	Integer ActionParameterType = "integer"
	// Deprecated: Use action_kit_api.ActionParameterTypeKeyValue instead.
	KeyValue ActionParameterType = "key_value"
	// Deprecated: Use action_kit_api.ActionParameterTypePercentage instead.
	Percentage ActionParameterType = "percentage"
	// Deprecated: Use action_kit_api.ActionParameterTypeRegex instead.
	Regex ActionParameterType = "regex"
	// Deprecated: Use action_kit_api.ActionParameterTypeSeparator instead.
	Separator ActionParameterType = "separator"
	// Deprecated: Use action_kit_api.ActionParameterTypeStressngWorkers instead.
	StressngWorkers ActionParameterType = "stressng-workers"
	// Deprecated: Use action_kit_api.ActionParameterTypeString instead.
	String ActionParameterType = "string"
	// Deprecated: Use action_kit_api.ActionParameterTypeString1 instead.
	String1 ActionParameterType = "string[]"
	// Deprecated: Use action_kit_api.ActionParameterTypeStringArray instead.
	StringArray ActionParameterType = "string_array"
	// Deprecated: Use action_kit_api.ActionParameterTypeStringTextarea instead.
	Textarea ActionParameterType = "textarea"
	// Deprecated: Use action_kit_api.ActionParameterTypeStringUrl instead.
	Url ActionParameterType = "url"

	// Deprecated: Use action_kit_api.QuantityRestrictionAll instead.
	All TargetSelectionQuantityRestriction = "all"
	// Deprecated: Use action_kit_api.QuantityRestrictionExactlyOne instead.
	ExactlyOne TargetSelectionQuantityRestriction = "exactly_one"
	// Deprecated: Use action_kit_api.QuantityRestrictionNone instead.
	None TargetSelectionQuantityRestriction = "none"
)
