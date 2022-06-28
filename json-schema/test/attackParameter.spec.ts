// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { validate } from './util';
import schema from '../schema/attackParameter.json';

describe('attackParameter', () => {
	it('must support minimum required fields', () => {
		expect(
			validate(schema, {
				label: 'Secret Key',
				name: 'secretKey',
				type: 'password',
			}).valid
		).toEqual(true);
	});

	it.each`
		field
		${'label'}
		${'name'}
		${'type'}
	`('fail when required field $field is missing', ({ field }) => {
		const value = {
			label: 'Secret Key',
			name: 'secretKey',
			type: 'password',
		};

		delete value[field];

		expect(validate(schema, value).valid).toEqual(false);
	});

	it('must support optional fields', () => {
		expect(
			validate(schema, {
				label: 'Secret Key',
				name: 'secretKey',
				type: 'password',
				description: 'The secret key used to access the system',
				required: true,
				advanced: false,
				order: 42,
				defaultValue: 'secret',
			}).valid
		).toEqual(true);
	});

	it('must required default values to be strings', () => {
		expect(
			validate(schema, {
				label: 'Size',
				name: 'size',
				type: 'integer',
				defaultValue: 42,
			}).valid
		).toEqual(false);
	});
});
