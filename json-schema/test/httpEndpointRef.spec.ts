// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { validate } from './util';
import schema from '../attack-api.schema.json';

describe('httpEndpointRef', () => {
	it('must successfully parse HTTP endpoint references', () => {
		expect(
			validate(schema.$defs.httpEndpointRef, {
				method: 'GET',
				path: '/something',
			}).valid
		).toEqual(true);
	});

	it('must require HTTP method', () => {
		expect(
			validate(schema.$defs.httpEndpointRef, {
				path: '/something',
			}).valid
		).toEqual(false);
	});

	it('must require specific HTTP methods', () => {
		expect(
			validate(schema.$defs.httpEndpointRef, {
				method: 'HEAD',
				path: '/something',
			}).valid
		).toEqual(false);
	});

	it('must require absolute paths', () => {
		expect(
			validate(schema.$defs.httpEndpointRef, {
				method: 'GET',
				path: 'something',
			}).valid
		).toEqual(false);
	});
});
