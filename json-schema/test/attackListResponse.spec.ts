// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { validate } from './util';
import schema from '../schema/attackListResponse.json';

describe('attackListResponse', () => {
	it('must support empty attack lists', () => {
		expect(
			validate(schema, {
				attacks: [],
			}).valid
		).toEqual(true);
	});

  it('must support single attack', () => {
		expect(
			validate(schema, {
				attacks: [
          {
            method: 'GET',
            path: '/list'
          }
        ],
			}).valid
		).toEqual(true);
	});

  it('must identify invalid references', () => {
		expect(
			validate(schema, {
				attacks: [
          {
            method: 'POST',
            path: 'non-absolute'
          }
        ],
			}).valid
		).toEqual(false);
	});
});
