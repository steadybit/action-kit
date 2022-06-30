// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { validate } from './util';
import schema from '../schema/stopAttackResponse.json';

describe('stopAttackResponse', () => {
	it('must support empty response', () => {
		expect(validate(schema, {}).valid).toEqual(true);
	});

	it('must support log messages', () => {
		expect(
			validate(schema, {
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
