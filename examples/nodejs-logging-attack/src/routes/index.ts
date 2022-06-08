// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { IndexResponse } from '@steadybit/custom-attacks';
import express from 'express';

export const router = express.Router();

router.get('/attacks', (_, res) => {
  const response: IndexResponse = {
    attacks: [
      {
        path: '/attacks/logging',
      }
    ]
  };
  res.json(response);
});

