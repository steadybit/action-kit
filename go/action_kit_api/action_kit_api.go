// Package action_kit_api provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/deepmap/oapi-codegen version v1.11.1-0.20220629212257-2cf7fcf5b26d DO NOT EDIT.
package action_kit_api

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Defines values for ActionDescriptionKind.
const (
	Attack   ActionDescriptionKind = "attack"
	Check    ActionDescriptionKind = "check"
	LoadTest ActionDescriptionKind = "load_test"
	Other    ActionDescriptionKind = "other"
)

// Defines values for ActionDescriptionTimeControl.
const (
	External      ActionDescriptionTimeControl = "external"
	Instantaneous ActionDescriptionTimeControl = "instantaneous"
	Internal      ActionDescriptionTimeControl = "internal"
)

// Defines values for ActionHintType.
const (
	HintInfo    ActionHintType = "hint_info"
	HintWarning ActionHintType = "hint_warning"
)

// Defines values for ActionKitErrorStatus.
const (
	Errored ActionKitErrorStatus = "errored"
	Failed  ActionKitErrorStatus = "failed"
)

// Defines values for ActionParameterType.
const (
	Boolean     ActionParameterType = "boolean"
	Duration    ActionParameterType = "duration"
	File        ActionParameterType = "file"
	Integer     ActionParameterType = "integer"
	KeyValue    ActionParameterType = "key_value"
	Password    ActionParameterType = "password"
	Percentage  ActionParameterType = "percentage"
	String      ActionParameterType = "string"
	String1     ActionParameterType = "string[]"
	StringArray ActionParameterType = "string_array"
)

// Defines values for DescribingEndpointReferenceMethod.
const (
	Get DescribingEndpointReferenceMethod = "get"
)

// Defines values for LogWidgetType.
const (
	ComSteadybitWidgetLog LogWidgetType = "com.steadybit.widget.log"
)

// Defines values for MessageLevel.
const (
	Debug MessageLevel = "debug"
	Error MessageLevel = "error"
	Info  MessageLevel = "info"
	Warn  MessageLevel = "warn"
)

// Defines values for MutatingHttpMethod.
const (
	Delete MutatingHttpMethod = "delete"
	Post   MutatingHttpMethod = "post"
	Put    MutatingHttpMethod = "put"
)

// Defines values for PredefinedWidgetType.
const (
	ComSteadybitWidgetPredefined PredefinedWidgetType = "com.steadybit.widget.predefined"
)

// Defines values for StateOverTimeWidgetType.
const (
	ComSteadybitWidgetStateOverTime StateOverTimeWidgetType = "com.steadybit.widget.state_over_time"
)

// Provides details about a possible action, e.g., what configuration options it has, how to present it to end-users and how to trigger the action.
type ActionDescription struct {
	// Used for categorization of the action within user interfaces.
	Category *string `json:"category,omitempty"`

	// Description for end-users to help them understand what the action is doing.
	Description string      `json:"description"`
	Hint        *ActionHint `json:"hint,omitempty"`

	// An icon that is used to identify your action in the ui. Needs to be a data-uri containing an image.
	Icon *string `json:"icon,omitempty"`

	// A technical ID that is used to uniquely identify this type of action. You will typically want to use something like `org.example.my-fancy-attack`.
	Id string `json:"id"`

	// Describes what kind of action this is. This is used to help users understand and classify the various runnable actions that are part of an experiment.
	Kind ActionDescriptionKind `json:"kind"`

	// A human-readable label for the action.
	Label      string                `json:"label"`
	Metrics    *MetricsConfiguration `json:"metrics,omitempty"`
	Parameters []ActionParameter     `json:"parameters"`

	// HTTP endpoint which the Steadybit platform/agent could communicate with.
	Prepare MutatingEndpointReference `json:"prepare"`

	// HTTP endpoint which the Steadybit platform/agent could communicate with.
	Start MutatingEndpointReference `json:"start"`

	// HTTP endpoint which the Steadybit platform/agent could communicate with.
	Status *MutatingEndpointReferenceWithCallInterval `json:"status,omitempty"`

	// HTTP endpoint which the Steadybit platform/agent could communicate with.
	Stop                     *MutatingEndpointReference `json:"stop,omitempty"`
	TargetSelectionTemplates *TargetSelectionTemplates  `json:"targetSelectionTemplates,omitempty"`

	// What target type this action should be offered for. Matches the `id` field within `DescribeTargetTypeResponse` within DiscoveryKit.
	TargetType *string `json:"targetType,omitempty"`

	// Actions can either be an instantaneous event, e.g., the restart of a host, or an activity spanning over an unspecified duration. For those actions having a duration, we differentiate between internally, e.g., waiting for a deployment to finish, and externally, e.g., waiting for a user-specified time to pass, controlled durations.
	TimeControl ActionDescriptionTimeControl `json:"timeControl"`

	// The version of the action. Remember to increase the value everytime you update the definitions. The platform will ignore any definition changes with the same action version. We do recommend usage of semver strings.
	Version string   `json:"version"`
	Widgets *Widgets `json:"widgets,omitempty"`
}

// Describes what kind of action this is. This is used to help users understand and classify the various runnable actions that are part of an experiment.
type ActionDescriptionKind string

// Actions can either be an instantaneous event, e.g., the restart of a host, or an activity spanning over an unspecified duration. For those actions having a duration, we differentiate between internally, e.g., waiting for a deployment to finish, and externally, e.g., waiting for a user-specified time to pass, controlled durations.
type ActionDescriptionTimeControl string

// ActionHint defines model for ActionHint.
type ActionHint struct {
	// The actual hint text (can contain markdown). Will be displayed in the product UI when configuring the action.
	Content string `json:"content"`

	// Will be used in the product UI to display the hint in a different color and with a different icon.
	Type ActionHintType `json:"type"`
}

// Will be used in the product UI to display the hint in a different color and with a different icon.
type ActionHintType string

// An enhanced version of RFC 7807 Problem Details for HTTP APIs compliant response body for error scenarios
type ActionKitError struct {
	// A human-readable explanation specific to this occurrence of the problem.
	Detail *string `json:"detail,omitempty"`

	// A URI reference that identifies the specific occurrence of the problem.
	Instance *string `json:"instance,omitempty"`

	// * failed - The action has detected some failures, for example a failing test which has been implemented by the action. The action will be stopped, if this status is returned by the status endpoint. * errored - There was a technical error while executing the action. Will be marked as red in the platform. The action will be stopped, if this status is returned by the status endpoint.
	Status *ActionKitErrorStatus `json:"status,omitempty"`

	// A short, human-readable summary of the problem type.
	Title string `json:"title"`

	// A URI reference that identifies the problem type.
	Type *string `json:"type,omitempty"`
}

// * failed - The action has detected some failures, for example a failing test which has been implemented by the action. The action will be stopped, if this status is returned by the status endpoint. * errored - There was a technical error while executing the action. Will be marked as red in the platform. The action will be stopped, if this status is returned by the status endpoint.
type ActionKitErrorStatus string

// Lists all actions that the platform/agent could execute.
type ActionList struct {
	Actions []DescribingEndpointReference `json:"actions"`
}

// ActionParameter defines model for ActionParameter.
type ActionParameter struct {
	// Unique file type specifiers describing what type of files are accepted for parameters of type 'file'.
	AcceptedFileTypes *[]string `json:"acceptedFileTypes,omitempty"`

	// Whether this parameter should be placed under the expandable advanced section within the user interface.
	Advanced *bool `json:"advanced,omitempty"`

	// A default value for this parameter. This value will be used if the user does not specify a value for this parameter.
	DefaultValue *string `json:"defaultValue,omitempty"`

	// Description for end-users to help them understand the action parameter.
	Description *string     `json:"description,omitempty"`
	Hint        *ActionHint `json:"hint,omitempty"`

	// A human-readable label for the action parameter.
	Label string `json:"label"`

	// The key under which the action parameter is stored. This key can then be found within the prepare request's config field.
	Name string `json:"name"`

	// Optional options for the `string`, `string[]` and `string_array` parameter types. Which result in suggestions for end-users.
	Options *[]ParameterOption `json:"options,omitempty"`

	// You can define this fields to order the parameters in the user interface. The lower the value, the higher the position.
	Order *int `json:"order,omitempty"`

	// Whether or not end-users need to specify a value for this parameter.
	Required *bool `json:"required,omitempty"`

	// What kind of value this parameter is capturing. The type selection influences the `config` passed as part of the `PrepareRequest`. It also results in improved user-interface elements.
	Type ActionParameterType `json:"type"`
}

// What kind of value this parameter is capturing. The type selection influences the `config` passed as part of the `PrepareRequest`. It also results in improved user-interface elements.
type ActionParameterType string

// Any kind of action specific state that will be passed to the next endpoints.
type ActionState map[string]interface{}

// Actions may choose to provide artifacts (arbitrary files) that are later accessible by users when inspecting experiment execution details. This comes in handy to expose load test reports and similar data.
type Artifact struct {
	// base64 encoded data
	Data string `json:"data"`

	// Human-readable label for the artifact. We recommend to include file extensions within the label for a better user-experience when downloading these artifacts, e.g., load_test_result.tar.gz.
	Label string `json:"label"`
}

// Artifacts defines model for Artifacts.
type Artifacts = []Artifact

// HTTP endpoint which the Steadybit platform/agent could communicate with.
type DescribingEndpointReference struct {
	// HTTP method to use when calling the HTTP endpoint.
	Method DescribingEndpointReferenceMethod `json:"method"`

	// Absolute path of the HTTP endpoint.
	Path string `json:"path"`
}

// HTTP method to use when calling the HTTP endpoint.
type DescribingEndpointReferenceMethod string

// You can use an explicit/fixed parameter option for a known / finite set of options that never change.
type ExplicitParameterOption struct {
	// A human-readable label describing this option.
	Label string `json:"label"`

	// The technical value which will be passed to the action as part of the `config` object.
	Value string `json:"value"`
}

// LogWidget defines model for LogWidget.
type LogWidget struct {
	LogType string        `json:"logType"`
	Title   string        `json:"title"`
	Type    LogWidgetType `json:"type"`
}

// LogWidgetType defines model for LogWidget.Type.
type LogWidgetType string

// Log-message that will be passed to the platform (default agent log).
type Message struct {
	// Any kind of action specific fields that will be rendered in the platform tooltip of LogWidget
	Fields    *MessageFields `json:"fields,omitempty"`
	Level     *MessageLevel  `json:"level,omitempty"`
	Message   string         `json:"message"`
	Timestamp *time.Time     `json:"timestamp,omitempty"`
	Type      *string        `json:"type,omitempty"`
}

// MessageLevel defines model for Message.Level.
type MessageLevel string

// Any kind of action specific fields that will be rendered in the platform tooltip of LogWidget
type MessageFields map[string]string

// Log-messages that will be passed to the platform (default agent log).
type Messages = []Message

// Metrics can be exposed by actions. These metrics can then be leveraged by end-users to inspect system behavior and to optionally abort experiment execution when certain metrics are observed, i.e., metrics can act as (steady state) checks.
type Metric struct {
	// Key/value pairs describing the metric. This type is modeled after Prometheus' data model, i.e., metric labels. You may encode the metric name as `__name__` similar to how Prometheus does it.
	Metric map[string]string `json:"metric"`

	// Metric name. You can alternatively encode the metric name as `__name__` within the metric property.
	Name *string `json:"name,omitempty"`

	// Timestamp describing at which moment the value was observed.
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// Metrics defines model for Metrics.
type Metrics = []Metric

// MetricsConfiguration defines model for MetricsConfiguration.
type MetricsConfiguration struct {
	Query *MetricsQueryConfiguration `json:"query,omitempty"`
}

// MetricsQueryConfiguration defines model for MetricsQueryConfiguration.
type MetricsQueryConfiguration struct {
	// HTTP endpoint which the Steadybit platform/agent could communicate with.
	Endpoint   MutatingEndpointReferenceWithCallInterval `json:"endpoint"`
	Parameters []ActionParameter                         `json:"parameters"`
}

// HTTP endpoint which the Steadybit platform/agent could communicate with.
type MutatingEndpointReference struct {
	Method MutatingHttpMethod `json:"method"`

	// Absolute path of the HTTP endpoint.
	Path string `json:"path"`
}

// MutatingEndpointReferenceWithCallInterval defines model for MutatingEndpointReferenceWithCallInterval.
type MutatingEndpointReferenceWithCallInterval struct {
	// At what frequency should the state endpoint be called? Takes durations in the format of `100ms` or `10s`.
	CallInterval *string            `json:"callInterval,omitempty"`
	Method       MutatingHttpMethod `json:"method"`

	// Absolute path of the HTTP endpoint.
	Path string `json:"path"`
}

// MutatingHttpMethod defines model for MutatingHttpMethod.
type MutatingHttpMethod string

// A meta option that represents all target attribute values for the key defined through the attribute field.
type ParameterOptionsFromTargetAttribute struct {
	// Target attribute key from which the possible parameter options are gathered.
	Attribute string `json:"attribute"`
}

// PredefinedWidget defines model for PredefinedWidget.
type PredefinedWidget struct {
	PredefinedWidgetId string               `json:"predefinedWidgetId"`
	Type               PredefinedWidgetType `json:"type"`
}

// PredefinedWidgetType defines model for PredefinedWidget.Type.
type PredefinedWidgetType string

// PrepareResult defines model for PrepareResult.
type PrepareResult struct {
	Artifacts *Artifacts `json:"artifacts,omitempty"`

	// An enhanced version of RFC 7807 Problem Details for HTTP APIs compliant response body for error scenarios
	Error *ActionKitError `json:"error,omitempty"`

	// Log-messages that will be passed to the platform (default agent log).
	Messages *Messages `json:"messages,omitempty"`
	Metrics  *Metrics  `json:"metrics,omitempty"`

	// Any kind of action specific state that will be passed to the next endpoints.
	State ActionState `json:"state"`
}

// QueryMetricsResult defines model for QueryMetricsResult.
type QueryMetricsResult struct {
	Artifacts *Artifacts `json:"artifacts,omitempty"`

	// Log-messages that will be passed to the platform (default agent log).
	Messages *Messages `json:"messages,omitempty"`
	Metrics  *Metrics  `json:"metrics,omitempty"`
}

// StartResult defines model for StartResult.
type StartResult struct {
	Artifacts *Artifacts `json:"artifacts,omitempty"`

	// An enhanced version of RFC 7807 Problem Details for HTTP APIs compliant response body for error scenarios
	Error *ActionKitError `json:"error,omitempty"`

	// Log-messages that will be passed to the platform (default agent log).
	Messages *Messages `json:"messages,omitempty"`
	Metrics  *Metrics  `json:"metrics,omitempty"`

	// Any kind of action specific state that will be passed to the next endpoints.
	State *ActionState `json:"state,omitempty"`
}

// StateOverTimeWidget defines model for StateOverTimeWidget.
type StateOverTimeWidget struct {
	Identity StateOverTimeWidgetIdentityConfig `json:"identity"`
	Label    StateOverTimeWidgetLabelConfig    `json:"label"`
	State    StateOverTimeWidgetStateConfig    `json:"state"`
	Title    string                            `json:"title"`
	Tooltip  StateOverTimeWidgetTooltipConfig  `json:"tooltip"`
	Type     StateOverTimeWidgetType           `json:"type"`
	Url      *StateOverTimeWidgetUrlConfig     `json:"url,omitempty"`
	Value    *StateOverTimeWidgetValueConfig   `json:"value,omitempty"`
}

// StateOverTimeWidgetType defines model for StateOverTimeWidget.Type.
type StateOverTimeWidgetType string

// StateOverTimeWidgetIdentityConfig defines model for StateOverTimeWidgetIdentityConfig.
type StateOverTimeWidgetIdentityConfig struct {
	From string `json:"from"`
}

// StateOverTimeWidgetLabelConfig defines model for StateOverTimeWidgetLabelConfig.
type StateOverTimeWidgetLabelConfig struct {
	From string `json:"from"`
}

// StateOverTimeWidgetStateConfig defines model for StateOverTimeWidgetStateConfig.
type StateOverTimeWidgetStateConfig struct {
	From string `json:"from"`
}

// StateOverTimeWidgetTooltipConfig defines model for StateOverTimeWidgetTooltipConfig.
type StateOverTimeWidgetTooltipConfig struct {
	From string `json:"from"`
}

// StateOverTimeWidgetUrlConfig defines model for StateOverTimeWidgetUrlConfig.
type StateOverTimeWidgetUrlConfig struct {
	From *string `json:"from,omitempty"`
}

// StateOverTimeWidgetValueConfig defines model for StateOverTimeWidgetValueConfig.
type StateOverTimeWidgetValueConfig struct {
	// Set to true to hide the metric value within the widget. This is useful when only the translated state information is relevant.
	Hide *bool `json:"hide,omitempty"`
}

// StatusResult defines model for StatusResult.
type StatusResult struct {
	Artifacts *Artifacts `json:"artifacts,omitempty"`

	// the agent will continue to poll the status endpoint as long as completed is false
	Completed bool `json:"completed"`

	// An enhanced version of RFC 7807 Problem Details for HTTP APIs compliant response body for error scenarios
	Error *ActionKitError `json:"error,omitempty"`

	// Log-messages that will be passed to the platform (default agent log).
	Messages *Messages `json:"messages,omitempty"`
	Metrics  *Metrics  `json:"metrics,omitempty"`

	// Any kind of action specific state that will be passed to the next endpoints.
	State *ActionState `json:"state,omitempty"`
}

// StopResult defines model for StopResult.
type StopResult struct {
	Artifacts *Artifacts `json:"artifacts,omitempty"`

	// An enhanced version of RFC 7807 Problem Details for HTTP APIs compliant response body for error scenarios
	Error *ActionKitError `json:"error,omitempty"`

	// Log-messages that will be passed to the platform (default agent log).
	Messages *Messages `json:"messages,omitempty"`
	Metrics  *Metrics  `json:"metrics,omitempty"`
}

// The target on which to act on as identified by a discovery.
type Target struct {
	// These attributes include detailed information about the target provided through the discovery. These attributes are typically used as additional parameters within the action implementation.
	Attributes map[string][]string `json:"attributes"`
	Name       string              `json:"name"`
}

// Users that want to configure an action with a targetType need to define a target selection through the query UI or query language. Extensions can define selection templates to help users define such target selections.
type TargetSelectionTemplate struct {
	// Longer target selection template description. For example, to explain the template's purpose.
	Description *string `json:"description,omitempty"`

	// Human-readable short label.
	Label string `json:"label"`

	// The target selection query is defined using Steadybit's query language. For example:
	//   aws.account="" AND aws.zone.id=""
	// For more information about the query language, please inspect Steadybit's documentation:
	//   https://docs.steadybit.com/use-steadybit/query-language
	Query string `json:"query"`
}

// TargetSelectionTemplates defines model for TargetSelectionTemplates.
type TargetSelectionTemplates = []TargetSelectionTemplate

// Widgets defines model for Widgets.
type Widgets = []Widget

// ActionListResponse defines model for ActionListResponse.
type ActionListResponse struct {
	union json.RawMessage
}

// ActionStatusResponse defines model for ActionStatusResponse.
type ActionStatusResponse struct {
	union json.RawMessage
}

// DescribeActionResponse defines model for DescribeActionResponse.
type DescribeActionResponse struct {
	union json.RawMessage
}

// PrepareActionResponse defines model for PrepareActionResponse.
type PrepareActionResponse struct {
	union json.RawMessage
}

// QueryMetricsResponse defines model for QueryMetricsResponse.
type QueryMetricsResponse struct {
	union json.RawMessage
}

// StartActionResponse defines model for StartActionResponse.
type StartActionResponse struct {
	union json.RawMessage
}

// StopActionResponse defines model for StopActionResponse.
type StopActionResponse struct {
	union json.RawMessage
}

// ActionStatusRequestBody defines model for ActionStatusRequestBody.
type ActionStatusRequestBody struct {
	// Any kind of action specific state that will be passed to the next endpoints.
	State ActionState `json:"state"`
}

// PrepareActionRequestBody defines model for PrepareActionRequestBody.
type PrepareActionRequestBody struct {
	// The action configuration. This contains the end-user configuration done for the action. Possible configuration parameters are defined through the action description.
	Config      map[string]interface{} `json:"config"`
	ExecutionId uuid.UUID              `json:"executionId"`

	// The target on which to act on as identified by a discovery.
	Target *Target `json:"target,omitempty"`
}

// QueryMetricsRequestBody defines model for QueryMetricsRequestBody.
type QueryMetricsRequestBody struct {
	// The metric query configuration. This contains the end-user configuration done for the action. Possible configuration parameters are defined through the action description.
	Config      map[string]interface{} `json:"config"`
	ExecutionId uuid.UUID              `json:"executionId"`

	// The target on which to act on as identified by a discovery.
	Target *Target `json:"target,omitempty"`

	// For what timestamp the metric values should be retrieved.
	Timestamp time.Time `json:"timestamp"`
}

// StartActionRequestBody defines model for StartActionRequestBody.
type StartActionRequestBody struct {
	// Any kind of action specific state that will be passed to the next endpoints.
	State ActionState `json:"state"`
}

// StopActionRequestBody defines model for StopActionRequestBody.
type StopActionRequestBody struct {
	// Any kind of action specific state that will be passed to the next endpoints.
	State ActionState `json:"state"`
}

func (t ActionListResponse) AsActionList() (ActionList, error) {
	var body ActionList
	err := json.Unmarshal(t.union, &body)
	return body, err
}

func (t *ActionListResponse) FromActionList(v ActionList) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

func (t ActionListResponse) AsActionKitError() (ActionKitError, error) {
	var body ActionKitError
	err := json.Unmarshal(t.union, &body)
	return body, err
}

func (t *ActionListResponse) FromActionKitError(v ActionKitError) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

func (t ActionListResponse) MarshalJSON() ([]byte, error) {
	b, err := t.union.MarshalJSON()
	return b, err
}

func (t *ActionListResponse) UnmarshalJSON(b []byte) error {
	err := t.union.UnmarshalJSON(b)
	return err
}

func (t ActionStatusResponse) AsStatusResult() (StatusResult, error) {
	var body StatusResult
	err := json.Unmarshal(t.union, &body)
	return body, err
}

func (t *ActionStatusResponse) FromStatusResult(v StatusResult) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

func (t ActionStatusResponse) AsActionKitError() (ActionKitError, error) {
	var body ActionKitError
	err := json.Unmarshal(t.union, &body)
	return body, err
}

func (t *ActionStatusResponse) FromActionKitError(v ActionKitError) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

func (t ActionStatusResponse) MarshalJSON() ([]byte, error) {
	b, err := t.union.MarshalJSON()
	return b, err
}

func (t *ActionStatusResponse) UnmarshalJSON(b []byte) error {
	err := t.union.UnmarshalJSON(b)
	return err
}

func (t DescribeActionResponse) AsActionDescription() (ActionDescription, error) {
	var body ActionDescription
	err := json.Unmarshal(t.union, &body)
	return body, err
}

func (t *DescribeActionResponse) FromActionDescription(v ActionDescription) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

func (t DescribeActionResponse) AsActionKitError() (ActionKitError, error) {
	var body ActionKitError
	err := json.Unmarshal(t.union, &body)
	return body, err
}

func (t *DescribeActionResponse) FromActionKitError(v ActionKitError) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

func (t DescribeActionResponse) MarshalJSON() ([]byte, error) {
	b, err := t.union.MarshalJSON()
	return b, err
}

func (t *DescribeActionResponse) UnmarshalJSON(b []byte) error {
	err := t.union.UnmarshalJSON(b)
	return err
}

func (t PrepareActionResponse) AsPrepareResult() (PrepareResult, error) {
	var body PrepareResult
	err := json.Unmarshal(t.union, &body)
	return body, err
}

func (t *PrepareActionResponse) FromPrepareResult(v PrepareResult) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

func (t PrepareActionResponse) AsActionKitError() (ActionKitError, error) {
	var body ActionKitError
	err := json.Unmarshal(t.union, &body)
	return body, err
}

func (t *PrepareActionResponse) FromActionKitError(v ActionKitError) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

func (t PrepareActionResponse) MarshalJSON() ([]byte, error) {
	b, err := t.union.MarshalJSON()
	return b, err
}

func (t *PrepareActionResponse) UnmarshalJSON(b []byte) error {
	err := t.union.UnmarshalJSON(b)
	return err
}

func (t QueryMetricsResponse) AsQueryMetricsResult() (QueryMetricsResult, error) {
	var body QueryMetricsResult
	err := json.Unmarshal(t.union, &body)
	return body, err
}

func (t *QueryMetricsResponse) FromQueryMetricsResult(v QueryMetricsResult) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

func (t QueryMetricsResponse) AsActionKitError() (ActionKitError, error) {
	var body ActionKitError
	err := json.Unmarshal(t.union, &body)
	return body, err
}

func (t *QueryMetricsResponse) FromActionKitError(v ActionKitError) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

func (t QueryMetricsResponse) MarshalJSON() ([]byte, error) {
	b, err := t.union.MarshalJSON()
	return b, err
}

func (t *QueryMetricsResponse) UnmarshalJSON(b []byte) error {
	err := t.union.UnmarshalJSON(b)
	return err
}

func (t StartActionResponse) AsStartResult() (StartResult, error) {
	var body StartResult
	err := json.Unmarshal(t.union, &body)
	return body, err
}

func (t *StartActionResponse) FromStartResult(v StartResult) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

func (t StartActionResponse) AsActionKitError() (ActionKitError, error) {
	var body ActionKitError
	err := json.Unmarshal(t.union, &body)
	return body, err
}

func (t *StartActionResponse) FromActionKitError(v ActionKitError) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

func (t StartActionResponse) MarshalJSON() ([]byte, error) {
	b, err := t.union.MarshalJSON()
	return b, err
}

func (t *StartActionResponse) UnmarshalJSON(b []byte) error {
	err := t.union.UnmarshalJSON(b)
	return err
}

func (t StopActionResponse) AsStopResult() (StopResult, error) {
	var body StopResult
	err := json.Unmarshal(t.union, &body)
	return body, err
}

func (t *StopActionResponse) FromStopResult(v StopResult) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

func (t StopActionResponse) AsActionKitError() (ActionKitError, error) {
	var body ActionKitError
	err := json.Unmarshal(t.union, &body)
	return body, err
}

func (t *StopActionResponse) FromActionKitError(v ActionKitError) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

func (t StopActionResponse) MarshalJSON() ([]byte, error) {
	b, err := t.union.MarshalJSON()
	return b, err
}

func (t *StopActionResponse) UnmarshalJSON(b []byte) error {
	err := t.union.UnmarshalJSON(b)
	return err
}

type ParameterOption interface {}
type Widget interface {}