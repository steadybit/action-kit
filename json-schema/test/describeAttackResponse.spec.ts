// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { validate } from './util';
import schema from '../schema/describeAttackResponse.json';

describe('describeAttackResponse', () => {
	it('must support minimum required fields', () => {
		expect(
			validate(schema, {
				id: 'org.example.attacks.foobar',
				label: 'My Foobar Attack',
				description: 'Something about the attack',
				version: '1.0.0',
				category: 'resource',
				target: 'host',
				timeControl: 'ONE_SHOT',
				parameters: [],
				prepare: {
					method: 'POST',
					path: '/prepare',
				},

				start: {
					method: 'POST',
					path: '/start',
				},

				stop: {
					method: 'POST',
					path: '/stop',
				},
			}).valid
		).toEqual(true);
	});

  it('must validate nested parameters', () => {
		expect(
			validate(schema, {
				id: 'org.example.attacks.foobar',
				label: 'My Foobar Attack',
				description: 'Something about the attack',
				version: '1.0.0',
				category: 'resource',
				target: 'host',
				timeControl: 'ONE_SHOT',
				parameters: [
          {
            label: 'Secret Key',
            name: 'secretKey',
            type: 'password',
          }
        ],
				prepare: {
					method: 'POST',
					path: '/prepare',
				},

				start: {
					method: 'POST',
					path: '/start',
				},

				stop: {
					method: 'POST',
					path: '/stop',
				},
			}).valid
		).toEqual(true);
	});

  it('must validate nested parameters', () => {
		expect(
			validate(schema, {
				id: 'org.example.attacks.foobar',
				label: 'My Foobar Attack',
				description: 'Something about the attack',
				version: '1.0.0',
				category: 'resource',
				target: 'host',
				timeControl: 'ONE_SHOT',
				parameters: [
          {
            label: 'Secret Key',
            name: 'secretKey',
            type: 'unknown',
          }
        ],
				prepare: {
					method: 'POST',
					path: '/prepare',
				},

				start: {
					method: 'POST',
					path: '/start',
				},

				stop: {
					method: 'POST',
					path: '/stop',
				},
			}).valid
		).toEqual(false);
	});
});
