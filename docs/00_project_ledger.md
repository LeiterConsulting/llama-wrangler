# Llama Wrangler Project Ledger

Last updated: 2026-07-02T19:20:38Z

This ledger is the working memory for Llama Wrangler. Use it to preserve requirements, decisions, implementation status, risks, vulnerabilities, ideas, side quests, and next actions without losing the true product direction.

## Operating Rule

Before meaningful implementation work, update this ledger with:

- new requirements or scope changes
- decisions made
- completed work
- risks, vulnerabilities, or open questions discovered
- side quests accepted, deferred, or rejected
- verification performed

Side quests are allowed, but they must be recorded here and tied back to the main goals.

## North Star

Build Llama Wrangler as a local-first, friendly, installable Ollama fleet control plane with:

- embedded setup and operations UI
- marshal, subscriber, and standalone service modes
- OpenAI-compatible and Ollama-compatible endpoints
- stateful sessions and safe routing
- UI-driven node enrollment and model alias management
- local-only safe defaults
- metadata-only observability by default
- optional Splunk HEC telemetry
- optional Frontier Delta only after explicit configuration, policy checks, redaction, and approval controls

Do not drift into a headless config-only MVP.

## Binding Source Material

Primary product and architecture:

- `docs/01_product_scope.md`
- `docs/02_architecture.md`
- `docs/03_execution_modes.md`
- `docs/04_capability_model.md`
- `docs/05_frontier_delta.md`
- `docs/06_splunk_app_scope.md`
- `docs/07_event_schema.md`
- `docs/08_mvp_roadmap.md`

UI-first and hardening requirements:

- `codex_prompts/llama_wrangler_ui_first_additive_prompt.md`
- `codex_prompts/llama_wrangler_ui_second_additive_prompt.md`

Implementation prompts:

- `codex_prompts/01_build_mvp_service.md`
- `codex_prompts/02_add_consensus_mode.md`
- `codex_prompts/03_add_frontier_delta.md`
- `codex_prompts/04_build_splunk_app.md`
- `codex_prompts/05_build_demo.md`
- `codex_prompts/06_build_friendly_ui.md`

Config, schema, and Splunk assets:

- `configs/marshal.example.yaml`
- `configs/subscriber.example.yaml`
- `schemas/hec_events.schema.json`
- `splunk_app/`

## Non-Negotiable Requirements

- Normal users should not need to edit YAML or `.env` files.
- The first-run wizard is a core product surface, not later polish.
- The service should bind to localhost by default.
- LAN access requires explicit user enablement.
- Frontier providers are disabled by default.
- Prompt and response body logging is disabled by default.
- Splunk telemetry defaults to metadata-only.
- Secrets must be masked in the UI.
- Prefer OS keychain or credential storage when feasible.
- If keychain support is not yet implemented, use a clearly scoped local fallback and keep it separate from ordinary app state.
- Do not split a single model across machines.
- Do not replace Ollama.
- Do not require Kubernetes.
- Do not execute arbitrary agent tools server-side in the MVP.
- Do not silently retry streaming requests after partial output has been sent.
- Do not send data to frontier providers unless explicitly allowed by policy.

## Current Implementation State

Implemented:

- Go module and single-binary CLI foundation.
- Commands: `start`, `open`, `install`, `uninstall`, `version`, `marshal`, `subscriber`, `standalone`.
- YAML config loader with safe defaults.
- Platform app-data state store.
- Explicit state schema versioning and config versioning.
- Lightweight migration path for legacy unversioned state files.
- Migration history persisted in app state.
- Encrypted local fallback secret store for admin tokens, client API keys, and Splunk HEC tokens.
- Legacy plaintext `secrets.json` migration to encrypted `secrets.enc.json` with legacy file removal.
- Secret-storage backup/restore guidance for encrypted fallback file-key and env-key modes.
- OS keychain feasibility and integration plan documented for admin tokens, client API keys, HEC tokens, and future provider keys.
- Minimal additive opt-in OS keychain backend spike behind the existing secret-store API, enabled with `LLAMA_WRANGLER_SECRET_BACKEND=os_keychain`, with encrypted fallback retained.
- Service-wrapper dry-run harness for review-only macOS launchd artifacts and keychain validation commands.
- Managed Node and Passive Endpoint control-mode planning documented for future enrollment, routing, consensus, benchmark, safety, and UI-badge work.
- App-state schema version 2 for Managed Node versus Passive Endpoint metadata.
- Node metadata migration for control level, trust level, capability source, approval state, source fields, support flags, telemetry level, and freshness timestamps.
- Nodes UI badges for control level, trust level, approval state, and capability source.
- Embedded web UI at `/ui`.
- First-run setup shell.
- Dashboard, Nodes, Models, Splunk, IDE, and Audit UI surfaces.
- Local system and Ollama detection.
- OpenAI-compatible endpoints:
  - `GET /v1/models`
  - `POST /v1/chat/completions`
  - `POST /v1/completions`
  - `POST /v1/embeddings`
- Ollama-compatible endpoints:
  - `GET /api/tags`
  - `POST /api/chat`
  - `POST /api/generate`
- Subscriber endpoints:
  - `GET /subscriber/capabilities`
  - `POST /subscriber/proxy/*`
- Management endpoints for setup, config, nodes, models, aliases, routing policies, telemetry status, HEC testing, audit, and metrics.
- Basic queue guard for incoming inference requests.
- Runtime queue snapshot with active, waiting, available, current, recent, and priority metadata.
- Dashboard queue visibility for active/waiting capacity and current/recent queue entries.
- Weighted-priority queue scheduler with configurable high/normal/low dispatch weights and FIFO fallback policy.
- Dashboard queue scheduling controls backed by routing policy config.
- Support-bundle export with schema/config metadata, safe config, node/session metadata, queue snapshot, metadata-only audit events, and secret-storage status.
- Settings UI support-bundle download action.
- Settings UI secret-storage backup/restore guidance that keeps support bundles clearly diagnostic-only.
- Settings UI OS keychain status, platform/runtime context, service-mode warning state, fallback availability, and migrated count while encrypted fallback remains available.
- Metadata-only operation stats for upstream retries, partial responses, and client cancellations.
- Dashboard Streaming Outcomes card with retry, partial-response, and cancellation counts.
- Streaming proxy retry semantics that allow fallback only before client-visible output begins.
- Cancellation, partial-output, and upstream-retry telemetry for inference proxy requests.
- Session affinity foundation with `none`, `soft`, `strict`, and `task` semantics available for routing decisions.
- Basic routing by model alias, node eligibility, health, model inventory, and fallback nodes.
- Manual subscriber add flow through the UI and API.
- Local admin token generation.
- Management endpoint protection after setup completion.
- UI admin unlock flow using the generated local admin token.
- Generated client API keys for IDE/agent inference traffic.
- Runtime LAN exposure detection for non-loopback listen addresses.
- Settings UI network exposure warning and auth failure rate-limit metadata.
- In-memory auth failure rate limiting for invalid admin-token and client API-key attempts.
- Admin token rotation from Settings.
- Local browser logout for the embedded UI.
- Client API-key regenerate and revoke controls.
- Optional client API-key enforcement after setup completion.
- Structured telemetry sink with JSON logs, local audit events, and optional Splunk HEC.
- HEC client with sourcetype mapping.
- Splunk HEC UI settings including:
  - enabled
  - URL
  - token
  - index
  - source
  - sourcetype prefix
  - verify TLS certificates toggle for self-signed lab compatibility
  - explicit warning state when TLS certificate verification is disabled
- Frontier policy/redaction stub for obvious secret detection and local-only enforcement.
- Demo configs and test client script.
- Initial focused tests for config, routing, and HEC event generation/TLS behavior.
- Focused tests for post-setup management admin auth and client API-key auth.
- Focused tests for admin token rotation and client API-key revocation.
- Focused tests for state creation, legacy state migration, future schema rejection, and config-version increments.
- Focused tests for encrypted secret persistence, legacy plaintext migration/removal, delete/match behavior, and env-provided fallback keys.
- Focused tests for encrypted fallback backup/restore guidance metadata in file-key and env-key modes.
- Focused tests validating the OS keychain plan, default status metadata, opt-in keychain migration, unavailable-keychain fallback, service-like runtime warning metadata, and opt-in platform check wiring.
- Focused tests for launchd service-wrapper dry-run output, service-mode env, keychain opt-in env, plist escaping, and invalid `start --config` combinations.
- Focused tests documenting the Phase A closure credential decision.
- Focused tests documenting the Phase B Managed Node versus Passive Endpoint data model and UI-flow plan.
- Focused tests for state schema version 2 migration, passive endpoint metadata defaults, bootstrap node metadata, and support-bundle node metadata.
- Focused tests for retry-before-first-token, no-retry-after-partial-output, and cancellation telemetry.
- Focused tests for OpenAI SSE and Ollama JSONL streaming compatibility through the public marshal routes.
- Focused tests for queue snapshot shape, priority normalization, bootstrap queue metadata, and marshal proxy queue telemetry.
- Focused tests for weighted-priority queue scheduling, FIFO fallback dispatch, routing-policy persistence, and queue scheduling metadata.
- Focused tests for OpenAI/Ollama error shapes, client-auth errors, no-eligible-node errors, upstream failure sanitization, and upstream 4xx normalization.
- Focused tests for support-bundle shape, schema/config metadata, queue metadata, and redaction of secrets, token-like values, prompt bodies, response bodies, and payload-like fields.
- Focused tests for LAN exposure metadata and admin/OpenAI/Ollama auth failure rate-limit behavior.
- Focused tests for metadata-only operation stats in bootstrap and metrics.
- Focused tests for Splunk HEC TLS warning metadata and HEC token non-leakage.

Verified:

- `go test ./...` passes.
- `go build ./cmd/llama-wrangler` passes.
- `/healthz` returns `200`.
- `/ui/` returns `200`.
- Live bootstrap reports `schema_version: 1`, `config_version: 1`, and a migration history entry for the existing local state file.
- Live bootstrap reports encrypted secret storage metadata without returning secret values.
- Live bootstrap, auth status, and support-bundle export report encrypted fallback backup/restore guidance metadata without returning secret values.
- Live bootstrap, auth status, support-bundle export, and Settings report OS keychain status/platform/runtime metadata without returning secret values.
- In-app browser verified Settings shows state schema and config version with no console errors.
- Automated auth lifecycle tests verify:
  - admin-token rotation invalidates the old admin token
  - client API-key revocation invalidates inference access
- In-app browser verified:
  - Settings nav is visible
  - Settings page shows admin-token rotation and logout
  - IDE page shows client API-key generation and lifecycle controls
  - browser console has no errors after this slice
- Automated auth tests verify:
  - management config access returns `401` without admin token after setup completion
  - management config access returns `200` with admin token after setup completion
  - `/v1/models` returns `401` without a client API key after setup completion
  - `/v1/models` returns `200` with a generated client API key after setup completion
  - rotated admin tokens invalidate old admin tokens
  - revoked client API keys no longer authorize inference access
- In-app browser verified:
  - Splunk TLS checkbox is visible.
  - Splunk save/test controls are visible.
  - Nodes add-subscriber form is visible.
  - browser console has no errors.
- Local app-data verification confirms `secrets.enc.json` and `secrets.key` are permissioned `0600`, and legacy `secrets.json` is absent after migration.
- In-app browser verified Settings shows secret-storage status, encrypted status, state schema, and config version with no console errors and no visible admin-token leakage.
- Proxy retry tests verify:
  - fallback after upstream 5xx before first token
  - fallback after body-read failure before first token
  - no fallback after partial streaming output
  - cancellation telemetry before first token
- Streaming compatibility tests verify:
  - OpenAI-compatible clients can read `text/event-stream` `data:` lines before upstream completion
  - Ollama-compatible clients can read newline-delimited JSON objects before upstream completion
- Live bootstrap and metrics report queue capacity, active depth, waiting depth, current entries, recent entries, and priority options.
- In-app browser verified Dashboard queue visibility with no console errors.
- Live bootstrap and support-bundle export report queue scheduling policy and priority weights.
- In-app browser verified Dashboard queue scheduling controls with no console errors.
- Live `/v1/chat/completions` error probe returns OpenAI-style `error` object with safe code/type/message fields.
- Live `/api/chat` error probe returns Ollama-style string `error` plus safe code/type fields.
- Error compatibility tests verify payloads, upstream URLs, and token-like values are not echoed in client-facing errors.
- Live support-bundle export reports schema/config metadata, migration history, queue metadata, safe config, and encrypted secret-storage status without known secret or payload markers.
- In-app browser verified Settings shows the support-bundle export action with no console errors.
- Live bootstrap reports localhost as not LAN-exposed, requires explicit LAN enablement, and includes auth failure rate-limit metadata.
- In-app browser verified Settings shows Network Exposure, localhost-only posture, auth failure limit, and lockout window with no console errors.
- Live bootstrap and metrics report metadata-only operation stats for retries, partial responses, cancellations, audit window, and latest event timestamps when present.
- In-app browser verified Dashboard shows Streaming Outcomes with retry, partial-response, and cancellation counts, no visible local tokens, and no console errors.
- Live bootstrap and telemetry status report Splunk HEC TLS warning metadata when certificate verification is disabled.
- In-app browser verified the Splunk page shows an explicit TLS verification warning when disabled, clears it when restored, and has no console errors.
- In-app browser verified Settings shows encrypted fallback Backup & Restore guidance with required companion files and no console errors.
- In-app browser verified Settings shows OS keychain status with encrypted fallback still available and no console errors.
- Live bootstrap and auth status report default secret backend `encrypted_file`, fallback availability, OS keychain status `disabled`, and opt-in guidance without returning secret values.
- Live support-bundle export reports versioned schema metadata, secret-storage metadata only, and privacy flags that exclude secrets and payloads.
- In-app browser verified Settings shows OS keychain status, encrypted fallback availability, and opt-in guidance with no visible full admin token or client-token patterns.
- Opt-in macOS keychain test passed with `LLAMA_WRANGLER_RUN_KEYCHAIN_TESTS=1`, verifying real set/get/delete through the OS keychain for the current interactive user session.
- Live opt-in keychain/service-mode-marked run with `LLAMA_WRANGLER_SECRET_BACKEND=os_keychain LLAMA_WRANGLER_SERVICE_MODE=1` reported backend `os_keychain`, platform `darwin`, runtime `service_like`, service warning metadata, encrypted fallback availability, and no secret-marker leakage in bootstrap/auth/support-bundle responses.
- Default runtime was restored after opt-in verification and reports backend `encrypted_file`, platform `darwin`, runtime `interactive_user`, and no service warning.
- Service-wrapper dry-run command emits review-only launchd plist JSON, service-mode env, keychain opt-in env, validation commands, keychain check commands, and fallback warnings without secret-marker leakage.
- Disposable macOS launchd validation was exercised on port `11437` with temp HOME and disposable keychain service namespace.
- Disposable launchd validation proved service-like metadata and encrypted fallback behavior, but did not prove keychain availability: both disposable interactive and launchd runs reported `os_keychain_status: unavailable` and `backend: encrypted_file`.
- Normal interactive keychain test still passes with `LLAMA_WRANGLER_RUN_KEYCHAIN_TESTS=1`, so the keychain limitation appears tied to disposable/service-like launch context rather than the basic keychain backend.
- Phase A closure decision recorded: encrypted fallback is the supported service/default credential path; OS keychain remains interactive opt-in; service-keychain behavior moves to packaging hardening.
- Phase B planning began for Managed Node versus Passive Endpoint data model, UI flows, badges, trust metadata, and routing/safety hooks.
- Live `/healthz`, `/ui/`, bootstrap, and support-bundle export verified after Phase A closure documentation and Phase B planning.
- Live bootstrap still reports encrypted fallback secret storage, queue scheduling, operation stats, safe defaults, client presets, and no full admin/client token pattern.
- Live support-bundle export still reports versioned schema/privacy metadata and no token-shaped secret marker leakage.
- In-app browser verified Settings and Nodes render with no console errors, encrypted fallback guidance visible, no full admin/client token pattern visible, and the existing subscriber flow intact.
- Live bootstrap reports `schema_version: 2` after migrating the local app state, including the 1-to-2 migration history entry.
- Live bootstrap and support-bundle export report node control/trust/approval/source metadata while preserving encrypted fallback, queue scheduling, safe defaults, and support-bundle privacy.
- In-app browser verified Nodes shows Managed/Local/Approved badges and capability source with no console errors and no full admin/client token patterns.

## Active Service State

As of the latest run:

- Local service is running at `http://localhost:11435/ui/`.
- Runtime Splunk settings were restored to safe defaults after testing:
  - HEC disabled
  - URL empty
  - TLS verification enabled
- Ollama was detected as available during a later local scan.

## Decisions

- Use Go for the service and single-binary path.
- Use embedded static UI for the initial MVP rather than waiting for a React/Vite pipeline.
- Keep UI API JSON-shaped so a future React/Vite UI can replace the static shell cleanly.
- Treat UI-first and deep hardening prompts as binding scope modifiers.
- Start with localhost binding and local-only safety.
- Keep generated binary ignored via `.gitignore`.
- Add a ledger as the first numbered doc so future work starts with current project reality.
- Close Phase A with encrypted fallback as the supported service/default credential path. Keep OS keychain interactive opt-in and move service-keychain guarantees to packaging/install hardening.

## Main Roadmap

### Phase A: Foundation Hardening

Status: complete as of 2026-07-02T19:09:53Z.

- Done: expand state schema for durable config versions and migrations.
- Done: add local admin token generation.
- Done: protect management endpoints after setup completion.
- Done: add API key generation and validation for IDE/agent clients.
- Done: improve secrets storage with an explicit encrypted fallback and legacy plaintext migration.
- Done: add streaming cancellation and retry semantics.
- Done: add queue priority metadata and queue visibility in the UI.
- Done: add structured error response compatibility for OpenAI and Ollama clients.
- Done: add support-bundle export that includes schema/config metadata while excluding secrets and payloads.
- Done: add explicit LAN enablement warning and auth failure rate limiting.
- Done: add real-client streaming/SSE compatibility checks.
- Done: add queue scheduling policy controls that use priority metadata for weighted dispatch.
- Done: add minimal additive opt-in OS keychain backend spike behind the secret-store API while keeping encrypted fallback available.
- Done: run opt-in macOS interactive keychain check and add service-like runtime warning/status metadata.
- Done: add minimal macOS launchd service-wrapper dry-run plan/harness without installing or mutating OS service state.
- Done: manually exercise disposable macOS launchd validation path and confirm service-like metadata plus encrypted fallback behavior.
- Done: record explicit Phase A closure decision with encrypted fallback as the service/default credential path and OS keychain as interactive opt-in plus packaging hardening.
- Later packaging hardening: real macOS launchd install/uninstall helpers, signing/notarization impact on keychain prompts, systemd service credential checks, and Windows service credential checks.

### Phase B: Node Enrollment and Discovery

Status: active planning and next implementation phase.

- Done: implement Managed Node versus Passive Endpoint data model foundation with control/trust/source/approval/freshness metadata.
- Done: add schema version 2 migration preserving existing manually added subscribers as Managed Node records.
- Done: add initial UI badges for control level, trust level, approval state, and capability source.
- Add Passive Endpoint add flow with safe validation and explicit limitations.
- Expand Managed Node install/enroll flow with approval state.
- Complete manual subscriber enrollment with approval state.
- Add enrollment token flow.
- Add subscriber registration heartbeat to marshal.
- Add mDNS discovery if feasible.
- Keep subnet scanning opt-in only.
- Show enrollment warnings and remediation in the UI.

### Phase C: Routing, Sessions, and Model State

Status: started.

- Track per-node/model state:
  - installed
  - loading
  - warm
  - busy
  - unloaded
  - evicted
  - failed
- Implement warm-model preference.
- Add keep-warm UI toggles.
- Persist session lineage and request chains.
- Add per-client and per-alias affinity settings.
- Improve fallback behavior for streaming versus non-streaming requests.

### Phase D: Benchmark and Evaluation Harness

Status: not started.

- Add benchmark tasks:
  - JSON extraction
  - summarization
  - regex with test cases
  - Python syntax/unit-test generation
  - simple code explanation
  - SPL review
  - task classification
  - customer-facing rewrite
- Persist results per node/model/task type.
- Feed benchmark results into routing recommendations.

### Phase E: Consensus

Status: foundation only.

- Implement participant selection.
- Add fan-out response collection with timeouts.
- Add deterministic validators:
  - JSON parse
  - YAML parse
  - regex tests
  - Python syntax checks
  - unit tests
  - markdown/frontmatter validation
  - shellcheck where available
  - schema validation
- Add agreement scoring.
- Emit consensus telemetry.
- Return consensus metadata only in debug mode.

### Phase F: Frontier Delta

Status: policy stub only.

- Add provider abstraction.
- Add OpenAI-compatible provider.
- Build minimized delta payloads.
- Add approval workflow and payload preview.
- Improve redaction:
  - API keys
  - bearer tokens
  - private keys
  - passwords
  - `.env` values
  - AWS/GitHub/Splunk tokens
  - hostnames
  - customer identifiers
  - email addresses
  - source code if policy forbids sharing
- Emit Frontier Delta telemetry.

### Phase G: Splunk App

Status: starter app exists.

- Add dashboards beyond overview:
  - Node Health
  - Model Performance
  - Routing Decisions
  - Consensus and Frontier Delta
  - Client/IDE Usage
  - Frontier Usage
- Add alerts for failures, queue pressure, policy denials, and HEC delivery failures.
- Add sample events for all sourcetypes.

### Phase H: Install and Packaging

Status: command stubs only.

- Add real service install/start helpers per OS.
- Add launch-at-login/background service support.
- Add uninstall path.
- Add signed/notarized packaging later if needed.

## Risks and Potential Vulnerabilities

### R1: Management API Authentication Needs More Hardening

Risk: management endpoints are now protected after setup completion, but the model is still a single local admin token and has not yet been expanded into rotation, expiry, or OS keychain-backed storage.

Current mitigation:

- localhost bind by default
- admin token generated locally
- admin token required for management endpoints after setup completion
- bearer-token auth avoids cookie CSRF
- non-loopback listen addresses produce explicit UI/API warnings
- repeated invalid presented admin and client tokens are rate limited per remote address

Required mitigation:

- done: add token rotation/regeneration UI
- done: add explicit LAN enablement warning
- partially done: store admin token in OS keychain where practical through the opt-in backend spike; macOS interactive check passed, true service-wrapper checks remain
- done: add logout/clear-token UI
- done: add rate limiting for auth failures

Priority: medium-high.

### R2: Encrypted Local Secrets Store Is a Fallback, Not OS Keychain

Risk: secrets are no longer stored in plaintext `secrets.json`, but the fallback still uses a local encrypted file and local key file rather than OS-backed secure storage.

Current mitigation:

- secrets are separate from ordinary app state
- API responses return only storage status and token hints, not secret values
- fallback storage writes AES-GCM encrypted `secrets.enc.json`
- fallback key file `secrets.key` is permissioned `0600`
- `LLAMA_WRANGLER_SECRETS_KEY` can provide a 32-byte base64 key instead of the local key file
- legacy plaintext `secrets.json` is migrated and removed
- Settings, bootstrap, auth status, and support bundles describe backup/restore requirements without including secret values
- support-bundle docs explicitly state that support bundles are diagnostic metadata, not backup/restore artifacts
- OS keychain feasibility plan defines non-destructive migration, platform risks, backend contract, and acceptance criteria
- optional OS keychain backend spike can be enabled with `LLAMA_WRANGLER_SECRET_BACKEND=os_keychain`
- encrypted fallback remains available and becomes active automatically if the keychain backend is unavailable
- macOS interactive opt-in keychain set/get/delete passed for the current user session
- service-like runtime metadata and warnings are surfaced when `LLAMA_WRANGLER_SERVICE_MODE=1` or common service markers are present
- launchd service-wrapper dry-run harness emits review-only plist and validation commands without installing services or exposing secrets
- disposable launchd validation confirmed fallback works in service-like runtime when OS keychain is unavailable
- Phase A closure decision recorded: encrypted fallback is the supported service/default credential path; OS keychain remains interactive opt-in; service-keychain behavior moves to packaging hardening

Required mitigation:

- done: add a minimal additive OS keychain backend spike behind the secret-store API
- done: run opt-in macOS interactive keychain check and add service-like runtime warning metadata
- done: add launchd dry-run harness and documentation for service-wrapper validation
- done: manually validate disposable launchd path; keychain was unavailable but encrypted fallback and service-like warning metadata worked
- done: record final Phase A decision: encrypted fallback as supported service default, OS keychain as interactive opt-in, and service keychain as packaging hardening
- planned: run real systemd/Windows-service keychain checks later when those service wrappers exist
- done: add fallback key rotation/rekey workflow
- done: document backup/restore behavior for encrypted fallback secrets

Priority: high.

### R3: HEC TLS Verification Can Be Disabled

Risk: disabling TLS verification enables trusted self-signed lab compatibility but weakens transport verification.

Current mitigation:

- default is `verify_ssl: true`
- UI copy says to disable only for trusted self-signed lab certificates
- bootstrap and telemetry status expose metadata-only TLS warning state when certificate verification is disabled
- Splunk UI shows an explicit warning when certificate verification is disabled

Required mitigation:

- done: add warning state in UI when disabled
- consider certificate pinning/import later

Priority: medium.

### R4: Streaming Semantics Need Hardening

Risk: retry-before-first-token versus no-retry-after-partial-output semantics are now explicit for marshal inference proxying, but broader compatibility work still needs named client matrix coverage.

Current mitigation:

- marshal does not write streaming response headers until upstream body bytes are available
- upstream connection errors, 5xx responses, and body-read failures can fallback before first token
- partial streamed output disables further fallback for that request
- non-streaming upstream responses are buffered before client commit
- cancellation, retry, and partial-output telemetry are metadata-only
- public-route compatibility tests cover OpenAI SSE and Ollama JSONL first-chunk streaming behavior
- operations UI surfaces metadata-only retry, partial-response, and cancellation counts

Required mitigation:

- done: add real-client streaming compatibility checks
- done: add OpenAI SSE and Ollama JSONL shape tests
- done: surface partial/cancel/retry counts in operations UI

Priority: high.

### R5: OpenAI Compatibility Is Incomplete

Risk: clients may expect exact error formats, usage fields, SSE shape, tool-call pass-through, JSON mode, stop tokens, and model listing behavior.

Required mitigation:

- create compatibility test matrix
- add client presets
- normalize errors and streaming formats

Priority: medium.

### R6: Manual Node Enrollment Trust Model Is Incomplete

Risk: manually added nodes are approved immediately.

Current mitigation:

- schema version 2 records control level, trust level, capability source, approval state, and metadata freshness
- existing manually added subscribers migrate as Managed Node records with explicit approval state
- Nodes UI shows control/trust/approval/source badges

Required mitigation:

- done: add node control/trust metadata foundation
- enrollment token flow
- marshal approval queue
- node identity persistence
- signed heartbeat or shared trust token

Priority: high.

### R7: Frontier Policy Stub Is Not Sufficient For Real External Calls

Risk: redaction and approval controls are incomplete.

Required mitigation:

- block all provider calls until policy, redaction, preview, and approval are real
- add tests for secrets and source-code policy

Priority: high.

## Open Questions

- Which auth model should the UI use first: admin bearer token, local password, or loopback-only bootstrap token?
- Should the static UI remain for MVP, or should React/Vite begin now?
- Should the first OS keychain implementation use `github.com/zalando/go-keyring` as recommended, or keep `github.com/99designs/keyring` available for broader Linux backend selection?
- Should subscriber registration be pull-based from marshal, push-based from subscriber, or both?
- What exact clients should define the first compatibility matrix: Cline, Continue, Open WebUI, generic OpenAI SDK, Ollama CLI?
- Should Splunk HEC token storage support both env var and UI-managed secret simultaneously?

## Ideas and Suggestions

- Add a visible “Local Only Mode” switch in the header.
- Add explicit support-bundle schema/version documentation for downstream support tooling.
- Add guided “copy endpoint” cards for Cline, Continue, Open WebUI, and generic OpenAI SDK.
- Add a simple request replay simulator that uses metadata only.
- Add model placement recommendations after benchmarks.
- Add “why this node?” routing explanations in request history.
- Add certificate warning UI when Splunk TLS verification is disabled.
- Add UI badges for prompt logging mode and frontier posture.
- Add a ledger update checklist to PR descriptions later.

## Side Quests

Accepted:

- Add Splunk HEC `verify_ssl`/TLS verification toggle for self-signed cert compatibility.
- Add manual subscriber enrollment form while working on UI foundation.
- Add Managed Node and Passive Endpoint modes as a Phase B/C planning expansion.

Deferred:

- Full React/Vite UI pipeline.
- OS-native service install.
- Keychain-only mode or destructive fallback cleanup beyond the minimal additive backend spike.
- mDNS discovery.
- Full Splunk dashboard suite.

Rejected for MVP:

- Model-parallel distributed inference.
- Server-side arbitrary tool execution.
- Cloud/Kubernetes requirement.
- Default frontier escalation.

## Latest Update Log

### 2026-07-01T19:44:45Z

- Verified token lifecycle slice with `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, `/ui/`, served UI asset checks, and in-app browser DOM checks.
- Confirmed Settings page exposes admin token rotation and local browser logout.
- Confirmed IDE page exposes client API-key generation and lifecycle controls.
- Confirmed browser console has no errors.

### 2026-07-01T19:50:34Z

- Continuing Phase A: Foundation Hardening.
- Active work item: add explicit state schema versioning and lightweight migrations for persisted app data.
- Goal: make existing and future app-data files upgrade safely without manual edits, orphaned fields, or silently missing defaults.
- Guardrail: do not mark Phase A complete; this slice only covers state/config versioning and migration tests.

### 2026-07-01T19:53:50Z

- Added `schema_version`, `config_version`, and `migration_history` to persisted app state.
- Added migration path from legacy unversioned state to schema version 1.
- Added config normalization reuse between YAML loading and state migration.
- Added bootstrap/auth status metadata for schema and config versions.
- Added Settings UI display for state schema and config version.
- Added tests for new state creation, legacy migration, future schema rejection, and config-version increments.
- Verified `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, `/ui/`, live bootstrap schema metadata, and in-app browser Settings metadata.

### 2026-07-01T19:55:18Z

- Continuing Phase A: Foundation Hardening.
- Active work item: improve local secret storage toward OS keychain or an explicit encrypted fallback.
- Goal: keep secrets out of ordinary app state and API responses while replacing plaintext `secrets.json` fallback storage.
- Guardrail: do not start full OS keychain integration in this slice; implement and document encrypted fallback storage with legacy plaintext migration.

### 2026-07-01T19:59:43Z

- Added AES-GCM encrypted fallback secret storage in `secrets.enc.json`.
- Added local fallback key handling through permissioned `secrets.key` or `LLAMA_WRANGLER_SECRETS_KEY`.
- Added legacy plaintext `secrets.json` migration with legacy file removal after successful encryption.
- Exposed secret-storage status metadata through bootstrap/auth status and Settings UI without exposing secret values.
- Added focused secret-store tests for plaintext exclusion, legacy migration/removal, delete/match behavior, and env-provided keys.
- Verified `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, `/ui/`, live bootstrap secret-storage metadata, local encrypted fallback file permissions, and in-app browser Settings rendering with no console errors.

### 2026-07-01T20:03:23Z

- Continuing Phase A: Foundation Hardening.
- Active work item: add streaming cancellation and retry semantics.
- Goal: make retry-before-first-token versus no-retry-after-partial-output explicit for upstream proxy requests.
- Guardrail: preserve state schema/migration behavior and encrypted secret storage; emit cancellation, partial-output, and retry telemetry without logging prompt or response bodies.

### 2026-07-01T20:08:17Z

- Added controlled marshal proxy copying so streaming responses are not committed to the client until upstream produces the first body bytes.
- Added fallback retry before first token for upstream request errors, upstream 5xx responses, and upstream body-read failures.
- Added no-retry-after-partial-output handling with `response_partial` telemetry.
- Added cancellation telemetry for client cancellation before first token and after partial output.
- Buffered non-streaming upstream responses before client commit so read failures can still fall back safely.
- Added focused proxy tests for retry-before-first-token, no-retry-after-partial-output, and cancellation telemetry.
- Documented streaming/retry semantics in architecture docs, UI/API docs, README, and this ledger.
- Verified `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, `/ui/`, live bootstrap schema/secret metadata, `/v1/models`, and in-app browser UI/settings checks with no console errors.

### 2026-07-01T22:04:14Z

- Continuing Phase A: Foundation Hardening.
- Active work item: add queue priority metadata and queue visibility in the UI.
- Goal: expose queue depth, capacity, active work, recent queue outcomes, and request priority as metadata-only operational state.
- Guardrail: do not change persisted app-state schema; preserve encrypted secret storage and streaming retry semantics; do not log prompt or response bodies.

### 2026-07-01T22:08:45Z

- Added in-memory runtime queue metadata with active, waiting, available, current, recent, and priority fields.
- Added queue priority metadata parsing from `X-Llama-Wrangler-Priority`, `priority`, and `queue_priority`.
- Added queue snapshots to bootstrap and metrics responses while preserving the existing `queue_depth` field.
- Added metadata-only queue telemetry for waiting, active, completed, cancelled, rejected, and full-queue outcomes.
- Added Dashboard queue visibility with capacity meter, active/waiting/available counts, and current/recent queue entries.
- Added focused queue tests for snapshot shape, priority normalization, bootstrap API shape, and marshal proxy queue telemetry.
- Documented queue visibility and priority metadata in architecture docs, UI/API docs, README, and this ledger.
- Verified `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, `/ui/`, live bootstrap queue metadata, metrics queue metadata, and in-app browser Dashboard queue rendering with no console errors.

### 2026-07-01T22:18:13Z

- Continuing Phase A: Foundation Hardening.
- Active work item: add structured OpenAI/Ollama error compatibility tests and normalized inference error shapes.
- Goal: make `/v1/*` errors OpenAI-compatible and `/api/*` errors Ollama-compatible while keeping management/UI errors stable.
- Guardrail: preserve state schema/migrations, encrypted secret storage, streaming retry semantics, and queue metadata; do not echo prompt bodies, response bodies, raw headers, API keys, HEC tokens, upstream URLs, or other secrets in error responses.

### 2026-07-01T22:23:11Z

- Added inference error compatibility helper for OpenAI-style `/v1/*` errors and Ollama-style `/api/*` errors.
- Updated client API-key failures, no-eligible-node failures, queue-full failures, upstream unavailable failures, and upstream 4xx failures to use normalized inference error shapes.
- Preserved management/UI JSON error behavior for the embedded web app.
- Added upstream 4xx normalization so raw Ollama error bodies are not copied through when they may contain payload or environment details.
- Added focused tests for OpenAI auth error shape, Ollama auth error shape, no-eligible-node shapes, upstream transport sanitization, and upstream 4xx normalization.
- Documented error compatibility and sanitization rules in architecture docs, UI/API docs, README, and this ledger.
- Verified `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, `/ui/`, live bootstrap schema/secret/queue metadata, live `/v1/chat/completions` and `/api/chat` error shapes, and in-app browser Dashboard rendering with no console errors.

### 2026-07-01T22:34:18Z

- Continuing Phase A: Foundation Hardening.
- Active work item: add a support-bundle export with schema/config metadata and sanitized operational state.
- Goal: provide a troubleshooting artifact that includes schema/config versions, safe config, nodes, queue metadata, migration history, and metadata-only audit events.
- Guardrail: do not change persisted app-state schema; preserve encrypted secret storage, streaming retry semantics, queue metadata behavior, and normalized OpenAI/Ollama error shapes; exclude secrets, API keys, auth headers, prompt bodies, response bodies, request bodies, and payload-like fields.

### 2026-07-01T22:39:01Z

- Added `POST /wrangler/support-bundle/export` for sanitized troubleshooting exports.
- Included schema/config versions, migration history, safe config metadata, node/session metadata, queue snapshot, metadata-only audit events, and encrypted secret-storage status.
- Added recursive sanitization for support-export maps and slices, including prompt, message, response, payload, header, token, API-key, password, and secret-like fields.
- Added Settings UI support-bundle download action without requiring YAML or manual file assembly.
- Added focused tests for support-bundle metadata shape and secret/payload redaction.
- Documented support-bundle scope and exclusions in architecture docs, UI/API docs, README, and this ledger.
- Verified `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, `/ui/`, live support-bundle metadata/redaction checks, and in-app browser Settings rendering with no console errors.

### 2026-07-01T22:43:25Z

- Continuing Phase A: Foundation Hardening.
- Active work item: add explicit LAN enablement warning and auth failure rate limiting.
- Goal: make non-localhost listen addresses visibly risky in the UI/API while slowing repeated failed admin and client API-key attempts.
- Guardrail: do not mark Phase A complete; preserve state schema/migration, encrypted secret storage, streaming retry semantics, queue metadata behavior, normalized OpenAI/Ollama error shapes, and sanitized support-bundle export behavior.

### 2026-07-01T22:49:05Z

- Added runtime LAN exposure detection from the configured listen address.
- Added bootstrap `safe_defaults` metadata for LAN exposure status, warning text, localhost default posture, and explicit enablement requirement.
- Added Settings Network Exposure card with localhost-only versus LAN-exposed posture.
- Added in-memory auth failure rate limiting for repeated invalid admin-token and client API-key attempts, keyed by remote address and auth scope.
- Preserved valid-token pass-through after failed attempts so a real admin/client can recover from typos without waiting for the limiter.
- Added `Retry-After` on rate-limited auth failures.
- Kept management rate-limit errors UI-friendly while preserving OpenAI/Ollama-compatible inference error shapes.
- Added focused tests for LAN exposure bootstrap metadata, admin auth rate limiting, and OpenAI/Ollama client auth rate-limit shapes.
- Documented LAN warning and auth rate-limit behavior in architecture docs, UI/API docs, first-run wizard docs, README, and this ledger.
- Verified `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, `/ui/`, live bootstrap LAN/rate-limit metadata, live support-bundle redaction preservation, and in-app browser Settings rendering with no console errors.

### 2026-07-01T23:06:55Z

- Continuing Phase A: Foundation Hardening.
- Active work item: add real-client streaming/SSE compatibility checks.
- Goal: verify SDK-style clients can consume OpenAI SSE and Ollama streaming JSONL through the marshal proxy while preserving first-token retry and no-retry-after-partial-output semantics.
- Guardrail: do not mark Phase A complete; preserve state schema/migration, encrypted secret storage, queue metadata, normalized OpenAI/Ollama errors, sanitized support-bundle export, LAN warnings, and auth failure rate limiting.

### 2026-07-01T23:10:03Z

- Added route-level streaming compatibility tests using real `net/http` clients and `httptest` upstream subscribers.
- Verified OpenAI-compatible `/v1/chat/completions` streaming preserves `text/event-stream` framing and lets clients read `data:` lines before upstream completion.
- Verified Ollama-compatible `/api/chat` streaming preserves newline-delimited JSON framing and lets clients read JSON object lines before upstream completion.
- Preserved existing first-token retry and no-retry-after-partial-output behavior without production proxy churn.
- Documented OpenAI SSE and Ollama JSONL streaming contracts in architecture docs, UI/API docs, README, and this ledger.
- Verified `go test ./internal/httpapi`, `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, `/ui/`, live bootstrap/schema/queue/secret/auth/LAN metadata, live support-bundle redaction preservation, and in-app browser UI rendering with no console errors.

### 2026-07-01T23:12:34Z

- Continuing Phase A: Foundation Hardening.
- Active work item: add queue scheduling policy controls that use priority metadata for weighted dispatch.
- Goal: move priority from metadata-only visibility to a lightweight configurable dispatch policy while preserving existing queue visibility, telemetry, and streaming behavior.
- Guardrail: do not mark Phase A complete; preserve state schema/migration, encrypted secret storage, streaming retry semantics and compatibility checks, normalized errors, sanitized support bundle, LAN warnings, and auth failure rate limiting.

### 2026-07-01T23:18:49Z

- Added routing config fields for `queue_scheduling_policy` and `queue_priority_weights`.
- Added default `weighted_priority` scheduling with high/normal/low weights of 3/2/1 and a `fifo` fallback policy.
- Replaced the metadata-only queue semaphore with a lightweight scheduler that dispatches waiting requests by configured policy while preserving active/waiting/recent queue tracking.
- Added queue scheduling metadata to bootstrap, metrics, and support-bundle queue snapshots.
- Added Dashboard queue scheduling controls that save through `PUT /wrangler/routing/policies`.
- Added queue scheduling policy examples to `configs/marshal.example.yaml`.
- Added focused tests for weighted-priority dispatch, FIFO dispatch, queue scheduling metadata, routing-policy persistence, and config defaults.
- Documented weighted-priority scheduling in architecture docs, UI/API docs, README, and this ledger.
- Verified `go test ./internal/config ./internal/httpapi`, `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, `/ui/`, live bootstrap/routing/support-bundle queue scheduling metadata, support-bundle redaction preservation, and in-app browser Dashboard controls with no console errors.

### 2026-07-01T19:41:28Z Completion

- Added admin-token rotation endpoint and Settings UI.
- Added browser-local logout action.
- Added client API-key regenerate and revoke endpoints.
- Added IDE-page controls for client-key lifecycle.
- Added tests that prove admin rotation invalidates old admin tokens and client-key revoke invalidates inference access.

### 2026-07-01T19:41:28Z

- Continuing Phase A: Foundation Hardening.
- Active work item: add token rotation/logout UI and client API-key lifecycle controls.
- Goal: reduce R1 further by allowing admin-token rotation, local UI logout, and client-key revoke/regenerate without expanding into full RBAC.
- Side quest guardrail: do not start OS keychain integration in this slice; keep fallback secret storage documented as a remaining risk.

### 2026-07-01T19:20:53Z

- Added local admin token generation and startup recovery-token handoff.
- Added post-setup management endpoint protection.
- Added generated client API keys for IDE/agent inference endpoints.
- Added UI unlock flow and IDE API-key generation surface.
- Added auth status and API-key creation endpoints.
- Added tests for management auth and inference client-key enforcement.
- Verified `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, and `/ui/`.

### 2026-07-01T19:15:24Z

- Continuing platform development from Phase A: Foundation Hardening.
- Active work item: add setup/admin auth for management endpoints and generated client API keys for IDE/agent inference endpoints.
- Goal: reduce R1 by protecting management APIs after setup completion while preserving first-run usability on localhost.
- Side quest guardrail: do not expand into full RBAC or OS keychain integration in this slice.

### 2026-07-01T19:12:49Z

- Created this ledger.
- Recorded source material, binding requirements, implementation state, risks, roadmap, and side quests.

### 2026-07-01 Earlier Session

- Added Go service foundation.
- Added embedded UI.
- Added app state store.
- Added local capability detection.
- Added OpenAI/Ollama-compatible routes.
- Added subscriber proxy routes.
- Added JSON audit/telemetry foundation.
- Added Splunk HEC client.
- Added HEC TLS verification support and UI control.
- Added manual subscriber add flow.
- Added demo configs and docs.
- Verified tests, build, UI availability, and in-app browser UI state.

### 2026-07-02T04:27:14Z

- Continuing Phase A: Foundation Hardening.
- Active work item: add client preset cards for Cline, Continue, Open WebUI, and generic OpenAI SDK.
- Goal: make common local clients easier to configure from the UI while deriving endpoint/model values from live runtime metadata.
- Guardrail: do not mark Phase A complete; preserve state schema/migration, encrypted secret storage, streaming retry semantics and compatibility checks, queue scheduling and visibility, normalized OpenAI/Ollama error shapes, sanitized support-bundle export behavior, LAN exposure warnings, and auth failure rate limiting. Preset metadata must not expose stored API keys, admin tokens, HEC tokens, prompts, responses, headers, or payloads.

### 2026-07-02T04:33:41Z

- Added sanitized client preset metadata to bootstrap and `GET /wrangler/client-presets`.
- Added IDE setup preset cards for Cline, Continue, Open WebUI, and generic OpenAI SDK clients.
- Presets derive the OpenAI-compatible base URL from the current request host and select configured aliases, preferring `local-code` for coding clients and `local-fast` for general clients when available.
- Preset API responses use `<client-api-key>` placeholders and do not return stored client API keys, admin tokens, HEC tokens, prompts, responses, headers, or payloads.
- UI copy snippets substitute the browser-local one-time client key only when the browser already has it from setup/key generation.
- Added focused tests for bootstrap preset shape, literal placeholder preservation, host/scheme derivation, alias fallback, and no client-key leakage.
- Documented client presets in UI API docs, first-run wizard docs, README, and this ledger.
- Verified `go test ./internal/httpapi`, `go test ./internal/config`, `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, live bootstrap preset/secret/queue metadata, live client-presets placeholder behavior, live support-bundle redaction preservation, and in-app browser IDE preset rendering with no console errors.

### 2026-07-02T13:29:12Z

- Received a duplicate request for the already completed client preset card slice.
- Audit scope: confirm the existing implementation still satisfies the Cline, Continue, Open WebUI, and generic OpenAI SDK preset-card requirements without creating duplicate code or orphan UI/API surfaces.
- Guardrail: do not mark Phase A complete, do not re-open completed status, and do not invent new implementation work unless verification finds a gap.

### 2026-07-02T13:30:15Z

- Duplicate-request audit complete: no missing client preset work found and no implementation changes were needed.
- Reverified preset code/docs/API coverage for Cline, Continue, Open WebUI, and generic OpenAI SDK.
- Reverified `go test ./internal/httpapi`, `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, live bootstrap/client-presets placeholder behavior, queue scheduling metadata, encrypted secret-storage metadata, support-bundle privacy flags, LAN safe-default metadata, auth rate-limit metadata, and in-app browser IDE preset rendering with no console errors.
- Preserved the next recommended work list with support-bundle schema/version documentation as the next Phase A slice.

### 2026-07-02T13:47:53Z

- Received another duplicate request for the already completed client preset card slice.
- Audit scope: reverify the existing client preset implementation and avoid duplicating code, docs, tests, routes, or UI surfaces unless a regression is found.
- Guardrail: keep Phase A open and preserve the next recommended Phase A slice as support-bundle schema/version documentation.

### 2026-07-02T13:48:47Z

- Repeat duplicate-request audit complete: no missing client preset work found and no implementation changes were needed.
- Reverified code, docs, tests, live bootstrap, live `GET /wrangler/client-presets`, support-bundle privacy flags, queue scheduling metadata, encrypted secret-storage metadata, LAN safe-default metadata, auth rate-limit metadata, and in-app browser IDE preset rendering.
- Confirmed the IDE view still shows Cline, Continue, Open WebUI, and OpenAI SDK preset cards with four copy buttons, placeholder API-key text, no visible `lw_client_` token material, and no console errors.
- Preserved the next recommended work list with support-bundle schema/version documentation as the next Phase A slice.

### 2026-07-02T13:55:07Z

- Continuing Phase A: Foundation Hardening.
- Active work item: add explicit support-bundle schema/version documentation for downstream support tooling.
- Goal: make support-bundle exports self-describing and document a stable schema contract for diagnostics tooling.
- Guardrail: do not mark Phase A complete; preserve state schema/migration, encrypted secret storage, streaming retry semantics and compatibility checks, queue scheduling and visibility, normalized OpenAI/Ollama error shapes, sanitized support-bundle export behavior, LAN exposure warnings, auth failure rate limiting, and client preset behavior. Do not add secrets, prompt bodies, response bodies, request bodies, headers, or payloads to support bundles.

### 2026-07-02T13:59:05Z

- Added support-bundle schema metadata to exports under `bundle_schema` with name, version, JSON Schema path, documentation path, and compatibility policy.
- Added `schemas/support_bundle.schema.json` as the machine-readable support-bundle contract for downstream tooling.
- Added `docs/13_support_bundle_schema.md` documenting schema identity, version separation, additive compatibility rules, required top-level fields, privacy guarantees, and consumer guidance.
- Updated architecture docs, UI API docs, and README to reference the versioned support-bundle schema contract.
- Added focused tests tying runtime `bundle_schema` metadata to the JSON Schema and documentation artifacts, while preserving redaction and queue/schema/config metadata tests.
- Verified `jq empty schemas/support_bundle.schema.json`, `go test ./internal/httpapi -run 'TestSupportBundle'`, `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, live support-bundle schema/privacy/queue metadata, live bootstrap schema/secret/queue/LAN/auth/client-preset metadata, live `GET /wrangler/client-presets`, and in-app browser Settings rendering with no console errors.

### 2026-07-02T13:59:54Z

- Continuing Phase A: Foundation Hardening.
- Active work item: add fallback key rotation/rekey workflow for encrypted local secrets.
- Goal: allow the local encrypted fallback secrets key to be rotated without exposing stored secret values or changing ordinary app state.
- Guardrail: do not mark Phase A complete; preserve state schema/migration, encrypted secret storage behavior, streaming retry semantics and compatibility checks, queue scheduling and visibility, normalized OpenAI/Ollama error shapes, sanitized and versioned support-bundle export behavior, LAN exposure warnings, auth failure rate limiting, and client preset behavior.

### 2026-07-02T14:05:40Z

- Added encrypted local fallback rekey support for file-backed secret storage.
- Added secret-storage status metadata for `rekey_supported` and a metadata-only rekey description.
- Added `POST /wrangler/secrets/rekey` as an admin-protected endpoint that rotates the local fallback key, rewrites `secrets.enc.json`, and returns metadata only.
- Env-sourced secret keys such as `LLAMA_WRANGLER_SECRETS_KEY` now return `409 secret_rekey_unsupported` for local rekey attempts.
- Added Settings UI Secret Storage card with backend, encrypted state, key source, local rekey availability, and Rotate fallback key action.
- Added focused tests proving file-key rekey changes key/ciphertext, preserves secrets after reopen, rejects env-key rekey, requires admin after setup, avoids token leakage, and preserves admin/client credentials after rekey.
- Documented rekey behavior in UI API docs, configuration storage docs, architecture docs, first-run wizard docs, README, and this ledger.
- Verified `go test ./internal/secrets`, `go test ./internal/httpapi -run 'TestSecretRekey|TestManagementAuth|TestClientAPIKey'`, `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, live bootstrap/auth status secret-storage metadata, live `POST /wrangler/secrets/rekey`, post-rekey support-bundle privacy/schema/queue metadata, live client-presets placeholder behavior, `0600` local secret file permissions, restart after rekey, and in-app browser Settings rendering with no console errors.

### 2026-07-02T14:07:03Z

- Continuing Phase A: Foundation Hardening.
- Active work item: surface partial/cancel/retry counts in the operations UI.
- Goal: summarize existing metadata-only `upstream_retry`, `response_partial`, and `request_cancelled` audit events for operator visibility.
- Guardrail: do not mark Phase A complete; preserve state schema/migration, encrypted secret storage and rekey behavior, streaming retry semantics and compatibility checks, queue scheduling and visibility, normalized OpenAI/Ollama error shapes, sanitized and versioned support-bundle export behavior, LAN exposure warnings, auth failure rate limiting, and client preset behavior. Do not expose prompts, responses, request bodies, raw headers, API keys, HEC tokens, or secret values in operations counters.

### 2026-07-02T14:12:08Z

- Added `operation_stats` metadata to bootstrap and metrics responses.
- Summarized existing `upstream_retry`, `response_partial`, and `request_cancelled` audit events into retry, partial-response, and cancellation counters.
- Added Dashboard Streaming Outcomes card with retry, partial-response, cancellation, no-retry-after-partial, cancel-after-partial, and latest-event visibility.
- Kept operation stats metadata-only and excluded prompts, responses, request bodies, raw headers, API keys, HEC tokens, admin tokens, client tokens, and secret values.
- Added focused tests for operation-stat aggregation and bootstrap/metrics response shape.
- Documented operation stats in architecture docs, UI API docs, README, and this ledger.
- Verified `go test ./internal/httpapi -run 'TestSummarizeOperations|TestBootstrapAndMetricsIncludeOperationStats|TestForward'`, `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, `/ui/`, live bootstrap operation stats, live metrics operation stats, support-bundle behavior preservation, and in-app browser Dashboard rendering with no console errors.

### 2026-07-02T14:16:57Z

- Continuing Phase A: Foundation Hardening.
- Active work item: add warning state in UI when Splunk TLS verification is disabled.
- Goal: make self-signed Splunk lab compatibility visibly risky whenever `verify_ssl` is off, without blocking the explicit user choice.
- Guardrail: do not mark Phase A complete; preserve state schema/migration, encrypted secret storage and rekey behavior, streaming retry semantics and operation stats, streaming compatibility checks, queue scheduling and visibility, normalized OpenAI/Ollama error shapes, sanitized and versioned support-bundle export behavior, LAN exposure warnings, auth failure rate limiting, and client preset behavior. Do not expose HEC tokens, admin tokens, client API keys, request bodies, prompts, responses, raw headers, or payloads in TLS warning metadata.

### 2026-07-02T14:20:59Z

- Added metadata-only Splunk HEC TLS posture fields to bootstrap and telemetry status responses.
- Added `tls_verification_disabled` and `tls_warning` when `verify_ssl` is disabled.
- Added a Splunk UI warning panel that appears only when TLS certificate verification is off.
- Preserved HEC token secrecy by exposing only `has_token`, not token values, in warning metadata.
- Added focused tests for disabled/enabled TLS warning metadata and HEC token non-leakage.
- Documented the Splunk TLS warning contract in architecture docs, UI API docs, README, and this ledger.
- Verified `go test ./internal/httpapi -run 'TestSplunkTLSWarning|TestManagementAuth|TestBootstrapAndMetricsIncludeOperationStats'`, `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, live bootstrap/telemetry status warning metadata, support-bundle behavior preservation, and in-app browser Splunk warning rendering with no console errors.
- Restored runtime Splunk settings to safe defaults after verification:
  - HEC disabled
  - URL empty
  - TLS verification enabled

### 2026-07-02T14:24:18Z

- Continuing Phase A: Foundation Hardening.
- Active work item: document backup/restore behavior for encrypted fallback secrets and surface it from Settings/support docs.
- Goal: make the local encrypted fallback backup contract explicit without turning support bundles into backup artifacts.
- Guardrail: do not mark Phase A complete; preserve state schema/migration, encrypted secret storage and rekey behavior, streaming retry semantics and operation stats, streaming compatibility checks, queue scheduling and visibility, normalized OpenAI/Ollama error shapes, sanitized and versioned support-bundle export behavior, LAN exposure warnings, auth failure rate limiting, client preset behavior, and Splunk TLS warning behavior. Do not expose admin tokens, client API keys, HEC tokens, provider keys, plaintext secrets, raw headers, prompts, responses, request bodies, or payloads in backup guidance metadata.

### 2026-07-02T14:28:13Z

- Added backup/restore guidance metadata to encrypted fallback secret-storage status.
- File-key mode now reports `secrets.enc.json` and `secrets.key` as required companion files.
- Env-key mode now reports `secrets.enc.json` and `LLAMA_WRANGLER_SECRETS_KEY` as required companion key sources without returning key values.
- Added Settings UI Backup & Restore guidance under Secret Storage.
- Clarified in storage docs and support-bundle docs that support bundles are diagnostic artifacts, not backups or restore/import formats.
- Added focused tests for backup guidance metadata, support-bundle status shape, and support-bundle documentation validation.
- Verified `go test ./internal/secrets ./internal/httpapi -run 'TestSecretStatus|TestEnvSecretKey|TestRekey|TestSupportBundle|TestSecretRekey|TestSplunkTLSWarning'`, `go test ./internal/httpapi -run 'TestSupportBundle'`, `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, live bootstrap/auth status/support-bundle backup guidance metadata, Splunk TLS warning preservation, and in-app browser Settings rendering with no console errors.

### 2026-07-02T14:31:44Z

- Continuing Phase A: Foundation Hardening.
- Active work item: start OS keychain feasibility/integration planning for admin tokens, client API keys, HEC tokens, and future provider keys.
- Goal: decide whether Phase A can close with the encrypted fallback plus a documented keychain plan, or whether Phase A needs a minimal OS keychain implementation slice first.
- Guardrail: do not mark Phase A complete unless the ledger evidence supports it; preserve state schema/migration, encrypted fallback secret storage, rekey behavior, backup/restore guidance, streaming retry semantics and operation stats, streaming compatibility checks, queue scheduling and visibility, normalized OpenAI/Ollama error shapes, sanitized and versioned support-bundle export behavior, LAN exposure warnings, auth failure rate limiting, client preset behavior, and Splunk TLS warning behavior. Do not expose admin tokens, client API keys, HEC tokens, provider keys, plaintext secrets, raw headers, prompts, responses, request bodies, or payloads in planning metadata.

### 2026-07-02T14:35:57Z

- Added `docs/14_os_keychain_plan.md` with scope, candidate libraries, backend contract, secret naming, non-destructive migration strategy, platform risks, UI/API status expectations, and acceptance criteria.
- Recommended a minimal additive OS keychain backend spike using a narrow `SecretBackend` boundary, with encrypted fallback retained.
- Surfaced OS keychain planning metadata through secret-storage status, bootstrap, auth status, support bundles, and Settings.
- Kept encrypted fallback as the active backend; no OS keychain dependency or destructive migration was introduced in this planning slice.
- Added focused validation for the OS keychain plan and planning metadata.
- Verified `go test ./internal/secrets ./internal/httpapi -run 'TestOSKeychainPlan|TestSecretStatus|TestSupportBundle|TestSecretRekey'`, `go test ./...`, `go build ./cmd/llama-wrangler`, `/healthz`, live bootstrap/auth status/support-bundle keychain planning metadata, Splunk TLS warning preservation, and in-app browser Settings rendering with no console errors.
- Decision: Phase A should remain open; planning alone is not enough evidence to close it.

### 2026-07-02T18:30:20Z

- Continuing Phase A: Foundation Hardening.
- Active work item: add a minimal additive OS keychain backend spike behind the existing secret-store API while keeping encrypted fallback available.
- Goal: introduce an optional OS keychain backend path with non-destructive fallback retention, metadata-only status, deterministic tests, and opt-in platform checks.
- Guardrail: do not mark Phase A complete unless the ledger evidence supports it; preserve state schema/migration, encrypted fallback secret storage, rekey behavior, backup/restore guidance, OS keychain planning metadata, streaming retry semantics and operation stats, streaming compatibility checks, queue scheduling and visibility, normalized OpenAI/Ollama error shapes, sanitized and versioned support-bundle export behavior, LAN exposure warnings, auth failure rate limiting, client preset behavior, and Splunk TLS warning behavior. Do not expose admin tokens, client API keys, HEC tokens, provider keys, plaintext secrets, raw headers, prompts, responses, request bodies, or payloads in keychain metadata.

### 2026-07-02T18:38:24Z

- Added a narrow secret backend boundary and optional OS keychain backend using `github.com/zalando/go-keyring`.
- Added `LLAMA_WRANGLER_SECRET_BACKEND=os_keychain` opt-in behavior while keeping encrypted fallback as the default and always retaining fallback availability.
- Added non-destructive fallback-to-keychain migration for missing keychain items, with migrated count metadata only.
- Added unavailable-keychain fallback behavior so encrypted fallback becomes active if keychain operations fail.
- Extended secret-storage status with fallback backend/availability, keychain status, keychain plan path, next-step guidance, and migrated count.
- Updated Settings Secret Storage UI to show OS keychain status and fallback availability without exposing secret values.
- Updated README, architecture, configuration storage, UI/API docs, and OS keychain plan to reflect the implemented opt-in spike rather than planning-only status.
- Added focused tests for opt-in keychain migration, retained encrypted fallback, unavailable-keychain fallback, default disabled metadata, docs validation, and an opt-in platform check guarded by `LLAMA_WRANGLER_RUN_KEYCHAIN_TESTS=1`.
- Verified `go test ./internal/secrets -run 'TestOSKeychain|TestSecretStatus|TestEnvSecretKey|TestRekey'`, `go test ./internal/httpapi -run 'TestSupportBundle|TestSecretRekey|TestSplunkTLSWarning'`, `go test ./...`, and `go build ./cmd/llama-wrangler`.
- Verified the real OS keychain platform check is skipped unless `LLAMA_WRANGLER_RUN_KEYCHAIN_TESTS=1` is set.
- Restarted the local service at `http://localhost:11435/ui/` and verified `/healthz`, `/ui/`, live bootstrap, auth status, and support-bundle export.
- Verified live metadata preserves state/config versioning, encrypted fallback, rekey behavior, backup/restore guidance, operation stats, queue scheduling, support-bundle schema/privacy fields, LAN warnings, client presets, and Splunk TLS warning metadata.
- Verified in-app browser Settings rendering shows backend `encrypted_file`, OS keychain status `disabled`, fallback `encrypted_file`, opt-in guidance, and no console errors.
- Decision: Phase A remains open until opt-in real keychain platform/service-mode checks provide enough evidence to close it or identify one more hardening slice.

### 2026-07-02T18:39:41Z

- Continuing Phase A: Foundation Hardening.
- Active work item: run and harden opt-in OS keychain platform/service-mode checks for the existing keychain backend spike.
- Goal: collect real OS keychain evidence where practical, make keychain platform/service-mode metadata explicit, and preserve encrypted fallback behavior if the keychain is unavailable or unsuitable.
- Guardrail: do not mark Phase A complete unless the evidence supports it; preserve state schema/migration, encrypted fallback secret storage, rekey behavior, backup/restore guidance, OS keychain planning/status metadata, streaming retry semantics and operation stats, streaming compatibility checks, queue scheduling and visibility, normalized OpenAI/Ollama error shapes, sanitized and versioned support-bundle export behavior, LAN exposure warnings, auth failure rate limiting, client preset behavior, and Splunk TLS warning behavior. Do not expose admin tokens, client API keys, HEC tokens, provider keys, plaintext secrets, raw headers, prompts, responses, request bodies, or payloads in keychain checks or metadata.

### 2026-07-02T18:44:17Z

- Ran the opt-in real OS keychain check with `LLAMA_WRANGLER_RUN_KEYCHAIN_TESTS=1`; macOS interactive set/get/delete passed for the current user session.
- Added metadata-only OS keychain platform/runtime context to secret-storage status:
  - `os_keychain_platform`
  - `os_keychain_runtime`
  - `os_keychain_service_mode`
  - `os_keychain_warning`
- Added `LLAMA_WRANGLER_SERVICE_MODE=1` and common service-environment marker detection so future service wrappers can explicitly surface service-like keychain risk.
- Added service-like runtime warning behavior while keeping encrypted fallback available and keychain opt-in behavior unchanged.
- Updated Settings Secret Storage UI to show platform, runtime, and service warning metadata without exposing secret values.
- Updated README, architecture, configuration storage, UI/API docs, and OS keychain plan to document the runtime metadata and service-mode caveat.
- Added focused tests for service-like runtime warning metadata and expanded OS keychain plan validation.
- Verified `go test ./internal/secrets -run 'TestOSKeychain|TestSecretStatus'`, `go test ./internal/httpapi -run 'TestSupportBundle|TestSecretRekey|TestSplunkTLSWarning'`, `LLAMA_WRANGLER_RUN_KEYCHAIN_TESTS=1 go test -v ./internal/secrets -run TestOSKeychainBackendPlatformOptIn`, `go test ./...`, and `go build ./cmd/llama-wrangler`.
- Temporarily ran the service with `LLAMA_WRANGLER_SECRET_BACKEND=os_keychain LLAMA_WRANGLER_SERVICE_MODE=1`; live bootstrap, auth status, and support-bundle export reported backend `os_keychain`, platform `darwin`, runtime `service_like`, service warning metadata, encrypted fallback availability, and no secret-marker leakage.
- Restored the default service at `http://localhost:11435/ui/`; live bootstrap and support-bundle export report backend `encrypted_file`, platform `darwin`, runtime `interactive_user`, fallback availability, support-bundle privacy, queue scheduling, operation stats, LAN safe defaults, client presets, and Splunk TLS metadata.
- Verified in-app browser Settings rendering shows backend `encrypted_file`, platform `darwin`, runtime `interactive_user`, fallback `encrypted_file`, and no console errors or visible full token patterns.
- Decision: Phase A remains open. The macOS interactive keychain path is proven for this user session, but true launchd/systemd/Windows service-wrapper keychain behavior has not been implemented or verified.

### 2026-07-02T18:46:05Z

- Continuing Phase A: Foundation Hardening.
- Active work item: add a minimal OS-native service-wrapper dry-run plan/harness, starting with macOS launchd, to verify keychain behavior across interactive setup and service-like runtime.
- Goal: generate reviewable service-wrapper artifacts and validation guidance without installing or mutating OS service state, so keychain service-mode behavior can be tested deliberately.
- Guardrail: do not mark Phase A complete unless evidence supports it; preserve encrypted fallback behavior, keychain opt-in behavior, service-like runtime metadata, state schema/migration, rekey behavior, backup/restore guidance, streaming retry semantics and operation stats, streaming compatibility checks, queue scheduling and visibility, normalized OpenAI/Ollama error shapes, sanitized and versioned support-bundle export behavior, LAN exposure warnings, auth failure rate limiting, client preset behavior, and Splunk TLS warning behavior. Do not expose admin tokens, client API keys, HEC tokens, provider keys, plaintext secrets, raw headers, prompts, responses, request bodies, or payloads in dry-run artifacts.

### 2026-07-02T18:49:55Z

- Added `llama-wrangler service-dry-run --target launchd` as a non-mutating dry-run harness.
- Added review-only launchd plan generation with plist text, service label, program arguments, environment, validation commands, keychain check commands, warnings, and notes.
- Dry-run output includes `LLAMA_WRANGLER_SERVICE_MODE=1` and includes `LLAMA_WRANGLER_SECRET_BACKEND=os_keychain` only when `--keychain` is passed.
- The harness rejects invalid `--mode start --config ...` combinations because `start` remains UI-first and does not accept config paths.
- Added `docs/15_service_wrapper_dry_run.md` with the manual validation sequence and Phase A decision rule.
- Updated README, friendly UI/install docs, and OS keychain plan docs to reference the dry-run harness.
- Added focused tests for launchd dry-run content, opt-in keychain env, default no-keychain behavior, plist escaping, and invalid config/mode handling.
- Verified `go test ./internal/servicewrap`, `go test ./internal/servicewrap ./internal/secrets -run 'TestLaunchd|TestOSKeychainPlan'`, `go test ./cmd/llama-wrangler`, `go test ./...`, and `go build ./cmd/llama-wrangler`.
- Verified `./llama-wrangler service-dry-run --target launchd --binary ./llama-wrangler --keychain` emits dry-run JSON with launchd plist, validation commands, keychain check commands, fallback warnings, and no secret-marker leakage.
- Verified invalid dry-run usage reports `config path requires marshal, subscriber, or standalone mode`.
- Restarted the default service at `http://localhost:11435/ui/`; verified `/healthz`, live bootstrap, support-bundle export, queue scheduling metadata, operation stats, LAN safe defaults, client presets, Splunk TLS metadata, and secret-storage metadata remain intact.
- Verified in-app browser Settings rendering still shows backend `encrypted_file`, platform `darwin`, runtime `interactive_user`, fallback `encrypted_file`, and no console errors or visible full token patterns.
- Decision: Phase A remains open. The dry-run harness is in place, but no real launchd bootstrap/load/keychain service-user validation was performed in this slice.

### 2026-07-02T18:52:15Z

- Scope update requested: add Managed Node and Passive Endpoint modes.
- Managed Node mode means a Llama Wrangler subscriber is installed on the asset with Ollama and can report hardware, model state, load, health, benchmarks, warm-model state, and richer telemetry.
- Passive Endpoint mode means the marshal only knows about an existing Ollama-compatible endpoint URL; it can route requests and collect marshal-observed telemetry, but cannot fully control or inspect the asset.
- Planning requirement: the UI must support both enrollment flows:
  - install/enroll Wrangler subscriber for full-control managed nodes
  - add existing Ollama endpoint for limited-control passive endpoints
- Planning requirement: routing, consensus, benchmarks, safety policy, and UI badges must account for node control level and trust level.
- Side-quest handling: capture this as a Phase B/C planning scope expansion without derailing the active Phase A launchd validation slice.
- Continuing active Phase A work item: manually exercise the macOS launchd dry-run validation path in a disposable test flow, preserving secret/keychain safety.

### 2026-07-02T19:02:30Z

- Added `docs/16_node_control_modes.md` defining Managed Node and Passive Endpoint modes, control levels, trust levels, routing/consensus policy implications, and two future UI flows.
- Updated architecture, capability model, and MVP roadmap docs so routing, consensus, benchmarks, safety policy, and UI badges account for node control level and trust level.
- Added `LLAMA_WRANGLER_KEYCHAIN_SERVICE` override for disposable keychain namespaces so launchd/service validation does not overwrite normal `llama-wrangler` keychain items.
- Added `service-dry-run --env KEY=VALUE` support so generated launchd plists can carry disposable `HOME` and keychain service namespace metadata.
- Updated service-wrapper dry-run docs to describe disposable validation with temp HOME and `LLAMA_WRANGLER_KEYCHAIN_SERVICE`.
- Verified focused tests for service wrapper and secret-store keychain namespace behavior, then verified `go test ./...`, `go build ./cmd/llama-wrangler`, and `LLAMA_WRANGLER_RUN_KEYCHAIN_TESTS=1 go test -v ./internal/secrets -run TestOSKeychainBackendPlatformOptIn`.
- Manually exercised launchd validation on a disposable port (`127.0.0.1:11437`) with a temp HOME and disposable keychain service namespace.
- Validation result:
  - `plutil -lint` passed for the generated plist.
  - `launchctl bootstrap` loaded the generated LaunchAgent and `launchctl print` showed it running.
  - Disposable interactive setup reported `backend: encrypted_file`, `fallback_available: true`, `os_keychain_status: unavailable`, runtime `interactive_user`.
  - Disposable launchd run reported `backend: encrypted_file`, `fallback_available: true`, `os_keychain_status: unavailable`, runtime `service_like`, `os_keychain_service_mode: true`, and warning metadata.
  - `launchctl bootout` completed.
  - disposable keychain cleanup was attempted and the test keychain item was not found afterward.
- Restarted the default service at `http://localhost:11435/ui/`; verified `/healthz`, bootstrap, support-bundle export, secret-storage metadata, queue scheduling metadata, operation stats, LAN safe defaults, client presets, Splunk TLS metadata, and no secret-marker leakage.
- Verified in-app browser Settings rendering still shows backend `encrypted_file`, platform `darwin`, runtime `interactive_user`, fallback `encrypted_file`, and no console errors or visible full token patterns.
- Decision: Phase A remains open for one explicit closure decision. Evidence now supports treating encrypted fallback as the service/default credential path while keeping OS keychain as interactive opt-in until packaging/service keychain behavior is revisited.

### 2026-07-02T19:07:44Z

- Continuing from the ledger on the next slice.
- Active work item: record the Phase A closure decision based on disposable launchd validation evidence, marking encrypted fallback as the supported service/default credential path and OS keychain as interactive opt-in plus packaging hardening.
- If the evidence supports closure, mark Phase A complete and begin Phase B planning for Managed Node versus Passive Endpoint data model and UI flows.
- Guardrail: preserve state schema/migration, encrypted fallback secret storage, rekey behavior, backup/restore guidance, OS keychain planning/status metadata, service-wrapper validation evidence, streaming retry semantics and operation stats, streaming compatibility checks, queue scheduling and visibility, normalized OpenAI/Ollama error shapes, sanitized and versioned support-bundle export behavior, LAN exposure warnings, auth failure rate limiting, client preset behavior, Splunk TLS warning behavior, and metadata-only telemetry.
- Guardrail: do not expose admin tokens, client API keys, HEC tokens, future provider keys, plaintext secrets, raw headers, prompts, responses, request bodies, or payloads in closure docs, Phase B planning, tests, support bundles, or UI/API verification.
- Do not mark Phase B implementation items done during planning-only work.

### 2026-07-02T19:09:53Z

- Added `docs/17_phase_a_closure_decision.md` documenting the Phase A credential decision, supporting launchd evidence, preserved secret/privacy requirements, and residual packaging hardening.
- Added `docs/18_phase_b_managed_passive_plan.md` defining the next Phase B data model, Managed Node flow, Passive Endpoint flow, UI badges, routing/safety hooks, and acceptance criteria.
- Updated configuration storage, OS keychain, and service-wrapper dry-run docs so they no longer describe service-keychain validation as a Phase A blocker.
- Marked Phase A complete based on recorded evidence: encrypted fallback is the supported service/default credential path, OS keychain remains interactive opt-in, and service-keychain guarantees move to packaging hardening.
- Began Phase B planning for Managed Node versus Passive Endpoint work without marking Phase B implementation tasks done.
- Added focused doc-validation tests for the Phase A closure decision and Phase B Managed Node versus Passive Endpoint plan.
- Verified `go test ./internal/secrets -run 'TestOSKeychainPlan|TestPhaseAClosure'`.
- Verified `go test ./internal/appstate -run TestManagedPassivePlanningDocumentsDataModelAndUIFlows`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Verified live `/healthz`, `/ui/`, bootstrap secret-storage/queue/operation/safe-default metadata, and support-bundle schema/privacy metadata.
- Verified token-shaped leak probes against bootstrap and support-bundle output returned no full admin/client token patterns or known secret markers.
- Verified in-app browser Settings renders encrypted fallback Secret Storage and Backup & Restore guidance with no full token patterns and no console errors.
- Verified in-app browser Nodes renders the existing Add Subscriber flow and current local node with no full token patterns and no console errors.

### 2026-07-02T19:14:51Z

- Repository checkpoint requested before additional development.
- Created local root commit `7e6d7de` with the completed Phase A foundation and Phase B planning docs after verifying `go test ./...`.
- Added remote `origin` as `https://github.com/LeiterConsulting/llama-wrangler.git`.
- Push to `origin/main` is blocked in the current shell because no GitHub HTTPS credentials are configured and `gh` is not logged in. Local commit remains intact and ready to push when credentials are available.
- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: implement the Managed Node versus Passive Endpoint state schema and migration foundation, including `control_level`, `trust_level`, `capability_source`, approval state, and source/freshness metadata.
- Guardrail: preserve all Phase A hardening requirements, keep secrets and payloads out of state/API/support bundles, preserve existing manually added subscribers, keep changes additive, and do not claim Passive Endpoint routing/UI flow completion before it exists.

### 2026-07-02T19:20:38Z

- Bumped app-state schema to version `2`.
- Added node metadata fields for `control_level`, `trust_level`, `capability_source`, `approval_state`, health/model/benchmark sources, warm/model-management support flags, telemetry level, `last_observed_at`, and `last_reported_at`.
- Added migration from schema version `1` to `2` that preserves existing local/manual subscriber records as Managed Node metadata, defaults non-loopback manual URLs to `lan_unverified`, and preserves approved legacy nodes as `approval_state: approved`.
- Added Passive Endpoint metadata defaults for future limited-control endpoint records: marshal-observed sources, pending approval, no management support, no warm-state support, marshal-observed telemetry, and no subscriber report timestamp.
- Added initial Nodes UI badges for control level, trust level, approval state, and capability source.
- Updated configuration storage docs, UI/API docs, support-bundle docs, support-bundle JSON schema, and the Phase B plan to reflect the implemented schema foundation.
- Added focused tests for schema migration, passive endpoint metadata defaults, bootstrap metadata, and support-bundle metadata.
- Verified `jq empty schemas/support_bundle.schema.json`.
- Verified `go test ./internal/appstate ./internal/httpapi`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Restarted the local service at `http://localhost:11435/ui/`.
- Verified live `/healthz`.
- Verified live bootstrap reports `schema_version: 2`, migration history, node control/trust/source/approval/freshness metadata, encrypted fallback status, queue scheduling, and safe defaults.
- Verified live support-bundle export reports support-bundle schema version `1`, service schema version `2`, node metadata, privacy flags, and no token-shaped secret marker leakage.
- Verified in-app browser Nodes renders Managed/Local/Approved badges and capability source with no console errors and no full admin/client token patterns.
- Scope not completed yet: dedicated Passive Endpoint add flow, subscriber enrollment approval workflow, routing/consensus/benchmark policy use of control/trust metadata.

## Next Recommended Work

1. Add the Passive Endpoint add flow with endpoint URL, display name, explicit trust-level selection, safe `/api/tags` validation, limitations copy, and passive metadata defaults.
2. Preserve Managed Node subscriber add behavior while separating it visually from Passive Endpoint addition.
3. Add focused tests for passive endpoint API/UI behavior, support-bundle privacy, and no prompt/payload/secret persistence.
