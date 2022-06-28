// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import Ajv, { ErrorObject } from 'ajv';

import fs from 'fs';
import path from 'path';

const pathToSchemas = path.join(__dirname, '..', 'schema');

const schemas = fs.readdirSync(pathToSchemas).map((file) => {
	const fileContent = fs.readFileSync(path.join(pathToSchemas, file), { encoding: 'utf8' });
	return JSON.parse(fileContent);
});

export interface ValidationResult {
	valid: boolean;
	errors: ErrorObject[];
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function validate(schema: any, test: any): ValidationResult {
	const ajv = new Ajv({
		schemas: schemas.filter((s) => s.$id !== schema.$id),
		allErrors: true,
	});

	const validate = ajv.compile(schema);
	const valid = validate(test);
	return {
		valid,
		errors: validate.errors ?? [],
	};
}
