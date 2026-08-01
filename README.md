# Silent Outposts

Silent Outposts is a local hackathon demo for finding robot outposts that have gone unexpectedly quiet and exchanging hidden messages through AI-generated carrier text.

## Architecture

- Vue 3, Naive UI, and Apache ECharts provide the operational dashboard.
- A Go `net/http` API owns data access, analysis, and steganography operations.
- SQLite stores the imported station archive and public steganography transcript records.
- Incident analysis is currently a deterministic, evidence-citing stub. It does not call OpenAI or another hosted model.
- An optional local Python/Hugging Face process generates and decodes steganographic carrier text. The Go server starts it once and reuses it.

The server imports `apps/server/data/source/sunken-garden-and-cs-building.zip` into SQLite on startup. Import is transactional and idempotent, so deleting the development database and restarting reconstructs it from the source archive.

## Quick Start: Dashboard

Requirements: Node.js 22+, pnpm 10+, Go 1.22+, and Python 3.10+ only if enabling steganography.

```bash
pnpm install
cp apps/server/.env.example apps/server/.env
pnpm dev:data
```

Open <http://localhost:5173>. The API runs at <http://localhost:3001>, and Vite proxies `/api` requests to it.

On first startup, the server creates one account for every imported station. Use any station ID as the username, for example `Outpost-Alpha`, with the local demo password `station-demo` unless `STATION_ACCOUNT_DEFAULT_PASSWORD` is changed in `apps/server/.env`.

`pnpm dev:data` explicitly disables the Python model. The full dataset, dashboard, station history, SQLite persistence, and deterministic incident summary remain available.

## Run the Go Backend Only

Use this when you want the API without the Vite frontend. From the repository root, install dependencies once and create the server env file:

```bash
pnpm install
cp apps/server/.env.example apps/server/.env
```

Start the Go API in data-only mode:

```bash
cd apps/server
STEGANOGRAPHY_ENABLED=false go run .
```

The backend listens on <http://localhost:3001> by default. On first startup it creates/migrates `apps/server/data/silent-outposts.db`, imports the source archive, and seeds one login account per station. Use a station ID such as `Outpost-Alpha` with password `station-demo` unless you changed `STATION_ACCOUNT_DEFAULT_PASSWORD`.

To use a different port:

```bash
cd apps/server
PORT=3002 STEGANOGRAPHY_ENABLED=false go run .
```

Quick health and authenticated API checks:

```bash
curl http://localhost:3001/api/health

TOKEN="$(curl -s http://localhost:3001/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"Outpost-Alpha","password":"station-demo"}' \
  | node -pe 'JSON.parse(require("node:fs").readFileSync(0, "utf8")).token')"

curl http://localhost:3001/api/dashboard -H "Authorization: Bearer $TOKEN"
```

You can also run the backend through pnpm:

```bash
pnpm --filter @vts/server dev:data
```

Enable the full steganography backend only after setting up the Python model in the next section and setting `STEGANOGRAPHY_ENABLED=true` in `apps/server/.env`.

## Headline Demo: Conversation Steganography

The steganography feature needs a local Hugging Face model. Set it up and run its preflight once:

```bash
python3 -m venv apps/server/.venv
apps/server/.venv/bin/python -m pip install --upgrade pip
apps/server/.venv/bin/python -m pip install torch transformers
cd apps/server
printf '%s\n' '{"op":"info"}' | ./.venv/bin/python conversationstenography/python/hf_model.py --model gpt2 --revision main --device cpu --dtype float32
```

The first preflight downloads GPT-2 and may take several minutes. Success prints JSON containing `"ok":true` and an `hf:` model fingerprint. Then set `STEGANOGRAPHY_ENABLED=true` in `apps/server/.env`, return to the repository root, and run:

```bash
pnpm dev
```

Both participants must use the same model revision, tokenizer, protocol settings, conversation ID, message order, sender spelling, and shared phrase. Carrier text must be copied exactly.

See [Local demo setup](docs/local-demo.md) for configuration and troubleshooting, and [Conversation steganography integration](docs/steganography-integration.md) for its API and synchronization contract.

## Data and Privacy

- SQLite stores station/archive data and public carrier transcript records.
- Station accounts and expiring session token hashes are stored in SQLite; raw session tokens live only in the current browser tab's `sessionStorage`.
- Shared phrases, derived keys, and plaintext messages are never persisted.
- A phrase may be kept in the current browser tab's `sessionStorage`; it must not enter URLs, logs, analytics, or durable browser storage.
- Steganography is experimental proof-of-concept software, not audited cryptography and not guaranteed to be undetectable. Use TLS outside this local demo.

## Commands

```bash
pnpm dev        # dashboard plus steganography when enabled in apps/server/.env
pnpm dev:data   # dashboard with the model forcibly disabled
pnpm typecheck  # TypeScript checks and Go tests
pnpm build      # production frontend and backend builds
pnpm start      # run the previously built Go API
```

Focused backend checks:

```bash
cd apps/server
go test ./...
go vet ./...
cd conversationstenography
go test ./...
go vet ./...
```

## License Warning

`apps/server/conversationstenography` is derived from GPL-3.0 software and is linked directly into the Go backend. Distributing the combined backend may impose GPL-3.0 obligations. Preserve its license and source notices, and review [the integration notes](docs/steganography-integration.md#license) before distributing or deploying the application. This is not legal advice.
