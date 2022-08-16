// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { IndexResponse } from '@steadybit/action-api';
import express from 'express';

export const router = express.Router();

router.get('/actions', (_, res) => {
	const response: IndexResponse = {
		attacks: [
			{
				path: '/actions/logging',
			},
		],
	};
	res.json(response);
});
