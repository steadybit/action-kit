// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

export type Method = 'GET' | 'POST' | 'PUT' | 'DELETE';

export interface HttpEndpointRef<ALLOWED_METHODS extends Method> {
	method?: ALLOWED_METHODS;
	path: string;
}

export interface HttpEndpointRefWithCallInternval<ALLOWED_METHODS extends Method>
	extends HttpEndpointRef<ALLOWED_METHODS> {
	/**
	 * The duration, e.g., `100ms` or `1s`.
	 */
	callInterval?: string;
}

export type IndexResponse = SuccessfulIndexResponse | Problem;

export interface SuccessfulIndexResponse {
	attacks: HttpEndpointRef<'GET'>[];
}

export type DescribeAttackResponse = SuccessfulDescribeAttackResponse | Problem;

export interface SuccessfulDescribeAttackResponse {
	id: string;
	label: string;
	icon: string;
	description: string;
	category?: 'resource' | 'network' | 'state';
	version: string;
	targetType: string;
	timeControl: 'INSTANTANEOUS' | 'INTERNAL' | 'EXTERNAL';
	parameters?: Array<BooleanParameter | IntegerParameter | StringParameter | MultiOptionParameter>;
	prepare: HttpEndpointRef<'POST' | 'PUT' | 'DELETE'>;
	start: HttpEndpointRef<'POST' | 'PUT' | 'DELETE'>;
	status: HttpEndpointRefWithCallInternval<'POST' | 'PUT' | 'DELETE'>;
	stop: HttpEndpointRef<'POST' | 'PUT' | 'DELETE'>;
}

export interface AttackParameter {
	label: string;
	name: string;
	description?: string;
	type: 'string' | 'string[]' | 'password' | 'integer' | 'boolean' | 'percentage' | 'duration';
	required?: boolean;
	// whether this parameter should be placed under the expandable advanced section within the user interface
	advanced?: boolean;
	order?: number;
	defaultValue?: string;
}

export interface BooleanParameter extends AttackParameter {
	type: 'boolean';
}

export interface IntegerParameter extends AttackParameter {
	type: 'integer' | 'percentage';
	min?: number;
	max?: number;
}

export interface StringParameter extends AttackParameter {
	type: 'string';
	options?: { label: string; value: string }[];
}

export interface MultiOptionParameter extends AttackParameter {
	type: 'string[]';
	options: { label: string; value: string }[];
}

export interface PrepareRequest {
	config: any;
	target: {
		name: string;
		attributes: Record<string, string[]>;
	};
}

export type PrepareResponse = SuccessfulPrepareResponse | Problem;

export interface SuccessfulPrepareResponse {
	state: any;
	messages?: Message[];
}

export interface StartRequest {
	state: any;
}

export type StartResponse = SuccessfulStartResponse | Problem;

export interface SuccessfulStartResponse {
	state: any;
	messages?: Message[];
}

export interface StatusRequest {
	state: any;
}

export type StatusResponse = SuccessfulStatusResponse | Problem;

export interface SuccessfulStatusResponse {
	completed: boolean;
	state?: any;
	messages?: Message[];
}

export interface StopRequest {
	state: any;
	canceled: boolean;
}

export type StopResponse = SuccessfulStopResponse | Problem;

export interface SuccessfulStopResponse {
	messages?: Message[];
}

export interface Problem {
	type?: string;
	title: string;
	detail?: string;
	instance?: string;
}

export interface Message {
	message: string;
	level?: 'debug' | 'info' | 'error';
}
