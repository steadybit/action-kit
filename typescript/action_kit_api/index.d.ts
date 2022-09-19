// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { components } from './schemas';

export type ActionList = components['schemas']['ActionList'];
export type ActionKitError = components['schemas']['ActionKitError'];
export type Messages = components['schemas']['Messages'];
export type Message = components['schemas']['Message'];
export type Artifacts = components['schemas']['Artifacts'];
export type Artifact = components['schemas']['Artifact'];
export type Metrics = components['schemas']['Metrics'];
export type Metric = components['schemas']['Metric'];
export type ActionDescription = components['schemas']['ActionDescription'];
export type ParameterOption = components['schemas']['ParameterOption'];
export type ExplicitParameterOption = components['schemas']['ExplicitParameterOption'];
export type ParameterOptionsFromTargetAttribute = components['schemas']['ParameterOptionsFromTargetAttribute'];
export type ActionParameter = components['schemas']['ActionParameter'];
export type Target = components['schemas']['Target'];
export type ActionState = components['schemas']['ActionState'];
export type PrepareResult = components['schemas']['PrepareResult'];
export type StartResult = components['schemas']['StartResult'];
export type StatusResult = components['schemas']['StatusResult'];
export type StopResult = components['schemas']['StopResult'];
export type QueryMetricsResult = components['schemas']['QueryMetricsResult'];
export type DescribingEndpointReference = components['schemas']['DescribingEndpointReference'];
export type MutatingHttpMethod = components['schemas']['MutatingHttpMethod'];
export type MutatingEndpointReference = components['schemas']['MutatingEndpointReference'];
export type MutatingEndpointReferenceWithCallInterval =
	components['schemas']['MutatingEndpointReferenceWithCallInterval'];

export type ActionListResponse = components['responses']['ActionListResponse']['content']['application/json'];
export type DescribeActionResponse = components['responses']['DescribeActionResponse']['content']['application/json'];
export type PrepareActionResponse = components['responses']['PrepareActionResponse']['content']['application/json'];
export type StartActionResponse = components['responses']['StartActionResponse']['content']['application/json'];
export type ActionStatusResponse = components['responses']['ActionStatusResponse']['content']['application/json'];
export type StopActionResponse = components['responses']['StopActionResponse']['content']['application/json'];
export type QueryMetricsResponse = components['responses']['QueryMetricsResponse']['content']['application/json'];

export type PrepareActionRequestBody =
	components['requestBodies']['PrepareActionRequestBody']['content']['application/json'];
export type StartActionRequestBody =
	components['requestBodies']['StartActionRequestBody']['content']['application/json'];
export type ActionStatusRequestBody =
	components['requestBodies']['ActionStatusRequestBody']['content']['application/json'];
export type StopActionRequestBody = components['requestBodies']['StopActionRequestBody']['content']['application/json'];
export type QueryMetricsRequestBody =
	components['requestBodies']['QueryMetricsRequestBody']['content']['application/json'];
