# Silent Outposts

A Vue and Go scaffold for the Silent Outposts hackathon project.

## Stack

- Vue 3, Vite, and Naive UI for the web app.
- Apache ECharts through Vue ECharts for charts and data exploration.
- Go `net/http` for the API.
- A deterministic incident-summary fallback.
- A shared package for API contracts, schemas, and domain types.

## Structure

```text
apps/
  web/       Vue application
  server/    Go API
packages/
  common/    Shared TypeScript types and validation schemas
hackathon/   Focused problem and dataset notes
```

## Run locally

Requires Node.js 22+, pnpm 10+, and Go 1.22+.

```bash
pnpm install
cp apps/server/.env.example apps/server/.env
pnpm dev
```

Open <http://localhost:5173>. The API runs on <http://localhost:3001> and Vite proxies `/api` to it.

The dashboard and deterministic fallback summary work without an API key. The first model launch may download GPT-2 and can take a while; remove or comment out `STEGANOGRAPHY_MODEL_COMMAND` to run the rest of the API without it.

## Commands

```bash
pnpm dev        # run all packages in watch mode
pnpm build      # production builds
pnpm typecheck  # check TypeScript packages and run Go server tests
pnpm start      # run the built Go API
```

## Conversation Steganography

The backend has been moved to Go and the upstream core is kept as the separate nested module `apps/server/conversationstenography`. The API starts one persistent model process and reuses it for encode/decode requests.

Install the development model dependencies with `python3 -m pip install torch transformers`, then configure the command, launch arguments, and shared protocol settings in `apps/server/.env`.

See [docs/steganography-integration.md](docs/steganography-integration.md) for the API contract, synchronization requirements, and GPL-3.0 implications.
