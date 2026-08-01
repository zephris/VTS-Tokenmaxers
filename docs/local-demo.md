# Local Demo Setup

## 1. Configure the Server

From the repository root:

```bash
pnpm install
cp apps/server/.env.example apps/server/.env
```

The Go server reads `apps/server/.env` when launched through the repository pnpm commands. Process environment variables take precedence. The important local settings are:

| Variable | Local default | Purpose |
| --- | --- | --- |
| `DATABASE_PATH` | `./data/silent-outposts.db` | SQLite database, relative to `apps/server` |
| `DATASET_ARCHIVE_PATH` | `./data/source/sunken-garden-and-cs-building.zip` | Full source ZIP containing both CSV files and Marv's notes |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173` | Comma-separated exact browser origins |
| `STATION_ACCOUNT_DEFAULT_PASSWORD` | `station-demo` | Password used when creating station accounts that do not exist yet |
| `STATION_SESSION_HOURS` | `12` | Lifetime for station browser sessions |
| `STEGANOGRAPHY_ENABLED` | `false` | Starts the optional Python model when `true` |

No OpenAI key is required. `/api/ai/incident-summary` is a deterministic, evidence-citing stub until hosted AI integration is intentionally added.

## 2. Start the Monitoring Demo

```bash
pnpm dev:data
```

This mode forcibly disables steganography and starts the Vue app and Go API. At startup, the server creates/migrates SQLite and transactionally imports the complete archive: 300 broadcasts, sender history, and the available source notes. Repeated startup is safe and does not duplicate rows.

The same startup creates a login account for every station if one does not already exist. Choose any station in the sign-in screen and use `station-demo` for a local demo password unless you changed `STATION_ACCOUNT_DEFAULT_PASSWORD`.

Deleting `apps/server/data/silent-outposts.db` is a local reset; the next startup reconstructs it from `DATASET_ARCHIVE_PATH`.

Useful checks:

```bash
curl http://localhost:3001/api/health
TOKEN="$(curl -s http://localhost:3001/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"Outpost-Alpha","password":"station-demo"}' \
  | node -pe 'JSON.parse(require("node:fs").readFileSync(0, "utf8")).token')"
curl http://localhost:3001/api/dashboard -H "Authorization: Bearer $TOKEN"
```

## 3. Enable the Headline Steganography Demo

Create an isolated Python environment and install the local model dependencies:

```bash
python3 -m venv apps/server/.venv
apps/server/.venv/bin/python -m pip install --upgrade pip
apps/server/.venv/bin/python -m pip install torch transformers
```

Preflight the exact command used by the Go server:

```bash
cd apps/server
printf '%s\n' '{"op":"info"}' | ./.venv/bin/python conversationstenography/python/hf_model.py --model gpt2 --revision main --device cpu --dtype float32
```

The first run downloads model files and can take several minutes. A successful response resembles:

```json
{"ok":true,"fingerprint":"hf:..."}
```

Set `STEGANOGRAPHY_ENABLED=true` in `apps/server/.env`, return to the repository root, and run `pnpm dev`. The API should report steganography as configured and show the same model fingerprint.

Use two signed-in station identities with one shared phrase to demonstrate encode, transfer of exact carrier text, decode, reply, and transcript convergence. The shared phrase stays in the current browser tab. SQLite persists only the public carrier transcript.

## Troubleshooting

- `503` from steganography routes: confirm `STEGANOGRAPHY_ENABLED=true`, the virtual environment exists, and the preflight succeeds.
- Model fingerprint mismatch: use the same model revision, Python package versions, and all protocol settings on both sides.
- Decode or synchronization failure: restore the exact carrier bytes and verify conversation ID, station transcript, sender spelling, and message order.
- CORS failure: add the browser's exact origin to the comma-separated `CORS_ALLOWED_ORIGINS`; do not use `*` for a deployed environment.
- Archive import failure: verify `DATASET_ARCHIVE_PATH` from the `apps/server` working directory and retain the tracked ZIP under `apps/server/data/source`.

## Distribution Warning

The conversation-steganography module is GPL-3.0 and directly linked into the backend. Distribution may require the combined backend to satisfy GPL-3.0 obligations. See [steganography-integration.md](steganography-integration.md#license) and preserve the nested module's notices. This is not legal advice.
