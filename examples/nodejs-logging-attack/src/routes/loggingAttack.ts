// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { DescribeAttackResponse, StartResponse, PrepareResponse } from '@steadybit/custom-attacks';
import express from 'express';

export const router = express.Router();

router.get('/attacks/logging', (_, res) => {
  const response: DescribeAttackResponse = {
    id: 'logging-attack',
    name: 'Logging Attack',
    description: 'Prints the received payload to the console to illustrate the custom attack API.',
    version: '1.0.0',
    category: 'resource',
    target: 'container',

    parameters: [
      {
        name: 'text',
        label: 'Text',
        type: 'string',
        required: true
      },
      {
        name: 'level',
        label: 'Level',
        type: 'string',
        required: true,
        options: [
          {label: 'Info', value: 'info'},
          {label: 'Warn', value: 'warn'},
          {label: 'Error', value: 'error'}
        ]
      },
    ],

    prepare: {
      path: '/attacks/logging/prepare',
    },
    start: {
      path: '/attacks/logging/start',
    },
    stop: {
      path: '/attacks/logging/stop',
    },
  };
  res.json(response);
});

router.post('/attacks/logging/prepare', (req, res) => {
  console.log('Got prepare request:', JSON.stringify(req.body));

  const response: PrepareResponse = {
    state: {
      generatedBy: 'prepare',
      text: req.body.config.text,
    },
  };
  res.json(response);
});

router.post('/attacks/logging/start', (req, res) => {
  console.log('Got start request:', JSON.stringify(req.body));

  const response: StartResponse = {
    state: {
      ...req.body.state,
      generatedBy: 'start'
    },
  };
  res.json(response);
});

router.post('/attacks/logging/stop', (req, res) => {
  console.log('Got stop request:', JSON.stringify(req.body));
  res.sendStatus(200);
});