// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

export type Method = 'GET' | 'POST' | 'PUT' | 'DELETE';

export interface HttpEndpointRef<ALLOWED_METHODS extends Method> {
    method?: ALLOWED_METHODS;
    path: string;
}

export interface IndexResponse {
    attacks: HttpEndpointRef<'GET'>[];
}

export interface DescribeAttackResponse {
    id: string;
    name: string;
    //TODO: icon for ui?
    description: string;
    category: 'resource' | 'network' | 'state';
    version: string;
    //TODO: support target-less attacks?
    target: 'container' | 'host' | 'kubernetes-deployment' | 'zone' | 'ec2-instance';
    parameters?: Array<AttackParameter>;
    prepare: HttpEndpointRef<'POST' | 'PUT' | 'DELETE'>;
    start: HttpEndpointRef<'POST' | 'PUT' | 'DELETE'>;
    stop: HttpEndpointRef<'POST' | 'PUT' | 'DELETE'>;
}

export interface AttackParameter {
    label: string;
    name: string;
    description?: string;
    //TODO: decouple UI element from type?
    type: 'string' | 'string[]' | 'integer' | 'boolean' | 'percentage' | 'duration';
    required?: boolean;
    advanced?: boolean;
    order?: number;
    defaultValue?: string;
    options?: { label: string; value: string; }[];
    min?: number;
    max?: number;
}

export interface BooleanParameter extends AttackParameter {
    type: 'boolean';
}

export interface IntegerParameter extends AttackParameter {
    type: 'integer' | 'percentage' | 'duration';
    min: number;
    max: number;
}

export interface StringParameter extends AttackParameter {
    type: 'string';
    options?: { label: string; value: string; }[];
}

export interface MultiOptionParameter extends AttackParameter {
    type: 'string[]';
    options: { label: string; value: string; }[];
}

export interface PrepareRequest {
    config: any;
    target: {
        name: string;
        attributes: Record<string, string[]>;
    };
}

export interface PrepareResponse {
    state: any;
}

export interface StartRequest {
    state: any;
}

export interface StartResponse {
    state: any;
}

export interface StopRequest {
    state: any;
    canceled: boolean;
}