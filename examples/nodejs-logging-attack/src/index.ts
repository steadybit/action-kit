// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { router as indexRouter } from './routes/index';
import { router as loggingAttackRouter } from './routes/loggingAttack';
import express from 'express';

const app = express();
const port = 3000;

app.use(express.json());

app.use(indexRouter);
app.use(loggingAttackRouter);

app.listen(port, () => {
  console.log(`Custom attack implementation listening on ${port}.`);
  console.log();
  console.log(`Attack list can be accessed via GET http://127.0.0.1:${port}/attacks`);
  
});
