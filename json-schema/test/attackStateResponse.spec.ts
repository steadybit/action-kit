// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { validate } from './util';
import schema from '../schema/attackStateResponse.json';

describe('attackStateResponse', () => {
	it('must not require state', () => {
		expect(validate(schema, {
		}).valid).toEqual(false);
	});

	it('must support minimum required fields', () => {
		expect(
			validate(schema, {
				completed: false
			}).valid
		).toEqual(true);
	});

	it('must support arbitrary state fields', () => {
		expect(
			validate(schema, {
				state: {
					anything: true,
				},
				completed: true
			}).valid
		).toEqual(true);
	});

	it('must support log messages', () => {
		expect(
			validate(schema, {
				state: {
					anything: true,
				},
				completed: true,
				messages: [{ message: 'one', level: 'debug' }, { message: 'two', level: 'info' }, { message: 'three' }],
			}).valid
		).toEqual(true);
	});

	it('must support rfc 7807 problems', () => {
		expect(
			validate(schema, {
				title: 'Something went wrong',
				details: 'Terrible things happens',
			}).valid
		).toEqual(true);
	});
});
