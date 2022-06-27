// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { validate } from './util';
import describingEndpointRefSchema from '../schema/describingEndpointRef.json';
import mutatingEndpointRefSchema from '../schema/mutatingEndpointRef.json';

describe('httpEndpointRef', () => {
	describe.each`
		schemaName                 | method   | schema
		${'describingEndpointRef'} | ${'GET'} | ${describingEndpointRefSchema}
		${'mutatingEndpointRef'}   | ${'PUT'} | ${mutatingEndpointRefSchema}
	`('$schemaName', ({ method, schema }) => {
		it('must successfully parse HTTP endpoint references', () => {
			expect(
				validate(schema, {
					method,
					path: '/something',
				}).valid
			).toEqual(true);
		});

		it('must require HTTP method', () => {
			expect(
				validate(schema, {
					path: '/something',
				}).valid
			).toEqual(false);
		});

		it('must require specific HTTP methods', () => {
			expect(
				validate(schema, {
					method: 'HEAD',
					path: '/something',
				}).valid
			).toEqual(false);
		});

		it('must require absolute paths', () => {
			expect(
				validate(schema, {
					method,
					path: 'something',
				}).valid
			).toEqual(false);
		});
	});
});
