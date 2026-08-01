# Conversation Steganography Integration

Source repository: https://github.com/nethical6/conversation-steganography

## Architecture

The backend uses Option A: the upstream Go core lives in `apps/server/conversationstenography` as a separate nested Go module. The server module imports it through a local `replace` directive, launches one persistent Python model process at startup, and reuses that warm model for every request.

Each encode/decode request supplies the shared phrase and all public records that precede the new message. The server derives the key, reconstructs a temporary `ConversationChain`, performs the operation, and discards the chain and key. The shared phrase is not written to server state or logs.

```text
web client -> Go API -> ConversationChain -> persistent ProcessModel -> local Python model
```

The upstream package serializes calls to the model process. This is appropriate for the initial single-worker hackathon deployment; higher concurrency will need a pool of model workers.

## Server Configuration

The Go server loads `apps/server/.env` on startup without overriding variables already present in the process environment. Set a different file with `ENV_FILE`.

Start from `apps/server/.env.example`. The launch settings are:

- `STEGANOGRAPHY_MODEL_COMMAND`: executable used to start the model, normally `python3`. If omitted, the steganography endpoints return `503` while the rest of the API remains available.
- `STEGANOGRAPHY_MODEL_ARGS`: either a shell-style argument string or, preferably, a JSON string array. The example launches the bundled Hugging Face adapter with GPT-2.
- `STEGANOGRAPHY_*` protocol settings: prompt, coding mode, candidate counts, temperature, style checks, and generation limits.

The model, exact model revision, tokenizer, and all protocol settings must match between participants. The health endpoint reports the model fingerprint so clients can compare it.

## API Contract

`POST /api/steganography/encode` accepts:

```json
{
  "conversationId": "team-chat",
  "sender": "alice",
  "secretPhrase": "a shared phrase at least 16 characters long",
  "plaintext": "Meet at the usual place.",
  "records": []
}
```

It returns the original `plaintext`, generated `carrierText`, the new `record`, the complete updated `records` transcript, and a `syncCode`.

`POST /api/steganography/decode` accepts the same conversation details and prior `records`, replacing `plaintext` with the exact unedited `carrierText`. It returns the recovered `plaintext` and updated public transcript.

For both endpoints, `records` means every accepted message before the message currently being encoded or decoded. Use the returned `records` array as the next request's input. The carrier text must not be trimmed, reformatted, or corrected.

## Security And Reliability

- The phrase is required for each request and is never returned. Production deployments should use TLS and avoid request-body logging at every proxy layer.
- Message order, sender spelling, conversation ID, carrier text, model fingerprint, and protocol settings are authenticated synchronization state. A mismatch causes decoding to fail.
- The public transcript is not secret and may be persisted client-side or in a database. It still needs integrity and ordering controls in a multi-user implementation.
- The upstream project describes itself as a proof of concept. Treat it as experimental rather than audited cryptographic software.

## License

The upstream module is GPL-3.0 licensed. Its source, tests, Python adapters, upstream README, and license are preserved inside `apps/server/conversationstenography`.

Directly linking this module into the server may require the combined backend to comply with GPL-3.0 obligations when distributed. Confirm that this is acceptable before deployment. This note is not legal advice.
