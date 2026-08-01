import 'dotenv/config';
import cors from 'cors';
import express from 'express';
import { incidentSummaryRequestSchema } from '@vts/common';
import { summarizeIncident } from './ai.js';
import { getDashboard, getIncidentEvidence } from './db.js';

const app = express();
const port = Number(process.env.PORT ?? 3001);

app.use(cors());
app.use(express.json());

app.get('/api/health', (_request, response) => {
  response.json({ ok: true });
});

app.get('/api/dashboard', (_request, response) => {
  response.json(getDashboard());
});

app.post('/api/ai/incident-summary', async (request, response, next) => {
  try {
    const parsed = incidentSummaryRequestSchema.safeParse(request.body);
    if (!parsed.success) {
      response.status(400).json({ error: 'A valid senderId is required.' });
      return;
    }

    const evidence = getIncidentEvidence(parsed.data.senderId);
    const result = await summarizeIncident(parsed.data.senderId, evidence);
    response.json(result);
  } catch (error) {
    next(error);
  }
});

app.use((error: unknown, _request: express.Request, response: express.Response, _next: express.NextFunction) => {
  console.error(error);
  response.status(500).json({ error: 'Unexpected server error.' });
});

app.listen(port, () => {
  console.log(`Silent Outposts API listening on http://localhost:${port}`);
});

