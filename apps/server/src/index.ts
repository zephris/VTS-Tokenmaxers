import 'dotenv/config';
import cors from 'cors';
import express from 'express';
import { incidentSummaryRequestSchema } from '@vts/common';
import {
  createDatabase,
  getCounts,
  getDashboard,
  getIncidentEvidence,
  getStationBroadcasts,
  getStations,
} from './db.js';

const steganographyEnabled = process.env.STEGANOGRAPHY_ENABLED === 'true';
const mode: 'full' | 'dataset-only' = steganographyEnabled ? 'full' : 'dataset-only';

const db = createDatabase();

const app = express();
const port = Number(process.env.PORT ?? 3001);

app.use(cors());
app.use(express.json());

app.get('/api/health', (_request, response) => {
  response.json({ ok: true, mode, counts: getCounts(db) });
});

app.get('/api/dashboard', (_request, response) => {
  response.json(getDashboard(db));
});

app.get('/api/stations', (_request, response) => {
  response.json({ stations: getStations(db) });
});

app.get('/api/stations/:senderId/broadcasts', (request, response) => {
  const { senderId } = request.params;
  const broadcasts = getStationBroadcasts(db, senderId);
  if (broadcasts === null) {
    response.status(404).json({ error: `Unknown station "${senderId}".` });
    return;
  }
  response.json({ senderId, broadcasts });
});

if (steganographyEnabled) {
  const { summarizeIncident } = await import('./ai.js');

  app.post('/api/ai/incident-summary', async (request, response, next) => {
    try {
      const parsed = incidentSummaryRequestSchema.safeParse(request.body);
      if (!parsed.success) {
        response.status(400).json({ error: 'A valid senderId is required.' });
        return;
      }

      const evidence = getIncidentEvidence(db, parsed.data.senderId);
      const result = await summarizeIncident(parsed.data.senderId, evidence);
      response.json(result);
    } catch (error) {
      next(error);
    }
  });
} else {
  app.post('/api/ai/incident-summary', (_request, response) => {
    response.status(403).json({
      error: 'AI incident summaries are disabled in dataset-only mode. Set STEGANOGRAPHY_ENABLED=true to enable them.',
    });
  });
}

app.use((error: unknown, _request: express.Request, response: express.Response, _next: express.NextFunction) => {
  console.error(error);
  response.status(500).json({ error: 'Unexpected server error.' });
});

app.listen(port, () => {
  console.log(`Silent Outposts API listening on http://localhost:${port} (mode: ${mode})`);
});
