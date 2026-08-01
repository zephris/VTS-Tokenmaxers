# Conversation Steganography Integration

Source: <https://github.com/nethical6/conversation-steganography>

## Architecture

The upstream Go core remains a separate nested module at `apps/server/conversationstenography`. The server imports it through a local `replace`, launches one persistent Python model process at startup, and serializes encode/decode work through that warm process.

```text
Vue client -> Go API -> temporary ConversationChain -> persistent Python model
                    -> SQLite public transcript
```

For each operation, the server loads the ordered public records for `(conversationId, stationId)` from SQLite, derives a key from the request's shared phrase, constructs a short-lived chain, and discards the chain and key after the request. Only a successful operation appends a public record.

SQLite may contain conversation IDs, station IDs, sequence information, sender names, model/protocol identity, and exact carrier text. It must never contain the shared phrase, derived key, source plaintext, or recovered plaintext.

## Availability and Configuration

`STEGANOGRAPHY_ENABLED=false` is a supported mode. Steganography endpoints return `503`, while health, dashboard, archive, and deterministic incident-analysis endpoints continue working.

When enabled, `STEGANOGRAPHY_MODEL_COMMAND` identifies the local Python interpreter and `STEGANOGRAPHY_MODEL_ARGS` supplies a JSON array (preferred) or shell-style argument string. The bundled Hugging Face adapter uses GPT-2 by default. The model fingerprint is exposed by health/status data so clients can detect incompatible participants.

All `STEGANOGRAPHY_*` protocol settings, model revision, tokenizer, and dependency versions must match between participants. Do not silently change these values during an active conversation.

See [local-demo.md](local-demo.md) for setup and preflight commands.

## API Contract

All steganography routes require `Authorization: Bearer <station session token>`. Create that token through `POST /api/auth/login` with a station account before calling the examples below.

`POST /api/steganography/encode` accepts:

```json
{
  "conversationId": "team-chat",
  "stationId": "station-a",
  "sender": "alice",
  "secretPhrase": "a shared phrase at least 16 characters long",
  "plaintext": "Meet at the usual place."
}
```

The server loads Station A's public transcript, creates a carrier, persists the new public record, and returns the exact `carrierText`, record/transcript metadata, synchronization code, and algorithm identity. Plaintext may be echoed to the requesting client for the local demo but is not stored.

`POST /api/steganography/decode` accepts the same identity fields, replacing `plaintext` with the exact unedited `carrierText`. The recovered plaintext is returned only to the requesting client. A successful decode appends the carrier record to that station's public transcript.

`GET /api/steganography/conversations/{conversationId}?stationId=station-a` returns that station's ordered public transcript. It never returns a phrase, key, or plaintext.

The server, rather than the browser, owns transcript history. Clients must not send an editable `records` array. A duplicate or out-of-sequence operation is rejected without a partial database write.

## Synchronization and Failure Rules

- Carrier text is protocol data: never trim, normalize, reflow, spell-check, or otherwise modify it.
- Record order, sender spelling, sender sequence, conversation ID, model fingerprint, tokenizer/model revision, and protocol settings are synchronization and authentication inputs.
- A wrong phrase, altered carrier, incompatible model, or divergent transcript must fail explicitly; it must not be treated as a valid empty message.
- Model launch or runtime errors must be actionable but must not include phrases, plaintext, keys, environment values, or request bodies.
- The model is expensive to start and is intentionally reused. Do not launch a process per request.

## Security Limitations

This is experimental proof-of-concept steganography, not audited cryptographic software. Generated text may be detectable, and the application must not claim otherwise. Use TLS in any non-local environment and disable request-body logging at every proxy layer.

The public transcript is intentionally not confidential, but its exact bytes, integrity, and order are security-relevant. Secret phrases belong only in request memory and, on the web client, the current tab's `sessionStorage`.

## License

The nested module is GPL-3.0 licensed. Its source, tests, Python adapters, upstream README, source record, and license are preserved in `apps/server/conversationstenography`.

Directly linking the module into the Go server may require the combined backend to comply with GPL-3.0 when distributed. Confirm those obligations before distribution or deployment. This is not legal advice.
