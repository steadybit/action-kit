// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { ActionList } from '@steadybit/action-kit-api';
import express from 'express';

export const router = express.Router();

router.get('/actions', (_, res) => {
	const response: ActionList = {
		actions: [
			{
				method: 'get',
				path: '/actions/logging',
			},
		],
	};
	res.json(response);
});
