# Llama Wrangler

Llama Wrangler is a local-first control plane and friendly local web app for Ollama fleets. It presents one OpenAI- and Ollama-compatible endpoint while coordinating multiple local machines behind the scenes.

The normal user path is UI-first: install or run the binary, open `http://localhost:11435/ui`, scan the local machine, accept recommended safe defaults, optionally configure Splunk HEC, then point an IDE or agent at one endpoint. YAML config files remain available for advanced users, but manual config editing is not required for the default flow.

## Project Ledger

Work is coordinated through [docs/00_project_ledger.md](docs/00_project_ledger.md). It tracks binding requirements, implementation state, decisions, risks, vulnerabilities, ideas, side quests, and next actions so the main plan stays intact while the product evolves.

## Quick Start

```bash
go build ./cmd/llama-wrangler
./llama-wrangler start
```

Open:

```text
http://localhost:11435/ui
```

The service binds to localhost by default. LAN access, external Frontier Delta providers, and prompt/response payload logging are disabled until explicitly configured.

The IDE / Agent Setup page includes preset cards for Cline, Continue, Open WebUI, and generic OpenAI SDK clients. Preset API responses use `<client-api-key>` placeholders; generated client API keys are still shown only once in the browser and stored outside ordinary app state.

## Service Modes

```bash
llama-wrangler marshal --config ./configs/marshal.example.yaml
llama-wrangler subscriber --config ./configs/subscriber.example.yaml
llama-wrangler standalone --config ./configs/marshal.example.yaml
llama-wrangler service-dry-run --target launchd --keychain
```

The CLI exists for automation, but the setup wizard is the primary configuration surface for normal operation.

## API Endpoints

OpenAI-compatible:

- `GET /v1/models`
- `POST /v1/chat/completions`
- `POST /v1/completions`
- `POST /v1/embeddings`

Ollama-compatible:

- `GET /api/tags`
- `POST /api/chat`
- `POST /api/generate`

Management UI/API:

- `GET /ui`
- `GET /wrangler/ui/bootstrap`
- `GET /wrangler/client-presets`
- `POST /wrangler/setup/scan-local`
- `GET /wrangler/nodes`
- `GET /wrangler/models`
- `GET /wrangler/audit/recent`

## Safe Defaults

- Marshal listens on `127.0.0.1:11435` by default.
- Settings warns when the configured listen address exposes the service beyond localhost.
- Management APIs are protected by a generated local admin token after setup completion.
- IDE and agent clients use generated client API keys after setup completion.
- Repeated invalid admin or client API-key attempts are rate limited with `Retry-After`.
- Admin tokens can be rotated from Settings, and client keys can be regenerated or revoked from IDE setup.
- Persisted app state is schema-versioned and migrated forward automatically.
- Secrets are kept out of ordinary app state and stored in an encrypted local fallback store.
- The encrypted local fallback key can be rotated from Settings when the key source is the local key file.
- Frontier providers are disabled by default.
- Local-only policy is enabled by default.
- Streaming requests retry only before client-visible output begins; partial streamed output is never silently retried.
- Streaming compatibility is checked for OpenAI SSE and Ollama newline-delimited JSON clients.
- Queue priority and activity are exposed as metadata-only operational state, with weighted-priority dispatch controls in the UI.
- Retry, partial-response, and cancellation counts are surfaced as metadata-only operation stats.
- OpenAI- and Ollama-compatible errors use client-appropriate shapes without echoing payloads or secrets.
- Support bundle export includes diagnostic metadata while excluding secrets and payloads.
- Support bundle export includes versioned `bundle_schema` metadata with a JSON Schema at `schemas/support_bundle.schema.json`.
- Support bundles are diagnostics only; encrypted fallback secrets require backing up `secrets.enc.json` with its matching key source.
- An opt-in OS keychain backend spike is available with `LLAMA_WRANGLER_SECRET_BACKEND=os_keychain`; encrypted fallback remains available and is still the default storage path. Secret-storage status reports platform/runtime metadata and warns for service-like runtimes until service-user keychain behavior is verified.
- The `service-dry-run` command prints review-only launchd service-wrapper artifacts and keychain validation commands without installing or starting a service.
- Prompt and response bodies are not written to telemetry by default.
- Splunk HEC is optional and can be tested from the UI.
- Splunk HEC supports a TLS verification toggle for trusted self-signed lab certificates.
- The UI shows an explicit warning when Splunk HEC TLS verification is disabled.
- Session affinity defaults to `soft` for client continuity.

## Core positioning

Llama Wrangler is not model-parallel distributed inference. It does not split a single model across machines. Instead, it distributes requests, subtasks, review passes, validation, and consensus workflows across local endpoints.

## Primary modes

- `single`: Route one request to the best local node.
- `race`: Ask multiple nodes and return the first valid response.
- `consensus`: Ask multiple nodes and compare outputs.
- `consensus_delta`: Ask multiple capable local nodes, compare results, and escalate only unresolved deltas when allowed.
- `frontier_delta`: Use frontier models only for minimized external review payloads.
- `local_only`: Never use external models.

## Companion Splunk app

The companion Splunk app ingests Llama Wrangler HEC events and provides operational visibility into request routing, model performance, endpoint health, consensus behavior, frontier usage, cost controls, and failure modes.

## Screenshots

Screenshot placeholders live with the UI-first docs until packaging polish is complete.

## Troubleshooting

- If Ollama detection fails, confirm Ollama is running at `http://localhost:11434`, then run the setup scan again.
- If Splunk HEC returns 403, confirm the token is enabled and allowed to write to the configured index.
- If an IDE cannot connect, confirm it uses `http://localhost:11435/v1` for OpenAI-compatible clients or `http://localhost:11435` for Ollama-compatible clients.
