// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { validate } from './util';
import schema from '../schema/prepareAttackResponse.json';

describe('prepareAttackResponse', () => {
  it('must require state', () => {
		expect(
			validate(schema, {
			}).valid
		).toEqual(false);
	});

	it('must support minimum required fields', () => {
		expect(
			validate(schema, {
				state: {},
			}).valid
		).toEqual(true);
	});

	it('must support arbitrary state fields', () => {
		expect(
			validate(schema, {
				state: {
					anything: true,
				},
			}).valid
		).toEqual(true);
	});
});
