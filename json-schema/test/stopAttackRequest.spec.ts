// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { validate } from './util';
import schema from '../schema/stopAttackRequest.json';

describe('stopAttackRequest', () => {
  it('must require state', () => {
		expect(
			validate(schema, {
				canceled: true
			}).valid
		).toEqual(false);
	});

	it('must support minimum required fields', () => {
		expect(
			validate(schema, {
				state: {},
				canceled: false
			}).valid
		).toEqual(true);
	});

	it('must support arbitrary state fields', () => {
		expect(
			validate(schema, {
				state: {
					anything: true,
				},
				canceled: true
			}).valid
		).toEqual(true);
	});
});
