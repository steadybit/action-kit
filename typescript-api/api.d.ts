// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

export type Method = 'GET' | 'POST' | 'PUT' | 'DELETE';

export interface BasicAuth {
  username: string;
  password: string;
}

export interface HttpEndpointRef<ALLOWED_METHODS> {
  method?: ALLOWED_METHODS;
  url: string;
  basicAuth?: BasicAuth;
}

export interface IndexResponse {
  attacks: HttpEndpointRef<'GET'>[]
}

export interface AttackInfoResponse {
  id: string;
  name: string;
  description: string;
  category: string;
  version: string;
  target: string;

  prepare: HttpEndpointRef<'POST' | 'PUT' | 'DELETE'>;
  start: HttpEndpointRef<'POST' | 'PUT' | 'DELETE'>;
  stop: HttpEndpointRef<'POST' | 'PUT' | 'DELETE'>;
}

export interface StateResponse {
  state: any;
}

export interface PrepareRequest {
  config: any;
  target: {
    name: string;
    attributes: Record<string, string[]>;
  }
}

export interface StartRequest {
  state: any;
}

export interface StopRequest {
  state: any;
  canceled: boolean;
}