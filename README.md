# Silent Outposts

A TypeScript monorepo for the Silent Outposts hackathon project: a dashboard for tracking silent outposts, conflicting broadcasts, and evidence-based incident triage in a peacock-disrupted relay network.

## Stack

- Vue 3, Vite, and Naive UI for the web app.
- Apache ECharts through Vue ECharts for charts and data exploration.
- Express 5 and SQLite through `better-sqlite3` for the API and persistence.
- Vercel AI SDK with the OpenAI provider for evidence-based incident summaries (disabled by default).
- A shared package for API contracts, schemas, and domain types.

## Structure

```text
apps/
  web/       Vue application (dashboard, station browser)
  server/    Express API and SQLite database
    src/index.ts     HTTP routes and server mode selection
    src/db.ts        SQLite schema, migration, and queries
    src/import.ts    CSV dataset import and derived sender profiles
    src/csv.ts       RFC-4180 CSV parser
    src/ai.ts        LLM incident brief (only loaded in full mode)
    src/import.test.ts  node:test suite for the dataset import
packages/
  common/    Shared TypeScript types and validation schemas
hackathon/   Focused problem and dataset notes
```

## Dataset import

On every startup the server imports `hackathon/dataset` into SQLite in a single transaction:

- `broadcast_message_log.csv` — all 300 broadcasts, preserving quoted fields (commas inside quotes), empty `message_text`, and empty `signal_strength` (stored as `NULL`).
- `sender_history.csv` — 9 sender profiles imported verbatim. The `location` column is not part of the file, so it is taken from each sender's latest broadcast.
- Sender IDs that appear in the broadcast log but not in `sender_history.csv` (Outpost-Epsilon, Outpost-Theta, Outpost-Zeta, Mini-Marv-04, Mini-Marv-05) get deterministic derived profiles: sender type from the name prefix, first/last seen from broadcast timestamps, reliability from the genuine-label ratio, and `current_status` (`gone_quiet` when silent 72+ hours against the dataset's latest broadcast).

The import is idempotent (`INSERT OR IGNORE`), so deleting the database file restores the full dataset on the next startup. Schema changes are tracked with `PRAGMA user_version`; older databases are dropped and re-imported automatically.

## Server modes

- **Dataset-only (default):** unless `STEGANOGRAPHY_ENABLED=true`, the server never loads or launches an LLM. `POST /api/ai/incident-summary` returns `403`, and all dataset/station APIs work fully offline.
- **Full:** set `STEGANOGRAPHY_ENABLED=true` (and `OPENAI_API_KEY`) to enable the AI incident brief.

`GET /api/health` reports the active mode and dataset counts:

```json
{ "ok": true, "mode": "dataset-only", "counts": { "stations": 14, "broadcasts": 300 } }
```

## API

| Method | Path | Description |
| --- | --- | --- |
| GET | `/api/health` | Mode and dataset counts |
| GET | `/api/dashboard` | Outpost watchlist, risk stats, recent broadcasts |
| GET | `/api/stations` | All stations (read-only) |
| GET | `/api/stations/:senderId/broadcasts` | Broadcasts for one station, `404` for unknown senders |
| POST | `/api/ai/incident-summary` | AI brief in full mode, `403` in dataset-only mode |

## Run locally

Requires Node.js 22+ and pnpm 10+.

```bash
pnpm install
cp .env.example apps/server/.env
pnpm dev
```

Open <http://localhost:5173>. The API runs on <http://localhost:3001> and Vite proxies `/api` to it.

## Commands

```bash
pnpm dev        # run all packages in watch mode
pnpm build      # production builds
pnpm typecheck  # check all TypeScript projects
pnpm test       # server dataset-import tests (node:test)
pnpm start      # run the built API
```
