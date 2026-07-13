# Llama Wrangler

<img width="768" height="512" alt="IMG_6600" src="https://github.com/user-attachments/assets/b4129459-2630-49d7-b621-72071b173276" />


Llama Wrangler is a local-first control plane and friendly local web app for Ollama fleets. It presents one OpenAI- and Ollama-compatible endpoint while coordinating multiple local machines behind the scenes.

The normal user path is UI-first: install or run the binary, open `http://localhost:11435/ui`, scan the local machine, accept recommended safe defaults, optionally configure Splunk HEC, then point an IDE or agent at one endpoint. YAML config files remain available for advanced users, but manual config editing is not required for the default flow.

## Project Ledger

Work is coordinated through [docs/00_project_ledger.md](docs/00_project_ledger.md). It tracks binding requirements, implementation state, decisions, risks, vulnerabilities, ideas, side quests, and next actions so the main plan stays intact while the product evolves.

## Current Status

Llama Wrangler is in V1 hardening and release preparation. The Go service reports pre-release version `0.1.0`; the companion Splunk app is version `0.2.0`.

Implemented and covered by local automated acceptance:

- embedded setup and operations UI with localhost-first safe defaults
- schema-versioned state, encrypted fallback secrets, token rotation, authentication rate limits, and sanitized support bundles
- Managed Node enrollment, approval, signed heartbeat credentials, freshness policy, benchmark jobs, model lifecycle metadata, and keep-warm action foundations
- Passive Endpoint registration, safe `/api/tags` probing, limited-control policy, and explicit UI warnings
- weighted queue scheduling, session affinity, policy-aware routing, and deterministic non-streaming consensus across eligible Managed Nodes
- OpenAI- and Ollama-compatible APIs, normalized error shapes, and single-route streaming retry/cancellation semantics
- metadata-only Splunk HEC telemetry, Overview and Operations dashboards, packaged searches/macros/event metadata, and TLS verification controls
- consolidated V1 acceptance harness plus disposable macOS user-level launchd lifecycle validation

Current release gaps:

- the Splunk app `0.2.0` package has been installed by the operator, but sanitized event ingestion, every dashboard/search, permissions, and performance still require recorded runtime validation
- signed/notarized macOS packaging, normal-path installer behavior, Linux/Windows service packages, and packaged keychain behavior remain open
- package-candidate checks for Cline, Continue, Open WebUI, the generic OpenAI SDK, and Ollama CLI remain open
- V1 streaming consensus remains deliberately unsupported; consensus requests must be non-streaming
- Frontier Delta is policy/configuration groundwork only and remains disabled by default

Phase B and V1 release acceptance are not yet marked complete. The detailed evidence and remaining gates live in [docs/21_v1_acceptance_security_matrix.md](docs/21_v1_acceptance_security_matrix.md).

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
llama-wrangler service-dry-run --target launchd
```

The CLI exists for automation, but the setup wizard is the primary configuration surface for normal operation.

## V1 Acceptance Harness

Run the consolidated automated release gates from the repository root:

```bash
./scripts/v1_acceptance.sh
```

The harness uses disposable app data and a temporary loopback port. It runs focused and full tests, validates schemas/Splunk assets, then performs a live setup, authentication, Managed Node enrollment/approval, encrypted-secret, support-bundle, and restart flow. It does not install or mutate real OS services. Environment-dependent release gates remain listed in `docs/21_v1_acceptance_security_matrix.md` and are not implied by a local pass.

On macOS, validate the user-level launchd package plan without registering a job:

```bash
./scripts/macos_user_launchd_acceptance.sh
```

The launchd harness is dry-run by default. Its explicit opt-in mode uses only a unique current-user launchd label and disposable home/package paths:

```bash
LLAMA_WRANGLER_MACOS_LAUNCHD_ACCEPTANCE=1 ./scripts/macos_user_launchd_acceptance.sh
```

That check covers disposable install/start/restart/atomic-upgrade/uninstall mechanics with encrypted fallback. It does not validate a signed package candidate, notarization, normal user install paths, or packaged OS keychain behavior.

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
- Non-streaming consensus fans out to bounded approved, fresh, trusted Managed Nodes, compares responses in memory, and returns one deterministic OpenAI- or Ollama-compatible winner without logging prompt or response bodies.
- Consensus defaults to 2 required and 4 maximum participants, caps fan-out at 8, excludes Passive Endpoints, and deliberately rejects V1 streaming consensus before upstream fan-out until a separate bounded aggregation protocol is designed and tested.
- Consensus partial failures use fixed metadata-only reason codes; enough successful participants still aggregate deterministically, while unmet quorum returns OpenAI- or Ollama-shaped errors without relaying upstream error bodies.
- OpenAI- and Ollama-compatible errors use client-appropriate shapes without echoing payloads or secrets.
- Support bundle export includes diagnostic metadata while excluding secrets and payloads.
- Support bundle export includes versioned `bundle_schema` metadata with a JSON Schema at `schemas/support_bundle.schema.json`.
- Support bundles are diagnostics only; encrypted fallback secrets require backing up `secrets.enc.json` with its matching key source.
- An opt-in OS keychain backend spike is available with `LLAMA_WRANGLER_SECRET_BACKEND=os_keychain`; encrypted fallback remains available and is still the default storage path. Secret-storage status reports platform/runtime metadata and warns for service-like runtimes until service-user keychain behavior is verified.
- The `service-dry-run` command prints review-only launchd service-wrapper artifacts with absolute user paths and encrypted fallback selected explicitly by default. OS keychain remains opt-in.
- Service-mode startup suppresses recovery-token output so launchd logs do not become a credential source.
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
- `consensus`: Ask bounded eligible Managed Nodes, compare non-streaming outputs locally, and return the deterministic majority or highest-ranked candidate.
- `consensus_delta`: Use the same local comparison and recommend unresolved-delta escalation in metadata; external escalation remains disabled until Frontier policy allows it.
- `frontier_delta`: Use frontier models only for minimized external review payloads.
- `local_only`: Never use external models.

## Companion Splunk app

The companion Splunk app ingests Llama Wrangler HEC events and provides an Overview plus a metadata-only Operations dashboard for request routing, model performance, endpoint health, consensus failure reasons, queue scheduling, streaming outcomes, benchmark scheduler/runner history, routing policy exclusions, model lifecycle actions, frontier usage, cost controls, and failure modes.

The clean install archive is [dist/llama_wrangler-0.2.0.tar.gz](dist/llama_wrangler-0.2.0.tar.gz). It contains one top-level `llama_wrangler/` directory and excludes `.DS_Store`, `__MACOSX`, and AppleDouble files. The operator has installed this package; full Splunk runtime acceptance remains in progress.

## Future Capabilities

The future direction is additive. V1 remains an Ollama-first fleet control plane, and current APIs/state should allow later capabilities without turning incomplete future plans into present product claims.

Potential V2 work includes:

- a generalized Capability Endpoint registry layered over the current Managed Node and Passive Endpoint model
- additional local inference runtimes such as LM Studio, MLX, and vLLM through explicit endpoint adapters
- richer subscriber-reported hardware, warm-model, load, benchmark, and lifecycle controls with capability-specific approval policy
- a separately designed bounded streaming-consensus protocol, only if payload privacy, cancellation, deterministic aggregation, and client compatibility can be proven
- Frontier Delta provider adapters with minimized payloads, redaction, cost limits, previews, and explicit operator approval
- signed cross-platform installers, service-account credential integrations, upgrade channels, and package integrity checks
- broader developer/build capability integrations only after V1, with narrow APIs, explicit trust boundaries, and no arbitrary server-side tool execution by default

See [docs/19_capability_endpoint_future_plan.md](docs/19_capability_endpoint_future_plan.md) for the V1/V2 boundary and sequencing rules.

## Screenshots

Screenshot placeholders live with the UI-first docs until packaging polish is complete.

## Troubleshooting

- If Ollama detection fails, confirm Ollama is running at `http://localhost:11434`, then run the setup scan again.
- If Splunk HEC returns 403, confirm the token is enabled and allowed to write to the configured index.
- If an IDE cannot connect, confirm it uses `http://localhost:11435/v1` for OpenAI-compatible clients or `http://localhost:11435` for Ollama-compatible clients.
