// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { validate } from './util';
import schema from '../schema/prepareAttackRequest.json';

describe('prepareAttackRequest', () => {
	it('must support minimum required fields', () => {
		expect(
			validate(schema, {
				config: {
					anything: true,
				},
			}).valid
		).toEqual(true);
	});

	it('must support targets', () => {
		expect(
			validate(schema, {
				config: {
					anything: true,
				},

				target: {
					name: 'gateway',
					attributes: {
						'k8s.deployment': ['gateway'],
						'k8s.cluster': ['demo-dev'],
					},
				},
			}).valid
		).toEqual(true);
	});
});
