// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { AttackInfoResponse, StateResponse } from '@steadybit/custom-attacks';
import express from 'express';

export const router = express.Router();

router.get('/attacks/logging', (_, res) => {
  const response: AttackInfoResponse = {
    id: 'logging-attack',
    name: 'Logging Attack',
    description: 'Prints the received payload to the console to illustrate the custom attack API.',
    version: '1.0.0',
    // TODO what are the supported values?
    category: 'Logging',
    // TODO what are the supported values?
    target: 'container',

    prepare: {
      url: '/attacks/logging/prepare',
    },
    start: {
      url: '/attacks/logging/start',
    },
    stop: {
      url: '/attacks/logging/stop',
    },
  };
  res.json(response);
});

router.post('/attacks/logging/prepare', (req, res) => {
  console.log('Got prepare request:', req.body);
  
  const response: StateResponse = {
    state: {
      generatedBy: 'prepare'
    },
  };
  res.json(response);
});

router.post('/attacks/logging/start', (req, res) => {
  console.log('Got start request:', req.body);

  const response: StateResponse = {
    state: {
      generatedBy: 'start'
    },
  };
  res.json(response);
});

router.post('/attacks/logging/stop', (req, res) => {
  console.log('Got stop request:', req.body);
  res.sendStatus(200);
});