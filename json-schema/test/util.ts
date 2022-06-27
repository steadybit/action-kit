// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import describingEndpointRef from '../schema/describingEndpointRef.json';
import attackListResponse from '../schema/attackListResponse.json';
import mutatingEndpointRef from '../schema/mutatingEndpointRef.json';
import Ajv, { ErrorObject } from 'ajv';

const ajv = new Ajv({ schemas: [describingEndpointRef, attackListResponse, mutatingEndpointRef], allErrors: true });

export interface ValidationResult {
	valid: boolean;
	errors: ErrorObject[];
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function validate(schema: any, test: any): ValidationResult {
	const validate = ajv.compile(schema);
	const valid = validate(test);
	return {
		valid,
		errors: validate.errors ?? [],
	};
}
