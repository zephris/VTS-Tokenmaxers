# AGENTS.md

This file applies to the entire repository.

## Project Overview

Silent Outposts is a hackathon project with a Vue web application and a Go API. The current product combines an operational outpost dashboard with an experimental encrypted-chat feature that hides ciphertext in AI-generated conversational text.

The steganography implementation is based on `nethical6/conversation-steganography` and is experimental, deterministic, and synchronization-sensitive.

## Repository Layout

```text
apps/web/                           Vue 3 and Vite frontend
apps/server/                        Go HTTP API
apps/server/internal/               Server-owned domain packages
apps/server/conversationstenography Separate vendored Go module
packages/common/                    Shared TypeScript contracts and schemas
hackathon/                          Challenge notes and datasets
docs/                               Architecture and integration notes
criteria.md                         Hackathon requirements and scoring notes
```

## Toolchain And Commands

Use Node.js 22+, pnpm 10+, and Go 1.22+.

```bash
pnpm install
pnpm dev
pnpm typecheck
pnpm build
```

Focused commands:

```bash
pnpm --filter @vts/web typecheck
pnpm --filter @vts/server typecheck
go test ./...                                    # from apps/server
go test ./...                                    # from apps/server/conversationstenography
go vet ./...                                     # run in each Go module
```

Run `gofmt` on changed Go files. Do not commit generated directories such as `node_modules`, `apps/web/dist`, `packages/common/dist`, or `apps/server/bin`.

## Development Conventions

- Keep changes scoped to the requested behavior and follow existing package boundaries.
- Use `packages/common` for browser-facing TypeScript contracts shared across frontend modules.
- Keep HTTP routing and transport concerns in `apps/server/main.go`; place reusable server behavior under `apps/server/internal`.
- Preserve the existing `/api` prefix. Vite proxies it to the Go server on port `3001`.
- Update tests with behavior changes. Run the focused checks while iterating and `pnpm typecheck` plus `pnpm build` before handoff.
- Keep `.env` files local. When adding configuration, update `apps/server/.env.example` and the relevant documentation.
- Treat files under `hackathon/dataset` as source data. Do not rewrite them as incidental formatting cleanup.

## Frontend Guidance

- Use Vue 3 Composition API and the existing Naive UI and ECharts dependencies.
- Match the current operational dashboard styling and interaction patterns.
- Keep interfaces responsive and keyboard-accessible, and provide explicit loading, empty, success, and error states for network operations.
- Do not expose secret phrases in URLs, browser logs, analytics, or persisted application state.
- The encrypted-chat workflow should make plaintext, generated carrier text, recovered plaintext, transcript synchronization, and model compatibility easy to inspect.

## Go Server Guidance

- The server uses the standard `net/http` stack; do not reintroduce the deleted TypeScript/Express backend.
- Start the language-model process once and reuse it. Model startup is expensive, and the upstream `ProcessModel` serializes access internally.
- Build a short-lived `ConversationChain` from the request's secret phrase and preceding public records. Do not retain plaintext passphrases or derived keys between requests.
- Never trim, normalize, spell-check, or otherwise modify carrier text. Decoding requires the exact generated string.
- Preserve record order, sender spelling, conversation ID, model fingerprint, tokenizer/model revision, and protocol settings. They are part of synchronization and authentication.
- Return actionable errors without including passphrases, derived keys, private message contents, or process environment values.
- Keep model launch arguments and protocol settings in `apps/server/.env`; `STEGANOGRAPHY_MODEL_ARGS` may be a JSON array or shell-style string.

## Conversationstenography Module

`apps/server/conversationstenography` is intentionally its own Go module. The parent server imports it using the local `replace` directive in `apps/server/go.mod`.

- Preserve the nested module boundary and package name `conversationstenography`.
- Do not move its code into `apps/server/internal` or merge its `go.mod` into the server module.
- Prefer adapters in `apps/server/internal/stego` over application-specific edits to the vendored core.
- When updating upstream code, record the exact commit and retrieval date in `apps/server/conversationstenography/SOURCE.md`.
- Preserve the upstream tests, Python model adapters, README, and GPL-3.0 license.
- Run tests in both Go modules after changing the core or its adapter.

## Security And Licensing

The steganography project is a proof of concept, not audited cryptographic software. Use TLS in deployed environments, avoid request-body logging, and do not claim that generated carrier text is undetectable.

The nested module is GPL-3.0 licensed. Directly linking it may impose GPL-3.0 obligations on the distributed backend. Preserve license notices and consult `docs/steganography-integration.md` before changing the integration or deployment model.
