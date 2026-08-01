# Silent Outposts

A TypeScript monorepo scaffold for the Silent Outposts hackathon project.

## Stack

- Vue 3, Vite, and Naive UI for the web app.
- Apache ECharts through Vue ECharts for charts and data exploration.
- Express 5 and SQLite through `better-sqlite3` for the API and persistence.
- Vercel AI SDK with the OpenAI provider for evidence-based incident summaries.
- A shared package for API contracts, schemas, and domain types.

## Structure

```text
apps/
  web/       Vue application
  server/    Express API and SQLite database
packages/
  common/    Shared TypeScript types and validation schemas
hackathon/   Focused problem and dataset notes
```

## Run locally

Requires Node.js 22+ and pnpm 10+.

```bash
pnpm install
cp .env.example apps/server/.env
pnpm dev
```

Open <http://localhost:5173>. The API runs on <http://localhost:3001> and Vite proxies `/api` to it.

The dashboard and deterministic fallback summary work without an API key. Add `OPENAI_API_KEY` to `apps/server/.env` to enable the AI-generated incident brief.

## Commands

```bash
pnpm dev        # run all packages in watch mode
pnpm build      # production builds
pnpm typecheck  # check all TypeScript projects
pnpm start      # run the built API
```
