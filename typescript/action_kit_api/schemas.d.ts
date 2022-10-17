/**
 * This file was auto-generated by openapi-typescript.
 * Do not make direct changes to the file.
 */

export interface paths {}

export interface components {
	schemas: {
		/**
		 * Action List
		 * @description Lists all actions that the platform/agent could execute.
		 */
		ActionList: {
			actions: components['schemas']['DescribingEndpointReference'][];
		};
		/**
		 * Error
		 * @description RFC 7807 Problem Details for HTTP APIs compliant response body for error scenarios
		 */
		ActionKitError: {
			/** @description A URI reference that identifies the problem type. */
			type?: string;
			/** @description A short, human-readable summary of the problem type. */
			title: string;
			/** @description A human-readable explanation specific to this occurrence of the problem. */
			detail?: string;
			/** @description A URI reference that identifies the specific occurrence of the problem. */
			instance?: string;
		};
		/**
		 * Messages
		 * @description Log-messages that will be passed to the agent log.
		 */
		Messages: components['schemas']['Message'][];
		/**
		 * Message
		 * @description Log-message that will be passed to the agent log.
		 */
		Message: {
			message: string;
			/**
			 * @default info
			 * @enum {string}
			 */
			level?: 'debug' | 'info' | 'warn' | 'error';
		};
		/** Artifacts */
		Artifacts: components['schemas']['Artifact'][];
		/**
		 * Artifact
		 * @description Actions may choose to provide artifacts (arbitrary files) that are later accessible by users when inspecting experiment execution details. This comes in handy to expose load test reports and similar data.
		 */
		Artifact: {
			/** @description Human-readable label for the artifact. We recommend to include file extensions within the label for a better user-experience when downloading these artifacts, e.g., load_test_result.tar.gz. */
			label: string;
			/** @description base64 encoded data */
			data: string;
		};
		/**
		 * Metric
		 * @description Metrics can be exposed by actions. These metrics can then be leveraged by end-users to inspect system behavior and to optionally abort experiment execution when certain metrics are observed, i.e., metrics can act as (steady state) checks.
		 */
		Metric: {
			/**
			 * Format: date-time
			 * @description Timestamp describing at which moment the value was observed.
			 */
			timestamp: string;
			/** @description Metric name. You can alternatively encode the metric name as `__name__` within the metric property. */
			name?: string;
			/** @description Key/value pairs describing the metric. This type is modeled after Prometheus' data model, i.e., metric labels. You may encode the metric name as `__name__` similar to how Prometheus does it. */
			metric: { [key: string]: string };
			value: number;
		};
		/** Metrics */
		Metrics: components['schemas']['Metric'][];
		/** Metrics Configuration */
		MetricsConfiguration: {
			query?: components['schemas']['MetricsQueryConfiguration'];
		};
		/** Metrics Query Configuration */
		MetricsQueryConfiguration: {
			endpoint: components['schemas']['MutatingEndpointReferenceWithCallInterval'];
			parameters: components['schemas']['ActionParameter'][];
		};
		/**
		 * Action Description
		 * @description Provides details about a possible action, e.g., what configuration options it has, how to present it to end-users and how to trigger the action.
		 */
		ActionDescription: {
			/** @description A technical ID that is used to uniquely identify this type of action. You will typically want to use something like `org.example.my-fancy-attack`. */
			id: string;
			/** @description A human-readable label for the action. */
			label: string;
			/** @description An icon that is used to identify your action in the ui. Needs to be a data-uri containing an image. */
			icon?: string;
			/** @description The version of the action. Remember to increase the value everytime you update the definitions. The platform will ignore any definition changes with the same action version. We do recommend usage of semver strings. */
			version: string;
			/** @description Description for end-users to help them understand what the action is doing. */
			description: string;
			/**
			 * @description Describes what kind of action this is. This is used to help users understand and classify the various runnable actions that are part of an experiment.
			 * @enum {string}
			 */
			kind: 'attack' | 'check' | 'load_test' | 'other';
			/** @description Used for categorization of the action within user interfaces. */
			category?: string;
			/** @description What target type this action should be offered for. Matches the `id` field within `DescribeTargetTypeResponse` within DiscoveryKit. */
			targetType?: string;
			/**
			 * @description Actions can either be an instantaneous event, e.g., the restart of a host, or an activity spanning over an unspecified duration. For those actions having a duration, we differentiate between internally, e.g., waiting for a deployment to finish, and externally, e.g., waiting for a user-specified time to pass, controlled durations.
			 * @enum {string}
			 */
			timeControl: 'instantaneous' | 'internal' | 'external';
			parameters: components['schemas']['ActionParameter'][];
			metrics?: components['schemas']['MetricsConfiguration'];
			prepare: components['schemas']['MutatingEndpointReference'];
			start: components['schemas']['MutatingEndpointReference'];
			status?: components['schemas']['MutatingEndpointReferenceWithCallInterval'];
			stop?: components['schemas']['MutatingEndpointReference'];
		};
		ParameterOption: Partial<components['schemas']['ExplicitParameterOption']> &
			Partial<components['schemas']['ParameterOptionsFromTargetAttribute']>;
		/** @description You can use an explicit/fixed parameter option for a known / finite set of options that never change. */
		ExplicitParameterOption: {
			/** @description A human-readable label describing this option. */
			label: string;
			/** @description The technical value which will be passed to the action as part of the `config` object. */
			value: string;
		};
		/** @description A meta option that represents all target attribute values for the key defined through the attribute field. */
		ParameterOptionsFromTargetAttribute: {
			/** @description Target attribute key from which the possible parameter options are gathered. */
			attribute: string;
		};
		ActionParameter: {
			/** @description A human-readable label for the action parameter. */
			label: string;
			/** @description The key under which the action parameter is stored. This key can then be found within the prepare request's config field. */
			name: string;
			/** @description Description for end-users to help them understand the action parameter. */
			description?: string;
			/**
			 * @description What kind of value this parameter is capturing. The type selection influences the `config` passed as part of the `PrepareRequest`. It also results in improved user-interface elements.
			 * @enum {string}
			 */
			type:
				| 'string'
				| 'string[]'
				| 'string_array'
				| 'password'
				| 'integer'
				| 'boolean'
				| 'percentage'
				| 'duration'
				| 'file'
				| 'key_value';
			/** @description Whether or not end-users need to specify a value for this parameter. */
			required?: boolean;
			/** @description Whether this parameter should be placed under the expandable advanced section within the user interface. */
			advanced?: boolean;
			/** @description You can define this fields to order the parameters in the user interface. The lower the value, the higher the position. */
			order?: number;
			/** @description A default value for this parameter. This value will be used if the user does not specify a value for this parameter. */
			defaultValue?: string;
			/** @description Optional options for the `string`, `string[]` and `string_array` parameter types. Which result in suggestions for end-users. */
			options?: components['schemas']['ParameterOption'][];
			/** @description Unique file type specifiers describing what type of files are accepted for parameters of type 'file'. */
			acceptedFileTypes?: string[];
		};
		/** @description The target on which to act on as identified by a discovery. */
		Target: {
			name: string;
			/** @description These attributes include detailed information about the target provided through the discovery. These attributes are typically used as additional parameters within the action implementation. */
			attributes: { [key: string]: string[] };
		};
		/**
		 * Action State
		 * @description Any kind of action specific state that will be passed to the next endpoints.
		 */
		ActionState: { [key: string]: unknown };
		/**
		 * Action Result
		 * @description The result of the action.
		 */
		Result: string;
		PrepareResult: {
			state: components['schemas']['ActionState'];
			result?: components['schemas']['Result'];
			messages?: components['schemas']['Messages'];
			artifacts?: components['schemas']['Artifacts'];
			metrics?: components['schemas']['Metrics'];
		};
		StartResult: {
			state?: components['schemas']['ActionState'];
			result?: components['schemas']['Result'];
			messages?: components['schemas']['Messages'];
			artifacts?: components['schemas']['Artifacts'];
			metrics?: components['schemas']['Metrics'];
		};
		StatusResult: {
			/** @description the agent will continue to poll the status endpoint as long as completed is false */
			completed: boolean;
			state?: components['schemas']['ActionState'];
			result?: components['schemas']['Result'];
			messages?: components['schemas']['Messages'];
			artifacts?: components['schemas']['Artifacts'];
			metrics?: components['schemas']['Metrics'];
		};
		StopResult: {
			result?: components['schemas']['Result'];
			messages?: components['schemas']['Messages'];
			artifacts?: components['schemas']['Artifacts'];
			metrics?: components['schemas']['Metrics'];
		};
		QueryMetricsResult: {
			messages?: components['schemas']['Messages'];
			artifacts?: components['schemas']['Artifacts'];
			metrics?: components['schemas']['Metrics'];
		};
		/**
		 * HTTP Endpoint Reference
		 * @description HTTP endpoint which the Steadybit platform/agent could communicate with.
		 */
		DescribingEndpointReference: {
			/** @description Absolute path of the HTTP endpoint. */
			path: string;
			/**
			 * @description HTTP method to use when calling the HTTP endpoint.
			 * @enum {string}
			 */
			method: 'get';
		};
		/** @enum {string} */
		MutatingHttpMethod: 'post' | 'put' | 'delete';
		/**
		 * HTTP Endpoint Reference
		 * @description HTTP endpoint which the Steadybit platform/agent could communicate with.
		 */
		MutatingEndpointReference: {
			/** @description Absolute path of the HTTP endpoint. */
			path: string;
			/** @description HTTP method to use when calling the HTTP endpoint. */
			method: components['schemas']['MutatingHttpMethod'];
		};
		/**
		 * HTTP Endpoint Reference
		 * @description HTTP endpoint which the Steadybit platform/agent could communicate with.
		 */
		MutatingEndpointReferenceWithCallInterval: components['schemas']['MutatingEndpointReference'] & {
			/** @description At what frequency should the state endpoint be called? Takes durations in the format of `100ms` or `10s`. */
			callInterval?: string;
		};
	};
	responses: {
		/** Response for the action list endpoint */
		ActionListResponse: {
			content: {
				'application/json': Partial<components['schemas']['ActionList']> &
					Partial<components['schemas']['ActionKitError']>;
			};
		};
		/** Response for the describe action endpoint */
		DescribeActionResponse: {
			content: {
				'application/json': Partial<components['schemas']['ActionDescription']> &
					Partial<components['schemas']['ActionKitError']>;
			};
		};
		/** Response for the action preparation endpoint */
		PrepareActionResponse: {
			content: {
				'application/json': Partial<components['schemas']['PrepareResult']> &
					Partial<components['schemas']['ActionKitError']>;
			};
		};
		/** Response for the start action endpoint */
		StartActionResponse: {
			content: {
				'application/json': Partial<components['schemas']['StartResult']> &
					Partial<components['schemas']['ActionKitError']>;
			};
		};
		/** Response for the start action endpoint */
		ActionStatusResponse: {
			content: {
				'application/json': Partial<components['schemas']['StatusResult']> &
					Partial<components['schemas']['ActionKitError']>;
			};
		};
		/** Response for the stop action endpoint */
		StopActionResponse: {
			content: {
				'application/json': Partial<components['schemas']['StopResult']> &
					Partial<components['schemas']['ActionKitError']>;
			};
		};
		/** Response for the metric query endpoint */
		QueryMetricsResponse: {
			content: {
				'application/json': Partial<components['schemas']['QueryMetricsResult']> &
					Partial<components['schemas']['ActionKitError']>;
			};
		};
	};
	requestBodies: {
		/** The HTTP request payload passed to the metric query endpoints. Multiple query executions happen for every action execution. */
		QueryMetricsRequestBody: {
			content: {
				'application/json': {
					/** Format: string */
					executionId: string;
					/**
					 * Format: date-time
					 * @description For what timestamp the metric values should be retrieved.
					 */
					timestamp: string;
					/** @description The metric query configuration. This contains the end-user configuration done for the action. Possible configuration parameters are defined through the action description. */
					config: { [key: string]: unknown };
					target?: components['schemas']['Target'];
				};
			};
		};
		/** The HTTP request payload passed to the action prepare endpoints. */
		PrepareActionRequestBody: {
			content: {
				'application/json': {
					/** Format: string */
					executionId: string;
					/** @description The action configuration. This contains the end-user configuration done for the action. Possible configuration parameters are defined through the action description. */
					config: { [key: string]: unknown };
					target?: components['schemas']['Target'];
				};
			};
		};
		/** The HTTP request payload passed to the start action endpoints. */
		StartActionRequestBody: {
			content: {
				'application/json': {
					state: components['schemas']['ActionState'];
				};
			};
		};
		/** The HTTP request payload passed to the action status endpoints. */
		ActionStatusRequestBody: {
			content: {
				'application/json': {
					state: components['schemas']['ActionState'];
				};
			};
		};
		/** The HTTP request payload passed to the stop action endpoints. */
		StopActionRequestBody: {
			content: {
				'application/json': {
					state: components['schemas']['ActionState'];
				};
			};
		};
	};
}

export interface operations {}

export interface external {}
