// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

import { router as indexRouter } from './routes/index';
import { router as loggingActionRouter } from './routes/logging';
import express from 'express';
import cors from 'cors';

const app = express();
const port = 8084;

app.use(express.json());

app.use(cors());
app.use(indexRouter);
app.use(loggingActionRouter);

app.listen(port, () => {
	console.log(`Action implementation listening on ${port}.`);
	console.log();
	console.log(`Action list can be accessed via GET http://127.0.0.1:${port}/actions`);
});
