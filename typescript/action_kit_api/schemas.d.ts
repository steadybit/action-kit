/**
 * This file was auto-generated by openapi-typescript.
 * Do not make direct changes to the file.
 */


export type paths = Record<string, never>;

export type webhooks = Record<string, never>;

export interface components {
  schemas: {
    /**
     * Action List 
     * @description Lists all actions that the platform/agent could execute.
     */
    ActionList: {
      actions: (components["schemas"]["DescribingEndpointReference"])[];
    };
    /**
     * Error 
     * @description An enhanced version of RFC 7807 Problem Details for HTTP APIs compliant response body for error scenarios
     */
    ActionKitError: {
      /**
       * @description * failed - The action has detected some failures, for example a failing test which has been implemented by the action. The action will be stopped, if this status is returned by the status endpoint. * errored - There was a technical error while executing the action. Will be marked as red in the platform. The action will be stopped, if this status is returned by the status endpoint. 
       * @default errored 
       * @enum {string}
       */
      status?: "failed" | "errored";
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
     * @description Log-messages that will be passed to the platform (default agent log).
     */
    Messages: (components["schemas"]["Message"])[];
    /**
     * Message Fields 
     * @description Any kind of action specific fields that will be rendered in the platform tooltip of LogWidget
     */
    MessageFields: {
      [key: string]: string | undefined;
    };
    /**
     * Message 
     * @description Log-message that will be passed to the platform (default agent log).
     */
    Message: {
      message: string;
      /** @default AGENT */
      type?: string;
      /**
       * @default info 
       * @enum {string}
       */
      level?: "debug" | "info" | "warn" | "error";
      /** Format: date-time */
      timestamp?: string;
      fields?: components["schemas"]["MessageFields"];
    };
    /** TargetSelectionTemplates */
    TargetSelectionTemplates: (components["schemas"]["TargetSelectionTemplate"])[];
    /**
     * TargetSelectionTemplate 
     * @description Users that want to configure an action with a targetType need to define a target selection through the query UI or query language. Extensions can define selection templates to help users define such target selections.
     */
    TargetSelectionTemplate: {
      /** @description Human-readable short label. */
      label: string;
      /** @description Longer target selection template description. For example, to explain the template's purpose. */
      description?: string;
      /**
       * @description The target selection query is defined using Steadybit's query language. For example:
       *   aws.account="" AND aws.zone.id=""
       * For more information about the query language, please inspect Steadybit's documentation:
       *   https://docs.steadybit.com/use-steadybit/query-language
       */
      query: string;
    };
    /** Widgets */
    Widgets: (components["schemas"]["Widget"])[];
    /** Widget */
    Widget: components["schemas"]["StateOverTimeWidget"] | components["schemas"]["LogWidget"] | components["schemas"]["PredefinedWidget"];
    /** LogWidget */
    LogWidget: {
      /** @enum {string} */
      type: "com.steadybit.widget.log";
      title: string;
      logType: string;
    };
    /** PredefinedWidget */
    PredefinedWidget: {
      /** @enum {string} */
      type: "com.steadybit.widget.predefined";
      predefinedWidgetId: string;
    };
    /** StateOverTimeWidget */
    StateOverTimeWidget: {
      /** @enum {string} */
      type: "com.steadybit.widget.state_over_time";
      title: string;
      identity: components["schemas"]["StateOverTimeWidgetIdentityConfig"];
      label: components["schemas"]["StateOverTimeWidgetLabelConfig"];
      tooltip: components["schemas"]["StateOverTimeWidgetTooltipConfig"];
      url?: components["schemas"]["StateOverTimeWidgetUrlConfig"];
      state: components["schemas"]["StateOverTimeWidgetStateConfig"];
      value?: components["schemas"]["StateOverTimeWidgetValueConfig"];
    };
    /** StateOverTimeWidgetIdentityConfig */
    StateOverTimeWidgetIdentityConfig: {
      from: string;
    };
    /** StateOverTimeWidgetLabelConfig */
    StateOverTimeWidgetLabelConfig: {
      from: string;
    };
    /** StateOverTimeWidgetTooltipConfig */
    StateOverTimeWidgetTooltipConfig: {
      from: string;
    };
    /** StateOverTimeWidgetUrlConfig */
    StateOverTimeWidgetUrlConfig: {
      from?: string;
    };
    /** StateOverTimeWidgetStateConfig */
    StateOverTimeWidgetStateConfig: {
      from: string;
    };
    /** StateOverTimeWidgetValueConfig */
    StateOverTimeWidgetValueConfig: {
      /** @description Set to true to hide the metric value within the widget. This is useful when only the translated state information is relevant. */
      hide?: boolean;
    };
    /** Artifacts */
    Artifacts: (components["schemas"]["Artifact"])[];
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
      metric: {
        [key: string]: string | undefined;
      };
      value: number;
    };
    /** Metrics */
    Metrics: (components["schemas"]["Metric"])[];
    /** Metrics Configuration */
    MetricsConfiguration: {
      query?: components["schemas"]["MetricsQueryConfiguration"];
    };
    /** Metrics Query Configuration */
    MetricsQueryConfiguration: {
      endpoint: components["schemas"]["MutatingEndpointReferenceWithCallInterval"];
      parameters: (components["schemas"]["ActionParameter"])[];
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
      kind: "attack" | "check" | "load_test" | "other";
      /** @description Used for categorization of the action within user interfaces. */
      category?: string;
      /** @description What target type this action should be offered for. Matches the `id` field within `DescribeTargetTypeResponse` within DiscoveryKit. */
      targetType?: string;
      targetSelectionTemplates?: components["schemas"]["TargetSelectionTemplates"];
      /**
       * @description Actions can either be an instantaneous event, e.g., the restart of a host, or an activity spanning over an unspecified duration. For those actions having a duration, we differentiate between internally, e.g., waiting for a deployment to finish, and externally, e.g., waiting for a user-specified time to pass, controlled durations. 
       * @enum {string}
       */
      timeControl: "instantaneous" | "internal" | "external";
      parameters: (components["schemas"]["ActionParameter"])[];
      hint?: components["schemas"]["ActionHint"];
      widgets?: components["schemas"]["Widgets"];
      metrics?: components["schemas"]["MetricsConfiguration"];
      prepare: components["schemas"]["MutatingEndpointReference"];
      start: components["schemas"]["MutatingEndpointReference"];
      status?: components["schemas"]["MutatingEndpointReferenceWithCallInterval"];
      stop?: components["schemas"]["MutatingEndpointReference"];
    };
    ParameterOption: components["schemas"]["ExplicitParameterOption"] | components["schemas"]["ParameterOptionsFromTargetAttribute"];
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
    ActionHint: {
      /**
       * @description Will be used in the product UI to display the hint in a different color and with a different icon. 
       * @enum {string}
       */
      type: "hint_info" | "hint_warning";
      /** @description The actual hint text (can contain markdown). Will be displayed in the product UI when configuring the action. */
      content: string;
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
      type: "string" | "string[]" | "string_array" | "password" | "integer" | "boolean" | "percentage" | "duration" | "file" | "key_value";
      /** @description Whether or not end-users need to specify a value for this parameter. */
      required?: boolean;
      /** @description Whether this parameter should be placed under the expandable advanced section within the user interface. */
      advanced?: boolean;
      /** @description You can define this fields to order the parameters in the user interface. The lower the value, the higher the position. */
      order?: number;
      /** @description A default value for this parameter. This value will be used if the user does not specify a value for this parameter. */
      defaultValue?: string;
      /** @description Optional options for the `string`, `string[]` and `string_array` parameter types. Which result in suggestions for end-users. */
      options?: (components["schemas"]["ParameterOption"])[];
      /** @description Unique file type specifiers describing what type of files are accepted for parameters of type 'file'. */
      acceptedFileTypes?: (string)[];
      hint?: components["schemas"]["ActionHint"];
    };
    /** @description The target on which to act on as identified by a discovery. */
    Target: {
      name: string;
      /** @description These attributes include detailed information about the target provided through the discovery. These attributes are typically used as additional parameters within the action implementation. */
      attributes: {
        [key: string]: (string)[] | undefined;
      };
    };
    /**
     * Action State 
     * @description Any kind of action specific state that will be passed to the next endpoints.
     */
    ActionState: {
      [key: string]: unknown | undefined;
    };
    PrepareResult: {
      state: components["schemas"]["ActionState"];
      error?: components["schemas"]["ActionKitError"];
      messages?: components["schemas"]["Messages"];
      artifacts?: components["schemas"]["Artifacts"];
      metrics?: components["schemas"]["Metrics"];
    };
    StartResult: {
      state?: components["schemas"]["ActionState"];
      error?: components["schemas"]["ActionKitError"];
      messages?: components["schemas"]["Messages"];
      artifacts?: components["schemas"]["Artifacts"];
      metrics?: components["schemas"]["Metrics"];
    };
    StatusResult: {
      /** @description the agent will continue to poll the status endpoint as long as completed is false */
      completed: boolean;
      state?: components["schemas"]["ActionState"];
      error?: components["schemas"]["ActionKitError"];
      messages?: components["schemas"]["Messages"];
      artifacts?: components["schemas"]["Artifacts"];
      metrics?: components["schemas"]["Metrics"];
    };
    StopResult: {
      error?: components["schemas"]["ActionKitError"];
      messages?: components["schemas"]["Messages"];
      artifacts?: components["schemas"]["Artifacts"];
      metrics?: components["schemas"]["Metrics"];
    };
    QueryMetricsResult: {
      messages?: components["schemas"]["Messages"];
      artifacts?: components["schemas"]["Artifacts"];
      metrics?: components["schemas"]["Metrics"];
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
      method: "get";
    };
    /** @enum {string} */
    MutatingHttpMethod: "post" | "put" | "delete";
    /**
     * HTTP Endpoint Reference 
     * @description HTTP endpoint which the Steadybit platform/agent could communicate with.
     */
    MutatingEndpointReference: {
      /** @description Absolute path of the HTTP endpoint. */
      path: string;
      /** @description HTTP method to use when calling the HTTP endpoint. */
      method: components["schemas"]["MutatingHttpMethod"];
    };
    /**
     * HTTP Endpoint Reference 
     * @description HTTP endpoint which the Steadybit platform/agent could communicate with.
     */
    MutatingEndpointReferenceWithCallInterval: components["schemas"]["MutatingEndpointReference"] & {
      /** @description At what frequency should the state endpoint be called? Takes durations in the format of `100ms` or `10s`. */
      callInterval?: string;
    };
  };
  responses: {
    /**
     * Action List Response 
     * @description Response for the action list endpoint
     */
    ActionListResponse: {
      content: {
        "application/json": components["schemas"]["ActionList"] | components["schemas"]["ActionKitError"];
      };
    };
    /**
     * Describe Action Response 
     * @description Response for the describe action endpoint
     */
    DescribeActionResponse: {
      content: {
        "application/json": components["schemas"]["ActionDescription"] | components["schemas"]["ActionKitError"];
      };
    };
    /**
     * Prepare Action Response 
     * @description Response for the action preparation endpoint
     */
    PrepareActionResponse: {
      content: {
        "application/json": components["schemas"]["PrepareResult"] | components["schemas"]["ActionKitError"];
      };
    };
    /**
     * Start Action Response 
     * @description Response for the start action endpoint
     */
    StartActionResponse: {
      content: {
        "application/json": components["schemas"]["StartResult"] | components["schemas"]["ActionKitError"];
      };
    };
    /**
     * Action Status Response 
     * @description Response for the start action endpoint
     */
    ActionStatusResponse: {
      content: {
        "application/json": components["schemas"]["StatusResult"] | components["schemas"]["ActionKitError"];
      };
    };
    /**
     * Stop Action Response 
     * @description Response for the stop action endpoint
     */
    StopActionResponse: {
      content: {
        "application/json": components["schemas"]["StopResult"] | components["schemas"]["ActionKitError"];
      };
    };
    /**
     * Query MetricsResponse 
     * @description Response for the metric query endpoint
     */
    QueryMetricsResponse: {
      content: {
        "application/json": components["schemas"]["QueryMetricsResult"] | components["schemas"]["ActionKitError"];
      };
    };
  };
  parameters: never;
  requestBodies: {
    /**
     * Query Metrics Request 
     * @description The HTTP request payload passed to the metric query endpoints. Multiple query executions happen for every action execution.
     */
    QueryMetricsRequestBody?: {
      content: {
        "application/json": {
          /** Format: string */
          executionId: string;
          /**
           * Format: date-time 
           * @description For what timestamp the metric values should be retrieved.
           */
          timestamp: string;
          /** @description The metric query configuration. This contains the end-user configuration done for the action. Possible configuration parameters are defined through the action description. */
          config: Record<string, never>;
          target?: components["schemas"]["Target"];
        };
      };
    };
    /**
     * Prepare Action Request 
     * @description The HTTP request payload passed to the action prepare endpoints.
     */
    PrepareActionRequestBody?: {
      content: {
        "application/json": {
          /** Format: string */
          executionId: string;
          /** @description The action configuration. This contains the end-user configuration done for the action. Possible configuration parameters are defined through the action description. */
          config: {
            [key: string]: unknown | undefined;
          };
          target?: components["schemas"]["Target"];
        };
      };
    };
    /**
     * Start Action Request 
     * @description The HTTP request payload passed to the start action endpoints.
     */
    StartActionRequestBody?: {
      content: {
        "application/json": {
          state: components["schemas"]["ActionState"];
        };
      };
    };
    /**
     * Action Status Request 
     * @description The HTTP request payload passed to the action status endpoints.
     */
    ActionStatusRequestBody?: {
      content: {
        "application/json": {
          state: components["schemas"]["ActionState"];
        };
      };
    };
    /**
     * Stop Action Request 
     * @description The HTTP request payload passed to the stop action endpoints.
     */
    StopActionRequestBody?: {
      content: {
        "application/json": {
          state: components["schemas"]["ActionState"];
        };
      };
    };
  };
  headers: never;
  pathItems: never;
}

export type external = Record<string, never>;

export type operations = Record<string, never>;
