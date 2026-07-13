# Llama Wrangler Project Ledger

Last updated: 2026-07-13T12:58:20Z

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
- a clean future path toward broader Capability Endpoints after the Ollama-first V1 is functional

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

Additive product and future-scope requirements:

- `docs/llama_wrangler_additive_managed_passive_nodes_and_product_refinements.md`
- `docs/llama_wrangler_additive_future_capability_endpoints.md`

Tasking and phase-planning overlays:

- `docs/18_phase_b_managed_passive_plan.md`
- `docs/19_capability_endpoint_future_plan.md`

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
- Do not implement GitHub, Xcode, Docker, CI/CD, Codex-agent, or other broad non-Ollama capability integrations before the V1 Ollama fleet control plane is working.
- Keep future Capability Endpoint work additive and explicitly labeled as future scope until V1 acceptance criteria are met.
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
- Consolidated V1 acceptance/security matrix and executable non-mutating local harness for automated safe-default, lifecycle, auth, node-policy, routing/consensus, benchmark/model, Splunk-asset, and support-bundle privacy gates.
- Managed Node and Passive Endpoint control-mode planning documented for future enrollment, routing, consensus, benchmark, safety, and UI-badge work.
- Future Capability Endpoint expansion guidance incorporated as a V2 planning boundary while preserving the current Ollama-first V1 scope.
- App-state schema version 2 for Managed Node versus Passive Endpoint metadata.
- Node metadata migration for control level, trust level, capability source, approval state, source fields, support flags, telemetry level, and freshness timestamps.
- Nodes UI badges for control level, trust level, approval state, and capability source.
- Passive Endpoint add flow with endpoint URL, display name, explicit trust-level selection, safe `/api/tags` validation, limitations UI, and passive metadata defaults.
- Node approval and revocation controls in the Nodes UI and API.
- Routing eligibility enforcement for pending versus approved nodes and endpoints.
- Explicit trust-level update controls for existing Managed Nodes and Passive Endpoints.
- Nodes UI warning states for `lan_unverified` and `external` trust-level changes before update submission.
- Routing policy use of control/trust metadata beyond approval gating:
  - `external` trust is excluded by default
  - Passive Endpoints and `lan_unverified` nodes are de-prioritized for approved single-route requests
  - consensus eligibility requires Managed Nodes with `local` or `lan_trusted` trust
  - consensus aliases enforce configured minimum participant counts
- Metadata-only routing candidate and exclusion reason telemetry.
- Session affinity guardrail preventing strict/task affinity from selecting a policy-excluded node.
- Managed Node enrollment token flow:
  - one-time short-lived token generation for operator display
  - token hashes and safe hints stored in the enrollment queue
  - subscriber self-registration through token consumption
  - registered subscribers created as pending Managed Nodes requiring approval before routing
- Nodes UI enrollment-token controls and sanitized enrollment queue display.
- Managed Node heartbeat and freshness handling:
  - `POST /subscriber/heartbeat` accepts metadata-only subscriber reports for registered Managed Nodes
  - heartbeat updates refresh `last_reported_at`, health/model/load metadata, and freshness state
  - heartbeat-required Managed Nodes with missing or stale reports are excluded from routing until a fresh report arrives
  - legacy/manual Managed Nodes without heartbeat-required metadata remain eligible under existing approval, health, model, control, and trust policies
- Managed Node heartbeat identity hardening:
  - derives a shared heartbeat credential from the one-time enrollment token and final node ID
  - stores the derived credential only in the encrypted secret backend
  - requires the credential on heartbeats for nodes that have one
  - exposes only safe method, derivation, and token-hint metadata
- Managed Node heartbeat credential rotation/re-provisioning:
  - explicit admin action at `POST /wrangler/nodes/:id/heartbeat-credential/rotate`
  - generated heartbeat credential is returned only in the immediate rotation response
  - raw credential is stored only in the configured secret backend
  - node state, UI, telemetry, and support bundles expose only safe hint/provisioning metadata
  - Passive Endpoints are rejected for heartbeat credential provisioning
  - old or missing credentials are rejected after rotation
- Subscriber-side heartbeat credential install guidance:
  - subscriber config supports `registration.heartbeat_credential_env`
  - documented env var is `LLAMA_WRANGLER_SUBSCRIBER_HEARTBEAT_CREDENTIAL`
  - heartbeat credential rotation responses include a safe `subscriber_install` guidance object with placeholder shell export, launchd dry-run, and heartbeat verification commands
  - Nodes UI shows a one-time install guidance card after rotation with copy actions while keeping raw credentials out of visible node metadata
  - raw rotated credentials remain limited to the immediate rotation response and are not persisted to app state, bootstrap, support bundles, telemetry, or node metadata
- Safe peer discovery:
  - explicit operator action at `POST /wrangler/setup/discover-peers`
  - one-shot bounded mDNS/Bonjour queries for Llama Wrangler and Ollama service names
  - candidates are review-only metadata with `approval_state: not_added` and default `trust_level: lan_unverified`
  - discovered candidates are not persisted, approved, or routed automatically
  - subnet scanning remains disabled and documented as future explicit opt-in only
- Nodes UI freshness display for Managed Node heartbeat/report state.
- Routing/operations policy warning status:
  - `routing_policy_status` in bootstrap and metrics
  - metadata-only warning reason codes for unapproved, disabled, Passive Endpoint consensus exclusion, `lan_unverified` de-prioritization/consensus exclusion, `external` exclusion, missing heartbeat, and stale heartbeat
  - Dashboard and Nodes warning cards explaining current routing/consensus posture
- Benchmark policy wiring:
  - `benchmark_policy_status` in bootstrap and metrics
  - eligible Managed Nodes can queue subscriber-reported benchmark metadata
  - Passive Endpoints are marked marshal-observed/probe-only and cannot queue local benchmark control actions
  - Dashboard and Nodes UI show benchmark policy/source state without exposing secrets or payloads
- Initial benchmark execution/result ingestion:
  - authenticated Managed Node subscribers can report safe benchmark result metadata through `POST /subscriber/benchmarks`
  - Managed Node result ingestion stores bounded status/timing/token-rate summaries and updates model token-rate metadata without prompts, responses, raw headers, request bodies, credentials, or payloads
  - Passive Endpoints can run a marshal-observed `/api/tags` availability/latency probe through `POST /wrangler/nodes/:id/benchmark-probe`
  - Passive Endpoint benchmark probes remain limited to marshal-observed metadata and do not imply local hardware, warm-state, load, model-management, or prompt-workload control
  - Nodes UI shows benchmark result summaries and keeps Managed Node benchmark requests distinct from Passive Endpoint `/api/tags` probes
- Managed Node benchmark job orchestration:
  - eligible Managed Node benchmark actions create explicit metadata-only benchmark jobs
  - subscriber job claim and status endpoints are authenticated with the same heartbeat credential semantics when a stored credential exists
  - benchmark result ingestion completes matching jobs by benchmark ID
  - Nodes UI shows benchmark job state alongside result summaries
  - Passive Endpoints remain limited to marshal-observed `/api/tags` probes and never receive local-control benchmark jobs
- Configurable benchmark scheduler policy controls:
  - default bounded retry/timeout policy remains `bounded_retry_timeout_v1`
  - safe defaults remain max attempts 3, lease timeout 600 seconds, retry delay 60 seconds
  - operator-configured values are normalized to bounded metadata-only limits
  - new benchmark jobs, subscriber claims, retry scheduling, and scheduler status use the normalized policy
  - Dashboard exposes max-attempt, lease-timeout, and retry-delay controls without storing prompts, responses, credentials, or payloads
- Optional background benchmark scheduler reconciliation:
  - disabled by default
  - operator-configured enablement and tick interval are normalized to bounded metadata-only config
  - background ticks reuse the existing reconciliation primitive for durable Managed Node benchmark jobs
  - ticks do not create jobs, run prompts, inspect Passive Endpoints, mutate subscribers, or store payloads
  - bootstrap, metrics, Dashboard, and telemetry expose only enablement, interval, timestamps, reason, and summary counts
- Subscriber benchmark runner dry-run harness and packaging hooks:
  - disabled by default and explicitly opt-in through `capabilities.benchmark_runner`
  - current mode is `dry_run_v1` with result body policy `metrics_only`
  - subscriber-mode loop claims Managed Node jobs, sends status transitions, and reports deterministic metric summaries only
  - bootstrap, metrics, API guidance, and Dashboard expose runner mode, status, poll interval, max jobs per tick, and packaging boundaries
  - built-in synthetic prompt text, response text, fixture contents, full local paths, raw credentials, and payloads stay out of marshal state/API/support bundles
- Hardened manual subscriber add compatibility path:
  - validates subscriber URLs with safe HTTP/HTTPS URL rules
  - rejects embedded credentials, relative URLs, and unsupported schemes
  - creates pending Managed Nodes instead of immediately approved nodes
  - preserves operator approval as the routing gate
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
- Focused tests for Passive Endpoint add validation, unsafe URL/trust rejection, Managed Node subscriber behavior preservation, and support-bundle privacy preservation.
- Focused tests for node approval/revocation actions and routing exclusion of pending/revoked nodes.
- Focused tests for node trust-level updates, invalid trust-level rejection, metadata preservation, and secret/payload non-leakage.
- Focused tests for routing control/trust policy, consensus eligibility, minimum consensus participants, metadata-only policy telemetry, and affinity exclusion guardrails.
- Focused tests for Managed Node enrollment token creation, token-hash-only storage, invalid token rejection, subscriber registration as pending Managed Node, token consumption, and support-bundle/bootstrap token non-leakage.
- Focused tests for subscriber heartbeat credential env-var resolution and rotation response install-plan placeholders, launchd dry-run guidance, heartbeat verification guidance, and raw credential non-duplication.
- Focused tests for retry-before-first-token, no-retry-after-partial-output, and cancellation telemetry.
- Focused tests for OpenAI SSE and Ollama JSONL streaming compatibility through the public marshal routes.
- Focused tests for queue snapshot shape, priority normalization, bootstrap queue metadata, and marshal proxy queue telemetry.
- Focused tests for weighted-priority queue scheduling, FIFO fallback dispatch, routing-policy persistence, and queue scheduling metadata.
- Focused tests for OpenAI/Ollama error shapes, client-auth errors, no-eligible-node errors, upstream failure sanitization, and upstream 4xx normalization.
- Focused tests for support-bundle shape, schema/config metadata, queue metadata, and redaction of secrets, token-like values, prompt bodies, response bodies, and payload-like fields.
- Focused tests for LAN exposure metadata and admin/OpenAI/Ollama auth failure rate-limit behavior.
- Focused tests for metadata-only operation stats in bootstrap and metrics.
- Focused tests for Splunk HEC TLS warning metadata and HEC token non-leakage.
- Focused tests for Managed Node benchmark result ingestion, subscriber benchmark authentication, metadata-only benchmark storage, support-bundle/bootstrap benchmark privacy, and Passive Endpoint marshal-observed `/api/tags` probe behavior.
- Focused tests for Managed Node benchmark job creation, credentialed job claim, job status updates, result completion, no-job claim behavior, Passive Endpoint job rejection, and job metadata privacy.
- Focused tests for benchmark scheduler policy config normalization, policy endpoint persistence, new job metadata, claim lease timeout, failed-job retry delay, scheduler status config, and secret/payload non-leakage.
- Focused tests for background scheduler disabled-by-default behavior, enabled tick reconciliation, background status metadata, background tick telemetry, config bounds, and secret/payload non-leakage.
- Focused tests for subscriber benchmark runner guidance endpoint/bootstrap metadata, supported suite IDs, placeholder-only commands, guidance-only runner status, and secret/payload non-leakage.

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
- Live passive-add route rejects credential-bearing endpoint URLs without echoing secret material.
- In-app browser verified Nodes shows separate Managed Node and Passive Endpoint add flows, trust-level selector, `/api/tags` limitation copy, no console errors, and no full admin/client token patterns.
- Live approval route for an unknown node returns safe `node not found` without mutating local state.
- In-app browser verified Nodes shows Revoke for the approved local node, does not show an unnecessary Approve button for that approved node, and has no console errors or full admin/client token patterns.
- Live trust route for an unknown node returns safe `node not found` without mutating local state.
- In-app browser verified Nodes shows the existing Managed Node trust selector, Update trust control, existing Revoke control, immediate `external` and `lan_unverified` warning text on selector changes, and no console errors.
- Live no-payload `local-consensus` probe with the current single approved local node returns OpenAI-style `no_eligible_node`, confirming minimum consensus participant enforcement without sending real prompt content.
- Served `/ui/`, `/ui/app.js`, and `/ui/app.css` return the expected updated UI assets.
- In-app browser automation timed out during this slice, so visual browser inspection was not completed after the routing policy change; served UI assets and API behavior were verified instead.
- Live enrollment-token API rejects credential-bearing subscriber URLs without mutating the real enrollment queue.
- Live subscriber enrollment rejects invalid/expired tokens without creating a node.
- Live bootstrap reports sanitized enrollment queue metadata without token hashes; the current live queue remains empty after non-mutating checks.
- Served UI assets include the Managed Node enrollment token controls and manual-add compatibility controls.
- Live `/subscriber/benchmarks` missing-node probe returns a safe `node not found` response without echoing submitted prompt-like fields.
- Live served `/ui/app.js` includes the new Benchmark result, Run benchmark, and Passive Endpoint `/api/tags` probe affordances.
- Live bootstrap remains schema version `2` and preserves benchmark policy metadata.
- Live support-bundle leak probe returns no heartbeat/enrollment/admin/client token-shaped values, 64-character token hashes, authorization headers, OpenAI-style `sk-` markers, prompt markers, response markers, or known secret markers.
- Live benchmark scheduler policy endpoint returns bounded metadata-only config and policy limits.
- Live bootstrap and metrics report benchmark scheduler config/limits without secrets or payloads.
- In-app browser verified Dashboard renders benchmark scheduler policy controls with bounded min/max values and no console errors.
- Live benchmark scheduler policy endpoint reports background reconciliation disabled by default with bounded tick interval limits.
- Live bootstrap and metrics report background scheduler status without secrets or payloads.
- In-app browser verified Dashboard renders background reconciliation toggle, tick interval bounds, next/last tick status, no visible secret markers, and no console errors.
- Live benchmark runner guidance endpoint reports guidance-only status, supported suite IDs, planned packaging hook status, and local fixture storage policy.
- Live bootstrap reports `benchmark_runner` guidance while preserving schema version 2 and metadata-only posture.
- In-app browser verified Dashboard renders Subscriber Benchmark Runner guidance with no visible secret markers and no console errors.

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
- Done: add Passive Endpoint add flow with safe `/api/tags` validation and explicit limitations.
- Done: add approval/revocation controls and enforce routing eligibility for pending versus approved nodes.
- Done: add explicit trust-level update controls and warnings for `lan_unverified` and `external`.
- Done: start using control/trust metadata in routing and consensus eligibility beyond approval gating.
- Done: add enrollment token flow and marshal approval queue for Managed Node subscriber enrollment.
- Done: add subscriber registration heartbeat and freshness handling for Managed Nodes.
- Done: add routing/operations UI warnings that explain current policy exclusions and limitations.
- Done: harden manual subscriber add behind safer pending-approval semantics and stronger warnings.
- Done: add shared-secret identity hardening for Managed Node heartbeats.
- Done: add configurable benchmark scheduler policy controls for max attempts, lease timeout, and retry delay with bounded metadata-only defaults.
- Done: add optional background scheduler tick for benchmark job reconciliation with operator-visible enablement and metadata-only telemetry.
- Done: add subscriber-side benchmark runner guidance and packaging hooks for suite/task-ID execution with metric-only reporting.
- Expand Managed Node install/enroll flow with approval state.
- Complete manual subscriber enrollment with approval state.
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

Status: non-streaming foundation and partial-failure hardening implemented; additional compatibility/acceptance work remains.

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

Status: overview and metadata-only operations dashboards implemented; full domain dashboard/alert/sample suite remains open.

- Done: add a dedicated Operations dashboard for consensus failure reasons, queue scheduling, streaming outcomes, benchmark scheduler/runner history, routing exclusions, and model lifecycle actions.
- Done: add compact operations signals to the Overview dashboard.
- Done: add packaged navigation, reusable operational macros/eventtypes/tags/props, disabled-by-default saved reports, and focused asset/privacy validation.
- Remaining domain dashboards:
  - Node Health
  - Model Performance
  - Routing Decisions
  - Consensus and Frontier Delta
  - Client/IDE Usage
  - Frontier Usage
- Add alerts for failures, queue pressure, policy denials, and HEC delivery failures.
- Add sample events for all sourcetypes.

### Phase H: Install and Packaging

Status: disposable local acceptance foundation implemented; real OS packaging remains open.

- Done: add the non-mutating V1 acceptance/security matrix and disposable local service start/restart harness.
- Done: add dry-run-first macOS user-level launchd package-plan validation and record current-host opt-in lifecycle evidence for disposable user-agent mechanics.
- Done: suppress recovery-token output in service-like runtimes and validate temporary launchd logs remain free of credential/payload markers.
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
- partially done: store admin token in OS keychain where practical through the opt-in backend spike; macOS interactive keychain and encrypted-fallback user-launchd lifecycle checks passed, while packaged service-keychain behavior remains open
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
- opt-in current-user launchd lifecycle validation confirmed encrypted fallback, schema/auth persistence, service-log privacy, atomic binary replacement, and complete bootout cleanup in a temporary package/home root
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
- Passive Endpoint add flow requires explicit trust selection and stores marshal-observed limited-control metadata after safe `/api/tags` validation
- approve/revoke controls make approval state operator-visible
- routing requires enabled plus approved state and excludes pending/revoked nodes
- trust-level update controls make `local`, `lan_trusted`, `lan_unverified`, and `external` changes explicit
- Nodes UI warns for `lan_unverified` and `external` trust selections before update submission
- routing excludes `external` trust by default
- routing de-prioritizes Passive Endpoints and `lan_unverified` candidates for approved single-route requests
- consensus eligibility requires Managed Nodes with `local` or `lan_trusted` trust
- routing telemetry includes only metadata: node ID, control level, trust level, capability source, and policy reason codes
- Managed Node enrollment tokens are shown once, expire, and are stored only as token hashes plus safe hints
- subscriber registration creates pending Managed Nodes that remain excluded from routing until approved

Required mitigation:

- done: add node control/trust metadata foundation
- done: add Passive Endpoint add flow with explicit trust selection and limited-control warnings
- done: enforce approval state for routing eligibility
- done: add explicit trust-level update controls and warnings
- done: use control/trust metadata in routing and consensus eligibility
- done: add enrollment token flow and marshal approval queue
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

### 2026-07-02T23:53:37Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: add the Passive Endpoint add flow with endpoint URL, display name, explicit trust-level selection, safe `/api/tags` validation, limitations UI, and passive metadata defaults.
- Guardrail: preserve all Phase A hardening requirements and schema version 2 behavior; keep secrets, raw headers, prompts, responses, request bodies, and payloads out of state, APIs, telemetry, and support bundles; preserve Managed Node subscriber behavior; keep Passive Endpoint telemetry marshal-observed and metadata-only.
- Do not mark subscriber enrollment approval, routing/consensus trust policy use, or benchmark policy work done in this slice.

### 2026-07-02T23:58:13Z

- Added `POST /wrangler/nodes/passive-add` for Passive Endpoint addition.
- Passive add requires an absolute HTTP/HTTPS endpoint URL without embedded credentials, an explicit trust level, and successful safe `/api/tags` validation.
- Passive Endpoint records store limited-control metadata defaults: `control_level: passive`, explicit `trust_level`, `capability_source: marshal_observed`, `approval_state: pending`, marshal-observed health/model inventory, no management support, no warm-state support, and no subscriber report timestamp.
- Passive validation does not forward client authorization headers or request bodies.
- Passive validation failures return generic client-facing errors without echoing raw headers, tokens, prompts, responses, request bodies, or payloads.
- Preserved existing Managed Node subscriber add behavior and now returns normalized managed metadata in the API response.
- Updated Nodes UI with separate Install/enroll Wrangler subscriber and Add existing Ollama endpoint cards.
- Added trust-level selector, endpoint/display-name fields, and limitations UI explaining `/api/tags` validation and limited-control behavior.
- Passive Endpoint cards omit benchmark controls and show limited-control copy so the UI does not imply managed-node inspection or control.
- Updated UI/API docs, node control docs, and Phase B plan docs to reflect the implemented Passive Endpoint add flow.
- Added focused tests for passive add success, unsafe input rejection, no authorization/body forwarding during validation, Managed Node subscriber behavior preservation, support-bundle privacy preservation, and bootstrap metadata.
- Verified `go test ./internal/httpapi -run 'TestPassiveAddNode|TestManualSubscriberAdd|TestBootstrapIncludesNodeControlTrustMetadata|TestSupportBundle'`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Restarted the local service at `http://localhost:11435/ui/`.
- Verified live `/healthz`.
- Verified live passive-add route rejects credential-bearing endpoint URLs safely.
- Verified live bootstrap remains schema version `2`, preserves the existing managed node, encrypted fallback status, queue scheduling, and safe defaults.
- Verified live support-bundle token-shaped leak probe returns no full admin/client token patterns or known secret markers.
- Verified in-app browser Nodes renders separate Managed Node and Passive Endpoint flows, trust options, `/api/tags` limitation copy, existing Managed/Local/Approved node badges, no console errors, and no full admin/client token patterns.
- Scope not completed yet: subscriber enrollment approval workflow, node approval/revocation controls, routing/consensus/benchmark policy use of control/trust metadata.

### 2026-07-03T00:35:11Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: add node approval/revocation controls and enforce routing eligibility for pending versus approved Managed Nodes and Passive Endpoints.
- Guardrail: preserve all Phase A hardening requirements, schema version 2 behavior, and Passive Endpoint add-flow behavior; keep secrets, raw headers, prompts, responses, request bodies, and payloads out of state, APIs, telemetry, and support bundles.
- Routing guardrail: pending, rejected, revoked, disabled, failed, or otherwise unapproved nodes must not be eligible for inference routing even if they are otherwise enabled or healthy.
- Do not mark trust-level editing, enrollment tokens, subscriber heartbeat, consensus policy, or benchmark policy work done in this slice.

### 2026-07-03T00:38:48Z

- Added node approval and revocation actions at `POST /wrangler/nodes/:id/approve` and `POST /wrangler/nodes/:id/revoke`.
- Approval sets `approval_state: approved`, `approved: true`, and enables the node for routing if other health/model checks pass.
- Revocation sets `approval_state: revoked`, `approved: false`, disables the node, and removes it from routing eligibility.
- Tightened app-state normalization so `approval_state` and legacy `approved` stay synchronized.
- Tightened routing eligibility so nodes must be enabled and approved; pending, rejected, revoked, disabled, failed, or otherwise unapproved nodes are excluded even if they are healthy or have models.
- Preserved legacy compatibility for older in-memory/test nodes that only set `approved: true`.
- Preserved Passive Endpoint add-flow behavior and Managed Node subscriber behavior.
- Added Nodes UI approval/revocation controls: approved nodes show Revoke; pending/revoked nodes can be approved; Passive Endpoint cards remain limited-control and do not show benchmark controls.
- Updated UI/API docs, node control docs, and Phase B plan docs for approval/revocation and routing eligibility behavior.
- Added focused tests for pending/revoked routing exclusion and node approve/revoke actions.
- Verified `go test ./internal/routing ./internal/httpapi -run 'TestSelect|TestNodeApprove|TestPassiveAddNode|TestManualSubscriberAdd|TestSupportBundle|TestBootstrapIncludesNodeControlTrustMetadata'`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Restarted the local service at `http://localhost:11435/ui/`.
- Verified live `/healthz`.
- Verified live bootstrap remains schema version `2`, reports the existing managed node as enabled and approved, and preserves encrypted fallback status, queue scheduling, and safe defaults.
- Verified live unknown-node approval returns safe `node not found`.
- Verified live support-bundle token-shaped leak probe returns no full admin/client token patterns or known secret markers.
- Verified in-app browser Nodes renders approval/revocation controls, keeps both add flows visible, shows Revoke for the approved local node, shows no unnecessary Approve button for that approved node, has no console errors, and shows no full admin/client token patterns.
- Scope not completed yet: trust-level editing controls, enrollment tokens, subscriber heartbeat, consensus policy, benchmark policy, and richer trust-aware routing beyond approval eligibility.

### 2026-07-03T01:21:23Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: add explicit trust-level update controls for existing Managed Nodes and Passive Endpoints, with warnings for `lan_unverified` and `external` trust levels.
- Guardrail: preserve all Phase A hardening requirements, schema version 2 behavior, Passive Endpoint add-flow behavior, and node approval/revocation routing eligibility.
- Guardrail: keep secrets, raw headers, prompts, responses, request bodies, and payloads out of state, APIs, telemetry, support bundles, and UI trust metadata.
- Do not mark enrollment tokens, subscriber heartbeat, consensus policy, benchmark policy, or richer trust-aware routing beyond explicit trust editing done in this slice.

### 2026-07-03T01:27:31Z

- Added `POST /wrangler/nodes/:id/trust` for explicit trust-level updates on existing Managed Nodes and Passive Endpoints.
- Trust updates accept only `local`, `lan_trusted`, `lan_unverified`, and `external`.
- Trust updates preserve control level, approval state, enabled state, Passive Endpoint metadata, and Managed Node metadata.
- Trust-update telemetry is metadata-only: node ID, control level, trust level, and approval state.
- Nodes UI now shows a trust selector and Update trust control for existing nodes/endpoints.
- Nodes UI shows immediate warning text when `lan_unverified` or `external` is selected, before update submission.
- Preserved approval/revocation controls and routing eligibility behavior.
- Updated UI/API docs, node control docs, and Phase B plan docs for trust-level update behavior.
- Added focused tests for valid trust update, invalid trust rejection, metadata preservation, and no obvious secret/payload marker leakage in trust responses.
- Verified `go test ./internal/httpapi -run 'TestNodeTrust|TestNodeApprove|TestPassiveAdd'`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Restarted the local service at `http://localhost:11435/ui/`.
- Verified live `/healthz`.
- Verified live bootstrap remains schema version `2`, reports the existing managed node as `trust_level: local`, `approval_state: approved`, enabled, managed, and preserves encrypted fallback status, weighted queue scheduling, and safe defaults.
- Verified live trust route for an unknown node returns safe `node not found` without mutating local state.
- Verified live support-bundle token-shaped leak probe returns no full admin/client token patterns or known secret markers.
- Verified in-app browser Nodes renders trust selectors, Update trust, existing Revoke control, separate add flows, immediate `external` and `lan_unverified` warnings on selector changes, and no console errors.
- Scope not completed yet: enrollment tokens, subscriber heartbeat, consensus policy, benchmark policy, and deeper trust-aware routing beyond approval eligibility.

### 2026-07-03T04:21:08Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: start using `control_level` and `trust_level` metadata in routing and consensus eligibility beyond approval gating.
- Guardrail: preserve all Phase A hardening requirements, schema version 2 behavior, Passive Endpoint add-flow behavior, node approval/revocation routing eligibility, and trust-level update behavior.
- Guardrail: telemetry, support bundles, APIs, and UI policy metadata must remain metadata-only and must not include secrets, raw headers, prompts, responses, request bodies, or payloads.
- Routing direction: keep approved/enabled health checks as the baseline, then exclude or de-prioritize risky control/trust combinations explicitly instead of silently ignoring the metadata.
- Do not mark Managed Node enrollment tokens, subscriber heartbeat, benchmark policy, or full consensus implementation complete in this slice.

### 2026-07-03T04:31:23Z

- Added routing policy classification for control/trust metadata.
- Default routing policy now excludes `external` trust from routing.
- Approved Passive Endpoints and `lan_unverified` nodes remain eligible for single-route requests but are de-prioritized and reasoned in metadata.
- Consensus eligibility now requires Managed Nodes with `local` or `lan_trusted` trust.
- Consensus aliases now enforce configured minimum participant counts before proxying.
- Routing decisions and no-eligible-node errors emit metadata-only candidate/exclusion details: node ID, control level, trust level, capability source, and policy reason codes.
- Strict/task session affinity now preserves policy eligibility by refusing to pin to a node outside the current candidate set.
- Updated UI/API docs, capability model docs, node control docs, and Phase B plan docs for the implemented routing/trust policy.
- Added focused tests for:
  - single-route de-prioritization of Passive Endpoint and `lan_unverified` candidates
  - default `external` exclusion
  - consensus exclusion of Passive Endpoints and `lan_unverified`
  - consensus minimum participant enforcement
  - session affinity exclusion guardrail
  - metadata-only route-boundary telemetry with prompt leak checks
- Verified `go test ./internal/routing ./internal/session ./internal/httpapi -run 'TestSelect|TestApplyAffinity|TestMarshalProxyEmitsMetadataOnlyRoutingPolicyReasons'`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Restarted the local service at `http://localhost:11435/ui/`.
- Verified live `/healthz`.
- Verified live bootstrap remains schema version `2`, reports the existing managed node as local/approved/enabled/managed, and preserves encrypted fallback status, weighted queue scheduling, and safe defaults.
- Verified live no-payload `local-consensus` request returns OpenAI-style `no_eligible_node` because the current fleet has fewer than the configured minimum consensus participants.
- Verified live support-bundle token-shaped leak probe returns no full admin/client token patterns or known secret markers.
- Verified served `/ui/`, `/ui/app.js`, and `/ui/app.css` are reachable and contain the current Nodes trust controls/static UI assets.
- Browser automation limitation: in-app browser control timed out on navigation and current-tab inspection during this slice, so no fresh visual DOM/browser-console pass was completed after the routing policy change.
- Scope not completed yet: enrollment tokens, subscriber heartbeat, benchmark policy, richer routing/operations UI explanation of policy exclusions, and full consensus execution.

### 2026-07-03T04:35:18Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: add enrollment token flow and marshal approval queue for Managed Node subscriber enrollment.
- Guardrail: preserve all Phase A hardening requirements, schema version 2 behavior, Passive Endpoint add-flow behavior, approval/revocation routing eligibility, trust-level update behavior, and routing control/trust policy behavior.
- Guardrail: keep raw enrollment tokens, admin tokens, client API keys, HEC tokens, raw headers, prompts, responses, request bodies, and payloads out of ordinary app state, APIs after one-time display, telemetry, and support bundles.
- Implementation direction: create one-time short-lived enrollment tokens for operator display, store only hashes and metadata in the enrollment queue, let subscribers register as pending Managed Nodes, and require operator approval before routing.
- Do not mark subscriber heartbeat, freshness expiration, benchmark policy, or routing/operations exclusion UI complete in this slice.

### 2026-07-03T04:42:33Z

- Added `POST /wrangler/enrollment-tokens` for short-lived one-time Managed Node enrollment tokens.
- Enrollment token creation stores token hash, token hint, expiry, control level, trust level, capability source, and approval state in the app-state enrollment queue.
- Raw enrollment tokens are returned only in the token creation response.
- Added `POST /subscriber/enroll` for token-based subscriber registration.
- Subscriber registration consumes the token hash, updates enrollment queue registration metadata, and creates a pending Managed Node with subscriber-reported capability metadata.
- Registered Managed Nodes remain `approval_state: pending`, `approved: false`, and excluded from routing until approved by the operator.
- Approval/revocation actions now mirror approval state back to matching enrollment queue entries.
- Bootstrap exposes sanitized enrollment queue metadata with token hashes removed.
- Support bundles continue to sanitize enrollment queue metadata and exclude token hashes.
- Nodes UI now includes Generate token controls, trust-level and TTL inputs, one-time-token warning copy, manual-add compatibility controls, and a sanitized Managed enrollment queue display.
- Manual subscriber add remains available for compatibility and still approves immediately; future work should migrate that into the enrollment flow or make the risk clearer.
- Updated UI/API docs, node control docs, support-bundle docs, and Phase B plan docs for the Managed Node enrollment-token flow.
- Added focused tests for enrollment token generation, hash-only storage, subscriber registration, invalid token rejection, pending Managed Node metadata, token consumption, bootstrap privacy, and support-bundle privacy.
- Verified `go test ./internal/httpapi -run 'TestManagedEnrollment|TestNodeTrust|TestNodeApprove|TestPassiveAddNode|TestMarshalProxyEmitsMetadataOnlyRoutingPolicyReasons|TestSupportBundle'`.
- Verified `go test ./internal/appstate ./internal/routing ./internal/session`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Restarted the local service at `http://localhost:11435/ui/`.
- Verified live `/healthz`.
- Verified live bootstrap remains schema version `2`, preserves the existing approved local Managed Node, reports an empty sanitized enrollment queue, and preserves encrypted fallback status and safe defaults.
- Verified live enrollment-token creation rejects credential-bearing subscriber URLs without mutating the real enrollment queue.
- Verified live subscriber enrollment rejects invalid/expired tokens without creating a node.
- Verified live support-bundle token/hash leak probe returns no enrollment tokens, 64-character token hashes, full admin/client token patterns, or known secret markers.
- Verified served `/ui/`, `/ui/app.js`, and `/ui/app.css` are reachable and include the new Managed Node enrollment controls.
- Browser automation limitation: in-app browser control timed out on navigation during this slice, so no fresh visual DOM/browser-console pass was completed after the enrollment UI change.
- Scope not completed yet: subscriber heartbeat, freshness expiration, node identity hardening beyond enrollment token proof, benchmark policy, routing/operations exclusion UI, and migration of manual-add compatibility into safer enrollment semantics.

### 2026-07-03T14:32:14Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: add subscriber registration heartbeat and freshness handling for Managed Nodes.
- Guardrail: preserve all Phase A hardening requirements, schema version 2 behavior, Passive Endpoint add-flow behavior, approval/revocation routing eligibility, trust-level update behavior, routing control/trust policy behavior, and Managed Node enrollment-token behavior.
- Guardrail: heartbeat telemetry, state, APIs, and support bundles must remain metadata-only and must not include secrets, raw headers, prompts, responses, request bodies, or payloads.
- Implementation direction: add a subscriber heartbeat endpoint that updates Managed Node freshness, reported health/model/load metadata, and timestamps; route only heartbeat-required Managed Nodes with fresh reports.
- Do not mark signed node identity, durable subscriber credentials, benchmark policy, or routing/operations exclusion UI complete in this slice.

### 2026-07-03T14:40:47Z

- Added `POST /subscriber/heartbeat` for metadata-only heartbeat updates from registered Managed Nodes.
- Heartbeats preserve existing approval state and trust level while refreshing subscriber-reported health, Ollama metadata, model inventory, tags, active jobs, queue depth, memory metadata, `last_reported_at`, and heartbeat freshness metadata.
- Registered subscribers now opt into `observed.heartbeat_required: true` during enrollment.
- Routing now excludes heartbeat-required Managed Nodes when `last_reported_at` is missing or older than the freshness window.
- Legacy/manual Managed Nodes without `observed.heartbeat_required: true` remain eligible under the existing approval, health, model, control, and trust policies.
- Routing exclusion metadata uses safe reason codes such as `heartbeat_missing` and `heartbeat_stale`.
- Nodes UI now surfaces a Freshness row showing report or heartbeat state without exposing request/response payloads.
- Updated UI/API docs, node-control docs, support-bundle docs/schema, capability model docs, and the Phase B plan for heartbeat and freshness behavior.
- Added focused tests for stale heartbeat routing exclusion, legacy Managed Node compatibility, heartbeat metadata refresh, and rejection of missing or Passive Endpoint heartbeats.
- Verified `go test ./internal/routing ./internal/httpapi -run 'TestSelect.*Heartbeat|TestSubscriberHeartbeat|TestManagedEnrollment'`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Verified live `/healthz`, schema version `2` bootstrap, missing-node heartbeat error path, served `/ui/`, served `/ui/app.js` freshness markers, and support-bundle token/secret leak probe.
- Verified in-app browser Nodes view shows the Freshness row and Generate token control, contains no token-shaped visible values, and reports no browser console errors.
- Scope not completed yet: signed heartbeat identity or durable subscriber credentials, benchmark policy, routing/operations exclusion UI, and migration of manual-add compatibility into safer enrollment semantics.

### 2026-07-03T14:43:13Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: add routing/operations UI warnings that explain why unapproved, passive, `lan_unverified`, `external`, missing-heartbeat, or stale-heartbeat nodes were excluded from routing or consensus.
- Guardrail: preserve all Phase A hardening requirements and Phase B schema, Passive Endpoint, approval/revocation, trust update, routing policy, enrollment-token, and heartbeat behavior.
- Guardrail: routing/operations warning metadata must remain metadata-only and must not include secrets, raw headers, prompts, responses, request bodies, endpoint credentials, API keys, HEC tokens, enrollment tokens, token hashes, or payloads.
- Implementation direction: expose a sanitized route policy status summary through existing UI/API metadata and render concise operator warnings in Dashboard/Nodes using existing control/trust/approval/freshness fields and routing reason codes.
- Do not mark signed heartbeat identity, durable subscriber credentials, benchmark policy, mDNS discovery, or manual-add hardening complete in this slice.

### 2026-07-03T14:48:32Z

- Added metadata-only `routing_policy_status` to `GET /wrangler/ui/bootstrap` and `GET /wrangler/metrics`.
- `routing_policy_status` includes a stable `window`, summary counts, and per-node warnings with node ID, severity, scope, code, message, control level, trust level, approval state, and capability source.
- Warning codes cover `node_not_approved`, `node_disabled`, `trust_external_excluded`, `passive_consensus_excluded`, `trust_lan_unverified_deprioritized`, `trust_lan_unverified_consensus_excluded`, `heartbeat_missing`, and `heartbeat_stale`.
- Dashboard and Nodes now render Routing Policy cards that explain current routing/consensus exclusions or show a healthy no-warning state.
- Empty warning sets now serialize as `warnings: []` for stable downstream UI/API consumption.
- Updated UI/API docs and Phase B plan docs for the routing policy status contract.
- Added focused tests for warning generation across unapproved, Passive Endpoint, `lan_unverified`, `external`, missing-heartbeat, and stale-heartbeat cases.
- Added focused tests that bootstrap and metrics expose routing policy status without prompt/header/token markers.
- Verified `go test ./internal/httpapi ./internal/routing -run 'TestSummarizeRoutingPolicy|TestBootstrapAndMetricsIncludeRoutingPolicyStatus|TestMarshalProxyEmitsMetadataOnlyRoutingPolicyReasons|TestSelect.*Heartbeat|TestSelect.*Consensus|TestSelect.*Control|TestSelect.*Trust'`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Verified live `/healthz`, bootstrap `routing_policy_status`, metrics `routing_policy_status`, served UI warning markers, and support-bundle token/secret/prompt leak probe.
- Verified in-app browser Dashboard and Nodes show the Routing Policy card, Nodes still shows Freshness, visible text has no token-shaped values, and browser console reports no errors.
- Scope not completed yet: signed heartbeat identity or durable subscriber credentials, benchmark policy, mDNS discovery, and migration of manual-add compatibility into safer enrollment semantics.

### 2026-07-03T14:51:56Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: harden manual subscriber add by moving it behind safer enrollment/approval semantics or stronger warnings.
- Guardrail: preserve all Phase A hardening requirements and Phase B schema, Passive Endpoint add-flow, approval/revocation, trust update, routing policy, enrollment-token, heartbeat, freshness, and routing-policy-warning behavior.
- Guardrail: manual subscriber hardening must keep secrets, raw headers, prompts, responses, request bodies, endpoint credentials, API keys, HEC tokens, enrollment tokens, token hashes, and payloads out of ordinary state, APIs, telemetry, support bundles, and UI policy metadata.
- Implementation direction: keep manual subscriber add as a compatibility path, but stop auto-approving it for routing; validate subscriber URLs consistently, create Managed Nodes in pending approval state, and make the UI warning explicit.
- Do not mark signed heartbeat identity, durable subscriber credentials, benchmark policy, mDNS discovery, or full Managed Node installer packaging complete in this slice.

### 2026-07-03T14:56:47Z

- Hardened `POST /wrangler/nodes/manual-add` as an approval-gated compatibility fallback.
- Manual subscriber add now validates subscriber URLs with the same safe absolute `http`/`https` URL rules as other endpoint-add flows.
- Manual subscriber add now rejects missing URLs, embedded credentials, unsupported schemes, and relative URLs.
- Manually added Managed Nodes now land with `approval_state: pending`, `approved: false`, and manual-add metadata instead of being immediately approved for routing.
- Manual capability probing continues to fetch metadata-only subscriber capabilities and does not forward client authorization headers.
- Manual-add telemetry is metadata-only and no longer emits the subscriber URL.
- Nodes UI warning copy now explains manual add is a compatibility fallback, did not use a one-time enrollment token, and still requires approval before routing.
- Updated UI/API docs, node-control docs, and Phase B plan docs for manual-add hardening.
- Added focused tests for pending manual Managed Node behavior, unsafe manual URL rejection, authorization-header non-forwarding during manual capability probes, and routing-policy warning visibility for pending manual nodes.
- Verified `go test ./internal/httpapi ./internal/routing -run 'TestManualSubscriberAdd|TestNodeApprove|TestManagedEnrollment|TestBootstrapAndMetricsIncludeRoutingPolicyStatus|TestSummarizeRoutingPolicy|TestSelect.*Approval|TestSelect.*Heartbeat'`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Verified live `/healthz`, schema version `2` bootstrap, `routing_policy_status`, served `/ui/app.js` manual-add warning markers, unsafe manual-add credential URL rejection, and support-bundle token/secret/prompt/credential leak probe.
- Browser automation limitation: in-app browser control timed out during two navigation/evaluation attempts, so no fresh visual DOM/browser-console pass was completed for this slice.
- Scope not completed yet: signed heartbeat identity or durable subscriber credentials, benchmark policy, mDNS discovery, and full Managed Node installer packaging.

### 2026-07-03T15:03:31Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: add node identity hardening for Managed Node heartbeats, such as a signed heartbeat or shared subscriber credential.
- Guardrail: preserve all Phase A hardening requirements and Phase B schema, Passive Endpoint add-flow, approval/revocation, trust update, routing policy, enrollment-token, heartbeat/freshness, routing-policy-warning, and manual-add hardening behavior.
- Guardrail: heartbeat identity must keep raw heartbeat credentials, enrollment tokens, token hashes, admin tokens, client API keys, HEC tokens, raw headers, prompts, responses, request bodies, endpoint credentials, and payloads out of ordinary app state, bootstrap, telemetry, support bundles, and UI warning metadata.
- Implementation direction: issue a one-time subscriber heartbeat credential during successful subscriber enrollment, store it only in the encrypted secret backend, store/report only safe auth metadata such as method and token hint, and require the credential on heartbeats for nodes that have one.
- Compatibility direction: legacy/manual Managed Nodes without a stored heartbeat credential should not be silently broken; they should remain identifiable as legacy/unverified heartbeat auth until they re-enroll or receive a credential-hardening flow.
- Do not mark benchmark policy, mDNS discovery, full Managed Node installer packaging, or asymmetric signed node identity complete in this slice.

### 2026-07-03T15:08:59Z

- Added shared-secret heartbeat identity hardening for token-enrolled Managed Nodes.
- Successful subscriber enrollment now derives a heartbeat credential from the one-time enrollment token and final node ID.
- The derived heartbeat credential is stored only in the configured encrypted secret backend under a node-ID-derived key.
- Enrollment responses include safe heartbeat auth metadata: method, header name, credential derivation ID, and token hint; they do not return the raw heartbeat credential.
- Node metadata records safe heartbeat auth fields such as `heartbeat_auth_method`, `heartbeat_auth_required`, `heartbeat_token_hint`, and `heartbeat_credential_derivation`.
- `POST /subscriber/heartbeat` now requires `X-Llama-Wrangler-Subscriber-Token` or `Authorization: Bearer` when a stored heartbeat credential exists for the Managed Node.
- Heartbeat auth failures return generic unauthorized errors and emit metadata-only auth-failure telemetry.
- Legacy/manual Managed Nodes without a stored heartbeat credential remain compatible and are marked `legacy_unverified` on heartbeat rather than silently broken.
- Nodes UI now shows safe Heartbeat auth status without exposing raw credentials.
- Updated UI/API docs, node-control docs, support-bundle docs, and Phase B plan docs for the shared-secret heartbeat contract.
- Added focused tests for enrollment credential derivation, encrypted-secret storage, bootstrap/support-bundle non-leakage, heartbeat rejection without/wrong credentials, successful credential-authenticated heartbeat, and legacy heartbeat compatibility.
- Verified `go test ./internal/httpapi -run 'TestManagedEnrollment|TestSubscriberHeartbeat|TestSupportBundle|TestBootstrapAndMetricsIncludeRoutingPolicyStatus'`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Verified live `/healthz`, schema version `2` bootstrap, `routing_policy_status`, served `/ui/app.js` heartbeat-auth markers, missing-node heartbeat error path, and support-bundle heartbeat/enrollment/admin/client token leak probe.
- Verified in-app browser Nodes view shows Heartbeat auth, Routing Policy, and manual-add warning text; visible text has no heartbeat/enrollment/admin/client token-shaped values, and browser console reports no errors.
- Scope not completed yet: asymmetric signed node identity, heartbeat credential rotation/re-provisioning for legacy/manual nodes, benchmark policy, mDNS discovery, and full Managed Node installer packaging.

### 2026-07-03T15:11:38Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: start benchmark policy wiring for Managed Nodes versus Passive Endpoints without implying Passive Endpoint local-control capabilities.
- Guardrail: preserve all Phase A hardening requirements and Phase B schema, Passive Endpoint add-flow, approval/revocation, trust update, routing policy, enrollment-token, heartbeat/freshness, heartbeat identity, routing-policy-warning, and manual-add hardening behavior.
- Guardrail: benchmark policy metadata must stay metadata-only and must not include secrets, raw headers, prompts, responses, request bodies, endpoint credentials, API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, or payloads.
- Implementation direction: expose benchmark policy status for nodes, allow benchmark queue actions only for eligible Managed Nodes, mark Passive Endpoints as marshal-observed/probe-only, and keep Passive Endpoint UI free of local benchmark controls.
- Do not mark full benchmark runner execution, benchmark-derived routing placement, mDNS discovery, or heartbeat credential rotation complete in this slice.

### 2026-07-03T15:19:08Z

- Completed the initial benchmark policy wiring slice.
- Added `benchmark_policy_status` for bootstrap and metrics with metadata-only node eligibility, control level, trust level, approval state, benchmark source, mode, reason codes, and operator message.
- Updated benchmark node actions so eligible Managed Nodes can queue subscriber-reported benchmark metadata, while Passive Endpoints are rejected with `benchmark_policy_rejected` and `passive_no_local_benchmark_control`.
- Updated Dashboard and Nodes UI surfaces to show benchmark policy/source metadata and to keep Passive Endpoint local benchmark controls hidden.
- Updated UI/API docs, node-control docs, capability model docs, and Phase B plan docs for Managed Node benchmark queue semantics and Passive Endpoint probe-only semantics.
- Added focused tests for Managed versus Passive benchmark policy summaries, benchmark action enforcement, bootstrap/metrics benchmark status, and redaction markers.
- Verified `go test ./internal/httpapi -run 'TestBenchmark|TestBootstrapAndMetricsIncludeBenchmarkPolicyStatus'`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Restarted the live standalone service and verified `/healthz`, schema version `2` bootstrap, `benchmark_policy_status` in bootstrap and metrics, served `/ui/app.js` benchmark policy markers, and support-bundle token/payload leak probe.
- Browser automation limitation: the in-app browser bridge timed out twice during visual UI verification, so a fresh DOM/console browser pass was not completed; API behavior and served UI assets were verified instead.
- Scope not completed yet: actual benchmark execution/result ingestion, benchmark-derived routing placement, Passive Endpoint marshal-observed benchmark probes, heartbeat credential rotation/re-provisioning, mDNS discovery, and full Managed Node installer packaging.

### 2026-07-03T22:06:52Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: add heartbeat credential rotation/re-provisioning for legacy/manual Managed Nodes.
- Guardrail: preserve all Phase A hardening requirements and Phase B schema, Passive Endpoint add-flow, approval/revocation, trust update, routing policy, enrollment-token, heartbeat/freshness, heartbeat identity, routing-policy-warning, manual-add hardening, and benchmark-policy behavior.
- Guardrail: heartbeat credential rotation must be an explicit admin action, store raw credentials only in the configured secret backend, return any newly generated credential only in the immediate rotation response, and expose only safe auth method/hint/provisioning metadata in state, UI, telemetry, support bundles, and API responses.
- Implementation direction: add a Managed Node credential re-provisioning endpoint and UI action for legacy/manual nodes, update node metadata to `shared_secret` after provisioning, require the new credential on subsequent heartbeats, and keep Passive Endpoints ineligible for heartbeat credential provisioning.
- Do not mark asymmetric signed node identity, automatic subscriber-side install updates, full installer packaging, mDNS discovery, or benchmark execution complete in this slice.

### 2026-07-03T22:11:31Z

- Completed the heartbeat credential rotation/re-provisioning slice for Managed Nodes.
- Added `POST /wrangler/nodes/:id/heartbeat-credential/rotate` for explicit admin rotation/provisioning of subscriber heartbeat shared secrets.
- Rotation generates a new `lw_hb_` credential, stores it only in the configured secret backend, returns it only in the immediate rotation response, and updates node metadata with safe auth method, token hint, derivation, provisioning timestamp/source, and re-provisioning-required status.
- Subsequent Managed Node heartbeats require the rotated credential; missing or old credentials are rejected with generic unauthorized responses.
- Successful credential-authenticated heartbeat clears `heartbeat_reprovisioning_required` and marks identity verified.
- Passive Endpoints are rejected for heartbeat credential provisioning.
- Nodes UI now shows Heartbeat provisioning status and a Rotate heartbeat credential action for Managed Nodes only; the one-time credential is shown through the immediate action response.
- Updated UI/API docs, node-control docs, support-bundle docs, and the Phase B plan for the credential rotation/re-provisioning contract.
- Added focused tests for legacy/manual Managed Node credential provisioning, raw credential non-leakage from node/bootstrap/support-bundle metadata, post-rotation heartbeat enforcement, old credential invalidation, and Passive Endpoint rejection.
- Verified `go test ./internal/httpapi -run 'TestManagedNodeHeartbeatCredentialRotation|TestSubscriberHeartbeatRequiresStoredCredential|TestManagedEnrollment|TestSupportBundle|TestBootstrapAndMetricsIncludeBenchmarkPolicyStatus'`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Restarted the live standalone service and verified `/healthz`, schema version `2` bootstrap, non-mutating unknown-node credential rotation response, served `/ui/app.js` heartbeat credential rotation markers, and support-bundle token/payload leak probe.
- Verified in-app browser Nodes view shows Heartbeat provisioning and Rotate heartbeat credential controls, with no token-shaped secret text and no console errors. The live rotate action was intentionally not clicked to avoid mutating the real local node credential.
- Scope not completed yet: asymmetric signed node identity, automatic subscriber-side credential install, mDNS discovery, full Managed Node installer packaging, actual benchmark execution/result ingestion, and benchmark-derived routing placement.

### 2026-07-03T22:20:15Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: add mDNS discovery planning or implementation while keeping subnet scanning opt-in only.
- Guardrail: preserve all Phase A hardening requirements and Phase B schema, Passive Endpoint add-flow, approval/revocation, trust update, routing policy, enrollment-token, heartbeat/freshness, heartbeat identity, heartbeat credential rotation, routing-policy-warning, manual-add hardening, and benchmark-policy behavior.
- Guardrail: peer discovery must be explicit operator action only, metadata-only, and must not create routable nodes, approve endpoints, store secrets, transmit prompt/response payloads, or perform background/aggressive subnet scanning.
- Implementation direction: replace the placeholder peer-discovery endpoint with a conservative discovery status/result contract that documents mDNS/Bonjour intent, records subnet scanning as disabled unless a future explicit opt-in exists, and routes operators toward Managed Node enrollment or Passive Endpoint add flows for actual adoption.
- Do not mark automatic LAN enrollment, routable peer creation, subnet scanning, mDNS package-level network probing, full Managed Node installer packaging, or benchmark execution complete in this slice unless implemented and verified.

### 2026-07-03T22:25:43Z

- Completed the safe peer discovery slice.
- Replaced the placeholder `POST /wrangler/setup/discover-peers` response with an explicit operator-initiated mDNS discovery status contract.
- Added one-shot bounded mDNS/Bonjour PTR queries for `_llama-wrangler._tcp.local.`, `_llama-wrangler-subscriber._tcp.local.`, and `_ollama._tcp.local.` with a short timeout.
- Added DNS response parsing for PTR/SRV/A/AAAA metadata and candidate projection.
- Discovery candidates are review-only metadata with safe IDs, service/instance/host/port/address metadata when advertised, `approval_state: not_added`, default `trust_level: lan_unverified`, and adoption path guidance.
- Discovery does not persist candidates as nodes, approve candidates, route traffic to candidates, send payloads, or perform subnet scanning.
- Subnet scanning is explicitly returned as `disabled_requires_future_explicit_opt_in`.
- Added First-Run Setup UI Safe LAN discovery card and Peer Discovery Results panel with candidate count, mDNS status, subnet scan status, and manual adoption warning.
- Updated UI/API docs, first-run wizard docs, node-control docs, and Phase B plan docs for safe mDNS discovery and subnet-scan non-goals.
- Added focused tests for mDNS candidate parsing, metadata-only candidate defaults, forbidden marker checks, endpoint response shape, subnet-scan disabled status, and no node persistence during discovery.
- Verified `go test ./internal/httpapi -run 'TestMDNS|TestDiscoverPeers'`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Restarted the live standalone service and verified `/healthz`, live `discover-peers` response shape, no node count mutation before/after discovery, served `/ui/app.js` discovery markers, and support-bundle token/payload leak probe.
- Live mDNS probe completed with no candidates in the current environment; this is acceptable and recorded as `mdns.status: no_candidates_found`.
- Verified in-app browser setup view shows Safe LAN discovery, clicking Discover peers renders Peer Discovery Results, subnet scan status is visible, no token-shaped secret text appears, and browser console reports no errors.
- Scope not completed yet: automatic LAN enrollment, routable peer creation from discovery candidates, subnet scanning opt-in flow, full Managed Node installer packaging, actual benchmark execution/result ingestion, and benchmark-derived routing placement.

### 2026-07-03T22:27:02Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: start actual benchmark execution/result ingestion for Managed Nodes and marshal-observed Passive Endpoint probes.
- Guardrail: preserve all Phase A hardening requirements and Phase B schema, Passive Endpoint add-flow, approval/revocation, trust update, routing policy, enrollment-token, heartbeat/freshness, heartbeat identity, heartbeat credential rotation, routing-policy-warning, manual-add hardening, benchmark-policy behavior, and safe-discovery behavior.
- Guardrail: benchmark execution/result ingestion must remain metadata-only and must not store or return prompts, responses, request bodies, raw headers, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, or payloads.
- Managed Node direction: accept authenticated subscriber-reported benchmark results and store summarized metrics as node metadata; benchmark queue action can mark a managed benchmark request as queued/requested, but full remote subscriber job orchestration may remain future work unless implemented.
- Passive Endpoint direction: allow only marshal-observed probes such as safe `/api/tags` latency/availability measurement; do not imply hardware inspection, local benchmark execution, warm-state inspection, load inspection, or model-management control.
- Do not mark benchmark-derived routing placement, full benchmark scheduler, long-running job management, prompt workload benchmark suites, or Passive Endpoint local-control capabilities complete in this slice unless implemented and verified.

### 2026-07-03T22:35:27Z

- Completed the initial benchmark execution/result ingestion slice.
- Added `POST /subscriber/benchmarks` for authenticated Managed Node subscriber-reported benchmark results.
- Managed benchmark ingestion stores bounded metadata-only result summaries in node observation metadata, including benchmark ID, source, status, model, timing, token counts, token rates, completion time, and safe error code.
- Managed benchmark ingestion updates matching model token-rate metadata when a safe subscriber-reported rate is present.
- Managed benchmark ingestion ignores prompt/response/raw-payload style fields and does not store or return prompts, responses, raw headers, request bodies, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, or payloads.
- Existing Managed Node benchmark actions now create a safe benchmark ID and continue to represent a queued/requested subscriber-reported benchmark until full subscriber job orchestration exists.
- Added `POST /wrangler/nodes/:id/benchmark-probe` for Passive Endpoint marshal-observed `/api/tags` availability/latency probes.
- Passive Endpoint probes store source, mode, status, duration, model count, and safe error code only; they do not enable local hardware inspection, warm-state inspection, load inspection, model-management control, or prompt workload execution.
- Existing `/wrangler/nodes/:id/benchmark` behavior still rejects Passive Endpoints with the explicit passive no-local-control reason.
- Nodes UI now shows Benchmark result summaries and renders `Run benchmark` for Managed Nodes versus `Probe /api/tags` for Passive Endpoints.
- Tightened support-bundle string redaction for heartbeat/enrollment token prefixes, OpenAI-style `sk-` values, and bearer authorization markers.
- Updated UI/API docs, node-control docs, support-bundle schema docs, and Phase B plan docs for Managed Node benchmark result ingestion and Passive Endpoint probe semantics.
- Added focused tests for Managed Node benchmark result ingestion, subscriber benchmark authentication, metadata-only storage, bootstrap/support-bundle privacy, and Passive Endpoint marshal-observed `/api/tags` probe behavior.
- Verified `go test ./internal/httpapi -run 'TestSubscriberBenchmark|TestPassiveBenchmark|TestBenchmark'`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Restarted the live standalone service and verified `/healthz`.
- Verified live `/subscriber/benchmarks` missing-node probe returns safe `node not found` without echoing submitted prompt-like fields.
- Verified live served `/ui/app.js` contains Benchmark result, Run benchmark, Passive Endpoint `/api/tags` probe, and benchmark result summary markers.
- Verified live bootstrap remains schema version `2`, preserves existing benchmark policy metadata, and keeps encrypted fallback/safe-default behavior intact.
- Verified live support-bundle leak probe returns no heartbeat/enrollment/admin/client token-shaped values, 64-character token hashes, authorization header markers, OpenAI-style `sk-` markers, prompt markers, response markers, or known secret markers.
- Browser automation limitation: the in-app browser harness timed out twice while checking the Nodes view, so this slice uses direct UI asset/API/service verification instead of claiming a fresh visual DOM/browser-console pass.
- Scope not completed yet: full Managed Node benchmark job orchestration between marshal and subscriber, subscriber-side benchmark runner implementation, long-running benchmark job management, benchmark-derived routing placement, prompt workload benchmark suites, and Passive Endpoint local-control capabilities.

### 2026-07-03T22:38:36Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: add subscriber-side credential installation/update guidance or packaging hooks for rotated heartbeat credentials.
- Guardrail: preserve all Phase A hardening requirements and Phase B schema, Passive Endpoint add-flow, approval/revocation, trust update, routing policy, enrollment-token, heartbeat/freshness, heartbeat identity, heartbeat credential rotation, routing-policy-warning, manual-add hardening, benchmark-policy behavior, safe-discovery behavior, and benchmark-ingestion behavior.
- Guardrail: guidance/hooks must not persist raw heartbeat credentials in ordinary app state, bootstrap, telemetry, support bundles, UI node metadata, docs examples, or API responses beyond the existing immediate rotation response.
- Implementation direction: make rotation output more operationally useful by returning safe installation metadata and copyable subscriber-side commands/templates that use placeholder or runtime-provided credential values, plus UI docs that explain env-var/config-file service usage and re-provisioning verification.
- Do not mark full Managed Node installer packaging, remote subscriber config mutation, background credential sync, asymmetric signed node identity, or benchmark job orchestration complete in this slice unless implemented and verified.

### 2026-07-03T22:44:39Z

- Completed the subscriber-side rotated heartbeat credential install guidance slice.
- Added `registration.heartbeat_credential_env` and env resolution for subscriber configs; the documented env var is `LLAMA_WRANGLER_SUBSCRIBER_HEARTBEAT_CREDENTIAL`.
- Updated `configs/subscriber.example.yaml` to reference the subscriber heartbeat credential env var instead of placing a plaintext credential in YAML.
- Extended heartbeat credential rotation responses with `subscriber_install` guidance metadata including credential hint, header name, env var, config key, config snippet, placeholder shell export command, placeholder launchd dry-run command, placeholder heartbeat verification command, restart notes, and warnings.
- Kept `subscriber_install` placeholder-based so it does not duplicate the raw rotated credential; the raw credential remains only the existing immediate `credential` response field.
- Updated the Nodes UI to show a one-time Subscriber Credential Install card after rotation, with copy actions for the immediate-response credential and credential-substituted shell/launchd/heartbeat commands while avoiding raw credential display in node cards.
- Updated UI/API docs, node-control docs, service-wrapper dry-run docs, Phase B plan docs, and subscriber example config for the install/update guidance path.
- Added focused tests for subscriber heartbeat credential env-var resolution, rotation response install-plan metadata, placeholder command behavior, launchd dry-run guidance, heartbeat verification guidance, raw credential non-duplication, and bootstrap/support-bundle non-leakage preservation.
- Verified `go test ./internal/config`.
- Verified `go test ./internal/httpapi -run 'TestManagedNodeHeartbeatCredentialRotation|TestSubscriberHeartbeatRequiresStoredCredential|TestManagedEnrollment|TestSubscriberBenchmark|TestSupportBundle'`.
- Verified `go test ./internal/servicewrap`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Restarted the live standalone service and verified `/healthz`.
- Verified live served `/ui/app.js` contains Subscriber Credential Install, copy actions, env-var guidance, and `subscriber_install` handling.
- Verified live missing-node heartbeat credential rotation returns safe `node not found` without mutating real node credentials.
- Verified live support-bundle leak probe returns no heartbeat/enrollment/admin/client token-shaped values, OpenAI-style `sk-` markers, authorization markers, prompt markers, or placeholder install commands; safe env-var/config-key names may appear in sanitized config.
- Scope not completed yet: remote subscriber config mutation, automatic subscriber-side credential sync, full Managed Node installer packaging, asymmetric signed node identity, full Managed Node benchmark job orchestration, and benchmark-derived routing placement.

### 2026-07-04T13:05:17Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: add full Managed Node benchmark job orchestration between marshal and subscriber.
- Guardrail: preserve all Phase A hardening requirements and Phase B schema, Passive Endpoint add-flow, approval/revocation, trust update, routing policy, enrollment-token, heartbeat/freshness, heartbeat identity, heartbeat credential rotation, subscriber credential install guidance, routing-policy-warning, manual-add hardening, benchmark-policy behavior, safe-discovery behavior, and benchmark-ingestion behavior.
- Guardrail: benchmark orchestration must remain metadata-only and must not store or return prompts, responses, request bodies, raw headers, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, benchmark payloads, or subscriber secrets in app state, bootstrap, telemetry, support bundles, UI metadata, or API responses.
- Managed Node direction: convert the existing queued/requested benchmark action into an explicit marshal-created benchmark job request that eligible Managed Nodes can poll, accept, run locally, and complete by posting sanitized benchmark result metadata.
- Subscriber direction: expose subscriber job polling and status/update endpoints protected by the same subscriber heartbeat credential semantics when a stored credential exists.
- Passive Endpoint direction: keep Passive Endpoints limited to marshal-observed `/api/tags` probes only; do not add prompt workload jobs, local hardware inspection, warm-state inspection, local load inspection, model management, or local benchmark execution for Passive Endpoints.
- Do not mark benchmark-derived routing placement, prompt workload suites, durable background scheduler, automatic retry/timeout management, or real subscriber runner implementation complete unless implemented and verified.

### 2026-07-04T13:11:05Z

- Completed the Managed Node benchmark job orchestration slice.
- Converted `POST /wrangler/nodes/:id/benchmark` for eligible Managed Nodes into a metadata-only benchmark job creation action returning `status: queued`, `node`, and safe `job` metadata.
- Added `POST /subscriber/benchmarks/claim` so authenticated Managed Node subscribers can claim the next queued job and move it to `running`.
- Added `POST /subscriber/benchmarks/status` so authenticated Managed Node subscribers can update job lifecycle state with `queued`, `running`, `completed`, `failed`, or `cancelled` plus safe error-code metadata.
- Updated `POST /subscriber/benchmarks` result ingestion so a result with a matching `benchmark_id` completes the matching job and keeps bounded result metadata behavior intact.
- Added bounded `observed.benchmark_jobs` metadata with benchmark ID, node ID, job type, source, mode, status, requested/claimed/completed/updated timestamps, model candidates, and subscriber endpoint hints.
- Nodes UI now shows a Benchmark job row alongside Benchmark result.
- Passive Endpoints remain limited to `POST /wrangler/nodes/:id/benchmark-probe` marshal-observed `/api/tags` probes; `POST /wrangler/nodes/:id/benchmark` and subscriber job claim paths do not create Passive Endpoint local-control jobs.
- Updated UI/API docs, node-control docs, support-bundle docs, and Phase B plan docs for the benchmark job lifecycle.
- Added focused tests for Managed Node benchmark job creation, credentialed job claim, job status update, result completion, no-job claim behavior, credential enforcement, Passive Endpoint job rejection, and job metadata non-leakage.
- Verified `go test ./internal/httpapi -run 'TestManagedBenchmarkJob|TestBenchmarkJob|TestSubscriberBenchmark|TestPassiveBenchmark|TestBenchmark'`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Verified `node -c internal/httpapi/static/app.js`.
- Restarted the live standalone service and verified `/healthz`.
- Verified live served `/ui/app.js` includes Benchmark job UI markers.
- Verified live `POST /subscriber/benchmarks/claim` and `POST /subscriber/benchmarks/status` missing-node paths return safe `node not found` responses without echoing submitted prompt-like markers.
- Verified live support-bundle leak probe returns no heartbeat/enrollment/admin/client token-shaped values, OpenAI-style `sk-` markers, authorization markers, prompt markers, response markers, or known secret markers.
- Verified in-app browser Nodes view shows Benchmark job and Benchmark result rows, no token-shaped visible values, and no browser console errors.
- Scope not completed yet: benchmark-derived routing placement, prompt workload benchmark suites, durable background benchmark scheduler, automatic benchmark retry/timeout management, real subscriber-side benchmark runner packaging, and Passive Endpoint local-control benchmark capabilities.

## Next Recommended Work

1. Add benchmark-derived routing placement policy using only approved, fresh, trusted, metadata-only benchmark summaries.
2. Add subscriber packaging/installer follow-through for applying env-var credentials to real service wrappers without remote mutation.
3. Add durable benchmark scheduler/retry/timeout controls for Managed Node jobs without adding payload storage.

### 2026-07-04T13:45:41Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: add benchmark-derived routing placement policy using only approved, fresh, trusted, metadata-only benchmark summaries.
- Guardrail: preserve all Phase A hardening requirements and Phase B schema, Passive Endpoint add-flow, approval/revocation, trust update, routing policy, enrollment-token, heartbeat/freshness, heartbeat identity, heartbeat credential rotation, subscriber credential install guidance, routing-policy-warning, manual-add hardening, benchmark-policy behavior, safe-discovery behavior, benchmark-ingestion behavior, and benchmark-job-orchestration behavior.
- Guardrail: benchmark placement must remain metadata-only and must not store or expose prompts, responses, request bodies, raw headers, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, or subscriber secrets in app state, bootstrap, telemetry, support bundles, UI metadata, or API responses.
- Placement direction: apply benchmark ranking only after existing approval, heartbeat freshness, control-level, trust-level, and model eligibility gates; ignore Passive Endpoint marshal-observed probes for local-control benchmark placement and keep those endpoints eligible only under existing single-route policy.
- Do not mark Phase B complete, full benchmark scheduling, prompt workload benchmark suites, or Passive Endpoint local-control benchmark capabilities complete in this slice unless implemented and verified.

### 2026-07-04T13:53:20Z

- Completed the benchmark-derived routing placement policy slice.
- Added routing placement ranking for otherwise eligible candidates using fresh subscriber-reported Managed Node benchmark summaries.
- Placement remains after normal routing gates: nodes must already pass approval, enabled/status, heartbeat freshness when required, control-level, trust-level, and model eligibility checks before benchmark summaries can affect ordering.
- Added a 24-hour benchmark placement freshness window and explicit placement reason codes for applied, fresh, passive probe ignored, untrusted ignored, source ignored, missing summary, stale summary, incomplete status, missing freshness, and missing token-rate metadata.
- Passive Endpoint marshal-observed `/api/tags` probe metadata remains ignored for benchmark placement and continues to avoid implying local-control benchmark capabilities.
- `lan_unverified` and `external` benchmark summaries do not influence placement; `lan_unverified` nodes retain their existing approved single-route behavior and consensus exclusion behavior.
- Extended `benchmark_policy_status` in bootstrap and metrics with placement eligibility, placement freshness window, placement reason codes, and placement operator messages.
- Updated Dashboard and Nodes UI to surface placement-ready counts and per-node benchmark placement posture.
- Updated UI/API docs and Phase B planning docs for metadata-only benchmark-derived placement behavior.
- Added focused routing tests for fresh trusted benchmark preference, Passive Endpoint probe ignored behavior, and untrusted/stale summary ignored behavior.
- Added focused HTTP policy tests for placement-ready and stale placement-limited benchmark policy status.
- Verified `go test ./internal/routing`.
- Verified `go test ./internal/httpapi -run 'TestSummarizeBenchmarkPolicy|TestBootstrapAndMetricsIncludeBenchmarkPolicyStatus|TestManagedBenchmarkJob|TestBenchmarkJob|TestSubscriberBenchmark|TestPassiveBenchmark|TestBenchmark'`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Verified `node -c internal/httpapi/static/app.js`.
- Restarted the live standalone service with the refreshed binary and verified `/healthz`.
- Verified live `/wrangler/metrics` includes `benchmark_policy_status`, `placement_eligible`, and benchmark placement reason metadata.
- Verified live served `/ui/app.js` includes placement-ready summary and Benchmark placement UI markers.
- Verified live support-bundle export did not expose heartbeat/enrollment/admin/client token-shaped values, OpenAI-style `sk-` markers, authorization markers, or known secret markers; support-bundle privacy metadata still intentionally lists redacted field names such as `prompt`.
- Verified in-app browser Nodes view shows Benchmark placement, Dashboard shows Placement-ready summaries, no visible token-shaped values were present, and no browser console errors were reported.
- Scope not completed yet: durable benchmark scheduler/retry/timeout controls, subscriber packaging/installer follow-through, prompt workload benchmark suites, full multi-model placement tuning, automatic placement freshness remediation, and Passive Endpoint local-control benchmark capabilities.

## Next Recommended Work

1. Add durable Managed Node benchmark scheduler/retry/timeout controls that operate on metadata-only benchmark jobs and do not store payloads.
2. Add subscriber packaging/installer follow-through for applying env-var heartbeat credentials to real service wrappers without remote mutation.
3. Add prompt workload benchmark suite definitions with safe synthetic prompts or operator-provided local-only fixtures, keeping prompts and responses out of persisted state and support bundles.

### 2026-07-04T13:56:04Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: add durable Managed Node benchmark scheduler/retry/timeout controls that operate on metadata-only benchmark jobs and do not store payloads.
- Guardrail: preserve all Phase A hardening requirements and Phase B Managed Node versus Passive Endpoint behavior, including schema version 2, encrypted fallback secrets, enrollment, heartbeat identity, credential rotation, Passive Endpoint add/probe-only semantics, routing policy warnings, benchmark ingestion, benchmark job orchestration, and benchmark-derived placement behavior.
- Guardrail: scheduler metadata must not store or expose prompts, responses, request bodies, raw headers, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, subscriber secrets, or benchmark payloads in app state, bootstrap, telemetry, support bundles, UI metadata, or API responses.
- Scheduler direction: keep jobs durable in node observed metadata; add bounded attempt, timeout, retry, deadline, next-attempt, and reason metadata; expose operator-visible scheduler posture without adding Passive Endpoint local-control jobs.
- Do not mark subscriber-side runner packaging, prompt workload suites, automatic model-specific benchmark tuning, or Passive Endpoint local-control benchmark capabilities complete in this slice unless implemented and verified.

### 2026-07-04T14:02:36Z

- Completed the durable Managed Node benchmark scheduler/retry/timeout controls slice.
- Added scheduler policy `bounded_retry_timeout_v1` for metadata-only Managed Node benchmark jobs.
- New benchmark jobs now persist attempt count, max attempts, lease timeout seconds, retry delay seconds, next attempt timestamp, timeout timestamp, scheduler state, and scheduler reason.
- Subscriber job claims reconcile that node first, claim only due queued jobs, increment attempts, and set running lease timeouts.
- Subscriber failed status updates move jobs into bounded `retry_wait` metadata until `next_attempt_at`; due retry-wait jobs reconcile back to queued.
- Timed-out running jobs reconcile into retry-wait while attempts remain and into exhausted failed state when max attempts are reached.
- Completed and cancelled jobs are marked terminal; result ingestion also marks matching jobs terminal through safe metadata.
- Added admin endpoint `POST /wrangler/benchmarks/scheduler/reconcile` for operator-triggered reconciliation and recovery.
- Added `benchmark_scheduler` to bootstrap and metrics with policy, summary counts, and safe per-job scheduler metadata.
- Added Dashboard Benchmark Scheduler card with jobs, claimable, retry-wait, timeout-due, exhausted counts, and Reconcile scheduler control.
- Nodes UI benchmark job row now includes scheduler state and attempt count when present.
- Passive Endpoints remain outside local-control benchmark jobs and scheduler reconciliation; `/api/tags` probes remain marshal-observed only.
- Updated UI/API docs and Phase B planning docs for scheduler metadata, retry, timeout, and reconcile behavior.
- Added focused tests for claim lease metadata, retry-wait failure handling, due retry reconciliation, timeout exhaustion, scheduler status, manual reconcile response, and metadata-only leak protection.
- Verified `go test ./internal/httpapi -run 'TestBenchmarkJobScheduler|TestBenchmarkScheduler|TestManagedBenchmarkJob|TestBenchmarkJob|TestSubscriberBenchmark|TestPassiveBenchmark|TestBenchmark'`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Verified `node -c internal/httpapi/static/app.js`.
- Restarted the live standalone service with the refreshed binary and verified `/healthz`.
- Verified live `/wrangler/metrics` includes `benchmark_scheduler`, `bounded_retry_timeout_v1`, and empty safe job summary metadata.
- Verified live `POST /wrangler/benchmarks/scheduler/reconcile` returns safe summary counts and scheduler status.
- Verified live served `/ui/app.js` includes Benchmark Scheduler, Reconcile scheduler, and attempt display markers.
- Verified live support-bundle leak probe did not expose heartbeat/enrollment/admin/client token-shaped values, OpenAI-style `sk-` markers, authorization markers, or known secret markers; support-bundle privacy metadata still intentionally lists redacted field names such as `prompt`.
- Verified in-app browser Dashboard shows Benchmark Scheduler, Bounded Retry Timeout V1, Reconcile scheduler, no visible token-shaped values, and no browser console errors.
- Scope not completed yet: subscriber packaging/installer follow-through, real subscriber-side benchmark runner packaging, prompt workload benchmark suites, configurable scheduler policy controls, automatic background scheduler tick, automatic placement freshness remediation, and Passive Endpoint local-control benchmark capabilities.

## Next Recommended Work

1. Add subscriber packaging/installer follow-through for applying env-var heartbeat credentials to real service wrappers without remote mutation.
2. Add prompt workload benchmark suite definitions with safe synthetic prompts or operator-provided local-only fixtures, keeping prompts and responses out of persisted state and support bundles.
3. Add configurable benchmark scheduler policy controls for max attempts, lease timeout, and retry delay while preserving safe defaults and metadata-only behavior.

### 2026-07-04T14:07:18Z

- Continuing Phase B: Node Enrollment and Discovery.
- Active work item: add subscriber packaging/installer follow-through for applying env-var heartbeat credentials to real service wrappers without remote mutation.
- Guardrail: preserve all Phase A hardening requirements and Phase B Managed Node versus Passive Endpoint behavior, including schema version 2, encrypted fallback secrets, enrollment, heartbeat identity, credential rotation, Passive Endpoint add/probe-only semantics, routing policy warnings, benchmark ingestion, benchmark job orchestration, benchmark-derived placement, and durable scheduler behavior.
- Guardrail: packaging/install metadata must not store or expose raw heartbeat credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, token hashes, endpoint credentials, prompts, responses, request bodies, raw headers, subscriber secrets, or benchmark payloads in app state, bootstrap, telemetry, support bundles, UI metadata, or API responses.
- Implementation direction: extend the existing immediate rotation response with placeholder-based service wrapper artifacts and commands for env-file/launchd usage, plus UI docs/copy affordances that substitute the one-time credential only in browser memory.
- Do not remotely mutate subscriber machines, write credentials into config files, or imply Passive Endpoint subscriber control in this slice.

### 2026-07-04T14:12:34Z

- Completed the subscriber packaging/installer follow-through slice for applying env-var heartbeat credentials to real service wrappers without remote mutation.
- Extended the one-time `subscriber_install` rotation response with placeholder-only env-file template metadata, launchd service-wrapper metadata, launchd plist template, install commands, validation commands, and uninstall commands.
- Kept `LLAMA_WRANGLER_SUBSCRIBER_HEARTBEAT_CREDENTIAL` as the documented env-var hook and `registration.heartbeat_credential_env` as the subscriber config reference.
- Launchd plist templates carry the credential placeholder through `EnvironmentVariables`; the raw rotated credential remains only in the top-level immediate rotation response.
- Nodes UI Subscriber Credential Install card now exposes copy actions for env file, launchd plist, install commands, validation commands, and uninstall commands, with one-time credential substitution only in browser memory.
- Install artifacts are manual operator guidance for the subscriber host; the marshal still does not remotely write subscriber configs, install launchd plists, mutate service wrappers, or start subscriber services.
- Passive Endpoints remain outside subscriber credential provisioning and service-wrapper guidance.
- Updated UI/API docs, service-wrapper dry-run docs, node-control docs, and Phase B plan docs for the new artifacts and non-mutating boundary.
- Added focused assertions that service-wrapper install artifacts are present, placeholder-only, and do not duplicate raw heartbeat credentials.
- Verified `go test ./internal/httpapi -run 'TestManagedNodeHeartbeatCredentialRotation|TestSubscriberHeartbeatRequiresStoredCredential|TestManagedEnrollment|TestSupportBundle'`.
- Verified `go test ./...`.
- Verified `go build ./cmd/llama-wrangler`.
- Verified `node -c internal/httpapi/static/app.js`.
- Restarted the live standalone service with the refreshed binary and verified `/healthz`.
- Verified live served `/ui/app.js` includes Copy env file, Copy launchd plist, Copy install commands, Copy validation commands, Copy uninstall commands, and service-wrapper markers.
- Verified live bootstrap does not expose `subscriber_install`, service-wrapper artifacts, heartbeat/enrollment/admin/client token-shaped values, OpenAI-style `sk-` markers, authorization markers, or known secret markers.
- Verified live support-bundle export does not expose `subscriber_install`, service-wrapper artifacts, heartbeat/enrollment/admin/client token-shaped values, OpenAI-style `sk-` markers, or known secret markers; support-bundle privacy metadata still intentionally lists redacted field names such as `authorization` and `prompt`.
- Verified in-app browser UI loads with the refreshed app shell, no visible token-shaped values, and no browser console errors.
- Scope not completed yet: real signed/package installer distribution, remote subscriber config mutation, automatic subscriber-side credential sync, prompt workload benchmark suites, configurable scheduler policy controls, automatic background scheduler tick, and Passive Endpoint local-control benchmark capabilities.

## Next Recommended Work

1. Add prompt workload benchmark suite definitions with safe synthetic prompts or operator-provided local-only fixtures, keeping prompts and responses out of persisted state and support bundles.
2. Add configurable benchmark scheduler policy controls for max attempts, lease timeout, and retry delay while preserving safe defaults and metadata-only behavior.
3. Add optional background scheduler tick for benchmark job reconciliation with operator-visible enablement and metadata-only telemetry.

### 2026-07-09T13:00:14Z

- Continuing Phase B planning after intake of two additive documents:
  - `docs/llama_wrangler_additive_managed_passive_nodes_and_product_refinements.md`
  - `docs/llama_wrangler_additive_future_capability_endpoints.md`
- Active work item: incorporate these documents into the ledger and tasking documentation, adjust plan ordering where needed, and begin only low-risk V1/V2 compatibility documentation work.
- Decision: the Managed Node versus Passive Endpoint requirements are already the active Phase B product direction and remain first-class.
- Decision: future Capability Endpoints are binding architecture direction for V2, but they must not derail V1 or trigger premature GitHub, Xcode, Docker, CI/CD, Codex-agent, build-runner, plugin, marketplace, or arbitrary tool-execution implementation.
- Guardrail: keep current implementation Ollama-first and UI-first, with Managed Nodes and Passive Endpoints as the concrete V1 control model.
- Guardrail: do not alter runtime state schema or APIs solely for speculative future endpoint types unless current V1 code blocks later extensibility.
- Guardrail: preserve all Phase A hardening and Phase B metadata-only, secret-exclusion, support-bundle, routing, benchmark, enrollment, heartbeat, trust, and approval behavior.

### 2026-07-09T13:03:18Z

- Completed the additive product/future documentation intake slice.
- Added `docs/19_capability_endpoint_future_plan.md` as the V1/V2 boundary document for future Capability Endpoints.
- Updated the ledger binding inputs so both new additive documents are tracked as product direction.
- Updated architecture, capability model, UI/API docs, node-control docs, MVP roadmap, and Phase B planning docs to make this decision explicit:
  - V1 remains an Ollama-first, local-first fleet control plane.
  - Managed Nodes and Passive Endpoints remain the concrete Phase B control model.
  - future Capability Endpoints are a V2 direction after the V1 foundation is functional.
  - no GitHub, Xcode, Docker, CI/CD, Codex-agent, build-runner, marketplace, plugin, or arbitrary tool-execution surfaces should be implemented prematurely.
- Compatibility assessment: no runtime state schema or API changes are required solely for these future documents right now. The current schema version 2 control/trust/capability-source/approval/freshness metadata, metadata-only telemetry posture, and support-bundle redaction boundaries already keep the future endpoint path open enough for V1.
- Intentionally deferred:
  - generic `endpoint_type` migration
  - `/wrangler/endpoints` compatibility layer
  - non-Ollama runtime/provider integrations
  - generic non-inference workflow routing
  - future automation/tool execution surfaces
- Verified `go test ./...`.
- Verified `go build -o ./llama-wrangler ./cmd/llama-wrangler`.
- Restarted standalone service and verified live `GET /healthz`.
- Verified live `GET /ui/` serves the app shell.
- Verified live bootstrap still reports `schema_version: 2`.
- Verified `git diff --check`.
- Scope not completed yet: prompt workload benchmark suite definitions, configurable scheduler policy controls, optional background scheduler tick, model lifecycle/warm-state V1 surfaces, consensus completion, Splunk dashboard expansion, Frontier Delta completion, and V2 Capability Endpoint implementation.

## Next Recommended Work

1. Add prompt workload benchmark suite definitions with safe synthetic prompts or operator-provided local-only fixtures, keeping prompts and responses out of persisted state and support bundles.
2. Add configurable benchmark scheduler policy controls for max attempts, lease timeout, and retry delay while preserving safe defaults and metadata-only behavior.
3. Add optional background scheduler tick for benchmark job reconciliation with operator-visible enablement and metadata-only telemetry.
4. Keep future Capability Endpoint work documentation-only until V1 acceptance criteria are met, unless a current V1 implementation detail clearly blocks later additive expansion.

### 2026-07-09T13:04:55Z

- Continuing Phase B benchmark foundations.
- Active work item: add prompt workload benchmark suite definitions with safe synthetic prompts or operator-provided local-only fixtures.
- Guardrail: benchmark workload suites must remain metadata-only in persisted app state, bootstrap, metrics, telemetry, UI node metadata, support bundles, and scheduler jobs.
- Guardrail: prompt text, response text, raw request bodies, raw headers, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, future provider keys, and payloads must not be stored or returned.
- Guardrail: safe synthetic suites may be referenced by stable IDs and metadata; operator-provided local fixtures may be referenced by local-only fixture IDs or paths, but fixture contents must stay on the subscriber/operator host and must not be copied into marshal state.
- Guardrail: preserve Phase A hardening, Managed Node versus Passive Endpoint behavior, benchmark scheduler behavior, and the V1/V2 Capability Endpoint boundary.
- Implementation direction: attach safe workload-suite descriptors to Managed Node benchmark jobs and subscriber job claims, keep Passive Endpoints probe-only, surface suite metadata in UI/API/docs, and add focused tests for non-leakage.

### 2026-07-09T13:14:10Z

- Completed the prompt workload benchmark suite definition slice.
- Added safe benchmark workload suite definitions:
  - `synthetic_smoke_v1` for built-in subscriber synthetic smoke tasks
  - `synthetic_code_v1` for built-in subscriber synthetic code-oriented tasks
  - `operator_local_fixtures_v1` for operator-provided local fixture manifests referenced by manifest ID only
- Added `GET /wrangler/benchmarks/workload-suites` for suite definitions without prompt text, response text, request bodies, raw headers, endpoint credentials, API keys, tokens, or payload content.
- Added `benchmark_workload_suites` to UI bootstrap and `benchmark_workload` summary metadata to bootstrap/metrics.
- Extended Managed Node benchmark job creation so `POST /wrangler/nodes/:id/benchmark` accepts optional `suite_id`, `fixture_manifest_id`, `fixture_id`, and `fixture_path`.
- Kept default benchmark queue behavior backward compatible by defaulting to `synthetic_smoke_v1`.
- Added workload suite metadata to Managed Node benchmark jobs and subscriber job claims:
  - suite ID/display name/source
  - input policy and fixture policy
  - task count and task IDs
  - result metric names
  - optional local fixture manifest ID or basename hint only
- Extended subscriber benchmark result ingestion to accept safe suite ID, task count, and fixture manifest ID metadata.
- Extended benchmark scheduler status with workload suite ID/source metadata.
- Updated Dashboard and Nodes UI:
  - Dashboard shows a Benchmark Suites card and storage posture
  - Managed Node cards show a suite selector and optional local fixture manifest ID field
  - Passive Endpoints remain probe-only and do not receive prompt workload controls
- Updated UI/API, node-control, support-bundle, support-bundle schema, and Phase B planning docs for benchmark workload suite metadata.
- Preserved the V1/V2 Capability Endpoint boundary; no non-Ollama integrations, generic endpoint registry, plugin execution, or arbitrary tool execution were added.
- Verified focused benchmark workload tests:
  - suite definitions expose metadata only
  - Managed Node jobs carry synthetic suite metadata only
  - local fixture suite requires a manifest/reference and stores no fixture contents or full local paths
  - unknown/future suite IDs are rejected
  - Passive Endpoints remain excluded from local-control benchmark jobs
- Verified `go test ./internal/httpapi -run 'TestBenchmarkWorkload|TestManagedBenchmarkJob|TestLocalFixtureBenchmark|TestSubscriberBenchmark|TestPassiveBenchmark|TestBenchmarkJob|TestBenchmarkScheduler|TestSummarizeBenchmarkPolicy|TestBootstrapAndMetricsIncludeBenchmarkPolicyStatus'`.
- Verified `go test ./...`.
- Verified `node -c internal/httpapi/static/app.js`.
- Verified `go build -o ./llama-wrangler ./cmd/llama-wrangler`.
- Restarted standalone service and verified live `GET /healthz`.
- Verified live `GET /wrangler/benchmarks/workload-suites` returns the three suite definitions without injected secret markers.
- Verified live `GET /wrangler/ui/bootstrap` exposes `benchmark_workload_suites`, `benchmark_workload`, and `schema_version: 2`.
- Verified live `GET /wrangler/metrics` exposes only `benchmark_workload` summary metadata and not full suite definitions.
- Verified live UI app shell includes Benchmark Suites, suite selector, and local fixture manifest controls.
- Verified in-app browser Dashboard and Nodes render the new Benchmark Suites surfaces with no console errors and no visible injected secret markers.
- Verified live invalid benchmark suite and missing local fixture manifest requests return safe reason codes without echoing submitted prompt/response markers.
- Verified live support-bundle export did not expose injected prompt/response markers, token-shaped values, OpenAI-style `sk-` markers, full fixture paths, or new suite payload content.
- Scope not completed yet: real subscriber-side execution of suite task IDs, synthetic task text packaging in the subscriber, configurable scheduler policy controls, optional background scheduler tick, full prompt workload result quality scoring, model lifecycle/warm-state V1 surfaces, consensus completion, and V2 Capability Endpoint implementation.

## Next Recommended Work

1. Add configurable benchmark scheduler policy controls for max attempts, lease timeout, and retry delay while preserving safe defaults and metadata-only behavior.
2. Add optional background scheduler tick for benchmark job reconciliation with operator-visible enablement and metadata-only telemetry.
3. Add subscriber-side runner guidance or packaging hooks for executing `synthetic_smoke_v1`, `synthetic_code_v1`, and local fixture manifest task IDs without returning prompts/responses to the marshal.
4. Keep future Capability Endpoint work documentation-only until V1 acceptance criteria are met, unless a current V1 implementation detail clearly blocks later additive expansion.

### 2026-07-09T13:16:05Z

- Continuing Phase B benchmark foundations.
- Active work item: add configurable benchmark scheduler policy controls for max attempts, lease timeout, and retry delay.
- Guardrail: scheduler policy controls must remain safe bounded metadata and must not add prompt, response, request body, raw header, endpoint credential, admin token, client API key, HEC token, enrollment token, heartbeat credential, token hash, fixture content, full fixture path, future provider key, or payload storage.
- Guardrail: preserve existing safe scheduler defaults: bounded retry/timeout policy, max attempts 3, lease timeout 600 seconds, retry delay 60 seconds unless explicitly configured inside allowed bounds.
- Guardrail: preserve Managed Node-only benchmark jobs, Passive Endpoint probe-only behavior, benchmark workload suite metadata, encrypted secrets behavior, support-bundle redaction, and the V1/V2 Capability Endpoint boundary.
- Implementation direction: add bounded config-backed scheduler policy metadata, expose it through bootstrap/metrics/UI, use it for new jobs, claims, retry waits, and reconcile fallback handling, and add focused tests for normalization and non-leakage.

### 2026-07-09T13:33:09Z

- Completed the configurable benchmark scheduler policy controls slice.
- Added `capabilities.benchmark_scheduler` config with normalized safe bounds for:
  - `policy`
  - `max_attempts`
  - `lease_timeout_seconds`
  - `retry_delay_seconds`
- Preserved existing default scheduler posture:
  - policy `bounded_retry_timeout_v1`
  - max attempts 3
  - lease timeout 600 seconds
  - retry delay 60 seconds
- Added `GET /wrangler/benchmarks/scheduler/policy` and `PUT /wrangler/benchmarks/scheduler/policy` for bounded metadata-only scheduler policy management.
- Added Dashboard scheduler controls for max attempts, lease timeout, and retry delay with server-provided limits.
- Updated new Managed Node benchmark jobs, subscriber job claims, failed-job retry scheduling, reconcile/default handling, bootstrap, and metrics to use the normalized scheduler policy.
- Preserved Managed Node-only benchmark jobs and Passive Endpoint probe-only behavior; Passive Endpoints still do not receive local-control benchmark jobs.
- Preserved benchmark workload suite metadata and the V1/V2 Capability Endpoint boundary; no prompt bodies, response bodies, fixture contents, raw headers, endpoint credentials, tokens, future provider keys, arbitrary tool execution, or non-Ollama endpoint integrations were added.
- Added focused tests for:
  - config defaults and bounds normalization
  - policy endpoint normalization and persistence
  - new job scheduler metadata
  - claim lease timeout behavior
  - failed-job retry delay behavior
  - scheduler status config/limits
  - support-bundle and API non-leakage
- Updated UI/API, node-control, and Phase B planning docs for configurable scheduler policy behavior.
- Verified `go test ./internal/config ./internal/httpapi -run 'TestBenchmarkScheduler|TestBenchmarkJobScheduler|TestManagedBenchmarkJob|TestBenchmarkWorkload|TestLoadMarshalExample|TestDefaultSafePosture|TestBenchmarkSchedulerNormalizationBounds'`.
- Verified `go test ./...`.
- Verified `node -c internal/httpapi/static/app.js`.
- Verified `git diff --check`.
- Verified `go build -o ./llama-wrangler ./cmd/llama-wrangler`.
- Restarted standalone service on `http://localhost:11435/ui/` and verified live `GET /healthz`.
- Verified live `GET /wrangler/benchmarks/scheduler/policy` returns bounded metadata-only config and limits.
- Verified live bootstrap and metrics expose benchmark scheduler config/limits while preserving schema version 2 and prior Phase A/Phase B metadata.
- Verified in-app browser Dashboard renders scheduler controls with bounded values and no console errors.
- Verified live support-bundle export includes scheduler metadata but no secret/payload markers.
- Scope not completed yet: optional background scheduler tick, subscriber-side execution/packaging hooks for benchmark workload suite task IDs, real synthetic task runner packaging, model lifecycle/warm-state V1 surfaces, consensus completion, Splunk dashboard expansion, Frontier Delta completion, and V2 Capability Endpoint implementation.

## Next Recommended Work

1. Add optional background scheduler tick for benchmark job reconciliation with operator-visible enablement and metadata-only telemetry.
2. Add subscriber-side runner guidance or packaging hooks for executing `synthetic_smoke_v1`, `synthetic_code_v1`, and local fixture manifest task IDs without returning prompts/responses to the marshal.
3. Add model lifecycle and warm-state V1 surfaces for Managed Nodes while keeping Passive Endpoints limited to marshal-observed metadata.
4. Keep future Capability Endpoint work documentation-only until V1 acceptance criteria are met, unless a current V1 implementation detail clearly blocks later additive expansion.

### 2026-07-09T13:50:30Z

- Continuing Phase B benchmark foundations.
- Active work item: add optional background scheduler tick for benchmark job reconciliation with operator-visible enablement and metadata-only telemetry.
- Guardrail: background reconciliation must be disabled by default unless explicitly enabled through bounded config/UI controls.
- Guardrail: background ticks may reconcile durable benchmark job metadata only; they must not queue new jobs automatically, execute prompts, store prompt/response bodies, store fixture contents, capture raw headers, store endpoint credentials, or expose admin/client/enrollment/heartbeat/HEC/future-provider secrets.
- Guardrail: preserve Managed Node-only benchmark jobs, Passive Endpoint probe-only behavior, benchmark workload suite metadata, configurable scheduler policy controls, encrypted secret behavior, support-bundle redaction, and the V1/V2 Capability Endpoint boundary.
- Implementation direction: reuse the existing scheduler reconciliation primitive, add bounded tick enablement/interval controls, expose status and last-run metadata in bootstrap/metrics/UI, emit metadata-only tick telemetry, and add focused tests for disabled-by-default behavior, enabled tick behavior, config normalization, and non-leakage.

### 2026-07-09T13:58:18Z

- Completed the optional background scheduler tick slice.
- Added `background_enabled` and `tick_interval_seconds` to `capabilities.benchmark_scheduler`.
- Preserved safe defaults:
  - background reconciliation disabled by default
  - tick interval default 60 seconds
  - tick interval bounds 10-3600 seconds
- Added a service-side background scheduler loop that polls config, respects runtime enable/disable changes, and reuses the existing benchmark job reconciliation primitive.
- Background ticks only reconcile existing Managed Node benchmark job metadata:
  - timed-out running jobs
  - due retry-wait jobs
  - exhausted max-attempt jobs
- Background ticks do not create benchmark jobs, execute prompt workloads, inspect Passive Endpoints, mutate subscribers, store fixture contents, capture raw headers, or store endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, future provider keys, prompts, responses, or payloads.
- Extended `benchmark_scheduler` bootstrap/metrics status with background enablement, interval, next tick, last tick, last reason, and last-run changed/timed-out/retried/exhausted counts.
- Extended `GET /wrangler/benchmarks/scheduler/policy` and `PUT /wrangler/benchmarks/scheduler/policy` to include bounded background scheduler settings.
- Hardened partial scheduler policy updates so omitted fields preserve existing settings before normalization, preventing older or narrower clients from accidentally disabling background reconciliation.
- Added metadata-only `benchmark_scheduler_background_tick` telemetry.
- Updated Dashboard Benchmark Scheduler controls with:
  - background reconciliation toggle
  - tick interval input
  - next tick status
  - last tick status
  - last tick change summary
- Updated UI/API, node-control, and Phase B planning docs for optional background reconciliation.
- Added focused tests for:
  - background scheduler disabled-by-default behavior
  - enabled background tick reconciliation
  - background status metadata
  - background tick telemetry
  - config and endpoint normalization
  - API/status/telemetry non-leakage
- Verified `go test ./internal/config ./internal/httpapi -run 'TestBenchmarkScheduler|TestBenchmarkJobScheduler|TestLoadMarshalExample|TestDefaultSafePosture|TestBenchmarkSchedulerNormalizationBounds'`.
- Verified `go test ./...`.
- Verified `node -c internal/httpapi/static/app.js`.
- Verified `git diff --check`.
- Verified `go build -o ./llama-wrangler ./cmd/llama-wrangler`.
- Restarted standalone service on `http://localhost:11435/ui/` and verified live `GET /healthz`.
- Verified live `GET /wrangler/benchmarks/scheduler/policy` reports background disabled by default and bounded tick interval limits.
- Verified live bootstrap and metrics expose background scheduler status while preserving schema version 2 and prior Phase A/Phase B metadata.
- Verified live support-bundle export includes background scheduler metadata but no secret/payload markers.
- Verified in-app browser Dashboard renders the background reconciliation toggle, tick interval bounds, next/last tick status, no visible secret markers, and no console errors.
- Scope not completed yet: subscriber-side execution/packaging hooks for benchmark workload suite task IDs, real synthetic task runner packaging, model lifecycle/warm-state V1 surfaces, consensus completion, Splunk dashboard expansion, Frontier Delta completion, and V2 Capability Endpoint implementation.

## Next Recommended Work

1. Add subscriber-side runner guidance or packaging hooks for executing `synthetic_smoke_v1`, `synthetic_code_v1`, and local fixture manifest task IDs without returning prompts/responses to the marshal.
2. Add model lifecycle and warm-state V1 surfaces for Managed Nodes while keeping Passive Endpoints limited to marshal-observed metadata.
3. Add UI/API affordances for benchmark scheduler history or recent background tick audit summaries if operators need more troubleshooting visibility.
4. Keep future Capability Endpoint work documentation-only until V1 acceptance criteria are met, unless a current V1 implementation detail clearly blocks later additive expansion.

### 2026-07-09T14:17:53Z

- Continuing Phase B benchmark foundations.
- Active work item: add subscriber-side runner guidance or packaging hooks for executing `synthetic_smoke_v1`, `synthetic_code_v1`, and local fixture manifest task IDs without returning prompts/responses to the marshal.
- Guardrail: this slice should add explicit guidance/hooks, not claim a full subscriber benchmark runner implementation unless one actually exists.
- Guardrail: runner guidance may include endpoint names, suite IDs, task IDs, metric field names, placeholder commands, env vars, config snippets, local fixture manifest references, and warnings only.
- Guardrail: prompt text, response text, fixture contents, full local fixture paths, raw request bodies, raw headers, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, future provider keys, and payloads must stay out of app state, API responses, bootstrap, metrics, telemetry, and support bundles.
- Guardrail: preserve Managed Node-only benchmark jobs, Passive Endpoint probe-only behavior, benchmark workload suite metadata, configurable scheduler policy controls, optional background scheduler tick behavior, encrypted secret behavior, support-bundle redaction, and the V1/V2 Capability Endpoint boundary.
- Implementation direction: expose sanitized subscriber benchmark runner guidance through UI/API docs and bootstrap, include packaging-hook placeholders for claim/status/result flows, add tests for non-leakage and clear not-yet-implemented runner status, and avoid remotely mutating subscriber hosts.

### 2026-07-09T14:23:38Z

- Completed the subscriber-side benchmark runner guidance and packaging-hook slice.
- Added `GET /wrangler/benchmarks/runner/guidance` with placeholder-only guidance for subscriber-side benchmark runner packaging.
- Added `benchmark_runner` to UI bootstrap so the Dashboard can surface runner status and boundaries.
- Runner guidance is explicitly marked `guidance_only_runner_not_implemented`; this slice does not claim a full runner loop exists.
- Guidance includes safe metadata only:
  - supported suite IDs
  - claim/status/result endpoint names
  - subscriber heartbeat header name
  - credential placeholders
  - env var names
  - config snippet
  - execution flow
  - metric field names
  - placeholder curl commands
  - local fixture manifest guidance
  - packaging-hook notes and warnings
- Preserved Managed Node-only runner posture and Passive Endpoint probe-only behavior.
- Preserved benchmark workload suite metadata, configurable scheduler policy controls, optional background scheduler tick behavior, encrypted secret behavior, support-bundle redaction, and the V1/V2 Capability Endpoint boundary.
- Kept prompt text, response text, fixture contents, full local fixture paths, raw credentials, token-shaped values, and payloads out of the guidance, bootstrap, UI card, and support-bundle checks.
- Added Dashboard Subscriber Benchmark Runner card with guidance-only status, supported suite badges, planned packaging-hook status, and local fixture storage policy.
- Updated UI/API docs, service-wrapper dry-run docs, node-control docs, and Phase B planning docs for subscriber runner guidance.
- Added focused tests for:
  - guidance endpoint shape
  - bootstrap guidance inclusion
  - supported suite IDs
  - placeholder-only command guidance
  - guidance-only runner status
  - prompt/response/credential/payload non-leakage
- Verified `go test ./internal/httpapi -run 'TestBenchmarkRunner|TestBenchmarkWorkload|TestManagedBenchmarkJob|TestLocalFixtureBenchmark|TestSubscriberBenchmark'`.
- Verified `go test ./...`.
- Verified `node -c internal/httpapi/static/app.js`.
- Verified `git diff --check`.
- Verified `go build -o ./llama-wrangler ./cmd/llama-wrangler`.
- Restarted standalone service on `http://localhost:11435/ui/` and verified live `GET /healthz`.
- Verified live `GET /wrangler/benchmarks/runner/guidance` reports guidance-only status, supported suite IDs, planned packaging-hook status, and local fixture storage policy.
- Verified live bootstrap reports `benchmark_runner` while preserving schema version 2 and prior Phase A/Phase B metadata.
- Verified live support-bundle export does not include secret/payload markers.
- Verified in-app browser Dashboard renders Subscriber Benchmark Runner guidance with expected labels, no visible secret markers, and no console errors.
- Scope not completed yet: real subscriber benchmark runner loop/dry-run harness, packaged built-in synthetic prompts on subscriber side, model lifecycle/warm-state V1 surfaces, consensus completion, Splunk dashboard expansion, Frontier Delta completion, and V2 Capability Endpoint implementation.

## Next Recommended Work

1. Add a real opt-in subscriber benchmark runner loop or dry-run harness once packaging and local prompt/fixture storage boundaries are ready.
2. Add keep-warm policy controls or model lifecycle actions only after explicit Managed Node subscriber support exists.
3. Add UI/API affordances for benchmark scheduler history or recent background tick audit summaries if operators need more troubleshooting visibility.
4. Keep future Capability Endpoint work documentation-only until V1 acceptance criteria are met, unless a current V1 implementation detail clearly blocks later additive expansion.

### 2026-07-09T18:21:03Z Current Status Pointer

- Latest completed slice: opt-in subscriber benchmark runner dry-run harness.
- Current runner state: implemented as disabled-by-default `dry_run_v1` subscriber-local plumbing that claims Managed Node jobs and reports metrics-only summaries.
- Phase B is not complete.
- Do not treat the older guidance-only runner entry as current status; it remains historical context for the previous slice.

## Next Recommended Work

1. Add keep-warm policy controls or model lifecycle actions only after explicit Managed Node subscriber support exists.
2. Add real subscriber-side synthetic prompt execution behind local-only packaging, with prompt text packaged exclusively on the subscriber and responses discarded locally.
3. Add UI/API affordances for benchmark scheduler history or recent background tick audit summaries if operators need more troubleshooting visibility.
4. Keep future Capability Endpoint work documentation-only until V1 acceptance criteria are met, unless a current V1 implementation detail clearly blocks later additive expansion.

### 2026-07-09T18:36:30Z Current Status Pointer

- Latest completed slice: keep-warm model lifecycle action queue with authenticated subscriber claim/status support.
- Current model lifecycle action state: implemented as metadata-only Managed Node keep-warm actions gated on approved, healthy, subscriber-reported warm-state and model-management support.
- Passive Endpoints remain inventory-only and have no lifecycle action controls.
- Phase B is not complete.
- Do not treat the older keep-warm next-work item above as current status; it remains historical context for the previous pointer.

## Next Recommended Work

1. Add real subscriber-side synthetic prompt execution behind local-only packaging, with prompt text packaged exclusively on the subscriber and responses discarded locally.
2. Add UI/API affordances for benchmark scheduler history or recent background tick audit summaries if operators need more troubleshooting visibility.
3. Add a small model lifecycle action history/filter surface if operators need to troubleshoot keep-warm action claims and completions.
4. Keep future Capability Endpoint work documentation-only until V1 acceptance criteria are met, unless a current V1 implementation detail clearly blocks later additive expansion.

### 2026-07-10T13:38:25Z Active Work

- Continuing Phase B benchmark scheduler operations hardening.
- Active work item: add UI/API affordances for recent benchmark scheduler reconciliation history so operators can troubleshoot retries and timeouts.
- Decision: retain a small bounded process-local audit ring for automatic background ticks and operator-triggered reconciliations. History resets on service restart and does not require a persisted app-state schema change.
- Guardrail: audit entries may contain timestamps, trigger/reason, scheduler policy metadata, and changed/timed-out/retried/exhausted counts only.
- Guardrail: audit entries must not persist or expose prompts, responses, fixture contents, full fixture paths, raw headers, raw request bodies, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, future provider keys, node payloads, or request payloads.
- Guardrail: preserve Phase A hardening, schema version 2 behavior, Managed Node/Passive Endpoint control and trust policy, benchmark workload suite metadata, configurable scheduler controls, optional background scheduler ticks, opt-in dry-run and synthetic subscriber runner behavior, model lifecycle/warm-state/action policy behavior, support-bundle sanitization/versioning, and the V1/V2 Capability Endpoint boundary.
- Implementation direction: record bounded newest-first audit summaries inside the existing synchronized scheduler runtime state, expose them through a dedicated admin API and scheduler status, render a compact Dashboard history, and add focused tests for trigger coverage, ordering, bounds, restart-empty behavior, and non-leakage.

### 2026-07-10T13:44:17Z Completion

- Completed the benchmark scheduler reconciliation history and operations UI slice.
- Added a bounded 24-entry, newest-first, process-local scheduler audit ring covering:
  - automatic background reconciliation ticks
  - operator-triggered manual reconciliations
- Audit entries contain only recorded timestamp, normalized trigger/reason, scheduler policy metadata, background enablement/tick interval, and changed/timed-out/retried/exhausted counts.
- Unknown internal trigger/reason labels are normalized to fixed safe values before entering the audit ring.
- History resets on service restart, returns an empty JSON array on a fresh process, and does not require or alter persisted app-state schema version 2.
- Added admin endpoint `GET /wrangler/benchmarks/scheduler/history`.
- Added `benchmark_scheduler.history` to bootstrap and metrics, including bounded retention metadata and aggregate trigger/outcome counts.
- Extended the Dashboard Benchmark Scheduler card with recent reconciliation count, timeout/retry totals, process-local retention status, and the six newest audit rows.
- Preserved Managed Node-only benchmark scheduling, Passive Endpoint probe-only behavior, benchmark workload suite metadata, configurable scheduler policy, optional background ticks, opt-in dry-run and synthetic subscriber runner behavior, model lifecycle/warm-state/action policy behavior, Phase A hardening, and the V1/V2 Capability Endpoint boundary.
- Kept prompts, responses, fixture contents, full fixture paths, raw headers, raw request bodies, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, future provider keys, and payloads out of persisted state, scheduler history, API responses, telemetry fields added by this slice, and support bundles.
- Added focused tests for fresh-process empty-array behavior, background/operator trigger coverage, newest-first ordering, 24-entry bounds, summary counts, bootstrap/metrics exposure, unsafe-label normalization, and scheduler-history/support-bundle non-leakage.
- Updated UI/API and Phase B planning documentation for the process-local audit boundary.
- Verified:
  - `go test ./internal/httpapi -run 'TestBenchmarkScheduler'`
  - `go test ./...`
  - `node -c internal/httpapi/static/app.js`
  - `go build -o ./llama-wrangler ./cmd/llama-wrangler`
  - `git diff --check`
  - live `GET /healthz`
  - live fresh-process `GET /wrangler/benchmarks/scheduler/history` returns `entries: []`
  - live `POST /wrangler/benchmarks/scheduler/reconcile` records a metadata-only operator audit entry
  - live history endpoint returns newest-first process-local audit metadata
  - live support-bundle marker scan found no prompt/response/token/header/full-fixture-path values
  - in-app browser Dashboard renders reconciliation count, retention, and recent audit rows with no overflow, visible secret markers, or console errors
- Phase B is not complete.

## Next Recommended Work

1. Add a compact model lifecycle action history/filter surface for keep-warm queue, claim, completion, failure, and safe error-code troubleshooting.
2. Complete V1 consensus execution and aggregation behavior for approved, fresh, trusted Managed Nodes while preserving current control/trust eligibility and no-retry-after-partial-output semantics.
3. Expand Splunk dashboards and metadata panels for queue scheduling, streaming outcomes, benchmark runner/scheduler history, routing policy, and model lifecycle actions.
4. Run V1 packaging/install and acceptance/security hardening, then release/docs/git stabilization.
5. Keep Capability Endpoint V2 work documentation-only unless a V1 implementation detail clearly blocks later additive expansion.

### 2026-07-10T16:05:39Z Active Work

- Continuing Phase B model lifecycle operations hardening.
- Active work item: add compact model lifecycle action history and filtering for keep-warm queue, claim, completion, failure, cancellation, and safe error-code troubleshooting.
- Decision: derive history from the existing bounded `model_lifecycle_actions` records already stored under sanitized node observed metadata. Do not add a second persisted action log or change app-state schema version 2.
- API direction: add a bounded metadata-only history projection with `status`, `node_id`, and `limit` filters, newest-first ordering, aggregate status counts, and explicit per-node retention metadata.
- UI direction: add a compact Dashboard action-history card with status/node selectors, apply/reset controls, safe summary counts, and recent action rows.
- Guardrail: strengthen action error-code normalization and expose only action ID/type/policy, node control/trust/approval metadata, safe model name, desired keep-warm state, status, safe timestamps, and safe error code.
- Guardrail: Passive Endpoints remain inventory-only and cannot queue, claim, complete, or otherwise receive model lifecycle actions.
- Guardrail: preserve Phase A hardening, schema version 2 behavior, Managed Node/Passive Endpoint behavior, benchmark scheduler history, benchmark workload suites, opt-in dry-run and synthetic subscriber runner behavior, model lifecycle/warm-state/action policy behavior, support-bundle sanitization/versioning, and the V1/V2 Capability Endpoint boundary.
- Guardrail: do not persist or expose prompts, responses, fixture contents, full fixture paths, raw headers, raw request bodies, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, future provider keys, or payloads.

### 2026-07-10T16:13:23Z Completion

- Completed the compact model lifecycle action history and filter slice.
- Added metadata-only admin endpoint `GET /wrangler/models/lifecycle/action-history`.
- Added bounded server-side filters:
  - `status`: `queued`, `running`, `completed`, `failed`, or `cancelled`
  - `node_id`: exact Managed Node ID
  - `limit`: 1-50, default 20
- Derived newest-first history from existing per-node `model_lifecycle_actions`; no second persisted log or app-state schema change was added.
- Preserved the existing retention bound of eight actions per Managed Node.
- Added safe action history fields for action ID/type/policy, node control/trust/approval metadata, model, desired keep-warm state, current status, queue/claim/update/completion/failure timestamps, and normalized safe error code.
- Strengthened model lifecycle action error-code handling:
  - lowercase code vocabulary
  - maximum 64 characters
  - letters, digits, underscore, hyphen, and period only
  - suspicious secret/header markers become `redacted_error_code`
  - malformed values become `invalid_error_code`
- Normalized unknown legacy action type, policy, status, and error-code values before history projection.
- Excluded Passive Endpoint action-like metadata from history; Passive Endpoints remain inventory-only and cannot queue or receive lifecycle actions.
- Added `model_lifecycle_action_history` to bootstrap and metrics with default recent-history filters and aggregate status/error counts.
- Added Dashboard Model Action History card with status and Managed Node selectors, apply/reset controls, matching/status/error summaries, queue/claim/outcome timestamps, and compact recent action rows.
- Strengthened support-bundle string sanitization for secret prompt/response marker values as defense in depth; support-bundle schema version remains 1 because no new top-level bundle field was added.
- Added focused tests for all lifecycle statuses, newest-first ordering, status/node/limit filters, Passive Endpoint exclusion, invalid filter rejection, legacy unsafe-value normalization, empty-array behavior, bootstrap/metrics exposure, and history/support-bundle non-leakage.
- Updated UI/API, support-bundle, and Phase B planning documentation.
- Verified:
  - `go test ./internal/httpapi -run 'TestModelLifecycle|TestManagedModelKeepWarm|TestModelKeepWarm'`
  - `go test ./...`
  - `node -c internal/httpapi/static/app.js`
  - `go build -o ./llama-wrangler ./cmd/llama-wrangler`
  - `git diff --check`
  - live `GET /healthz`
  - live unfiltered and failed-only action-history API responses
  - live invalid filter returns generic HTTP 400 without echoing filter content
  - live bootstrap remains schema version 2 and exposes default action history
  - live metrics exposes the same metadata-only history
  - live support-bundle marker scan found no prompt/response/token/header/full-fixture-path values
  - in-app browser Dashboard applies and resets status/node filters with no overflow, visible secret markers, or console errors
- Current live fleet has no lifecycle action records because its Managed Node has not reported warm-state support; the UI correctly shows an empty history state.
- Phase B is not complete.

## Next Recommended Work

1. Add the V1 non-streaming consensus fan-out and deterministic aggregation foundation for approved, fresh, trusted Managed Nodes only.
2. Add consensus partial-failure, cancellation, timeout, and streaming compatibility hardening while preserving no retry after partial output.
3. Expand Splunk dashboards and metadata panels for queue scheduling, streaming outcomes, benchmark runner/scheduler history, routing policy, consensus, and model lifecycle actions.
4. Run V1 packaging/install, acceptance/security hardening, documentation/release polish, and git stabilization.
5. Keep Capability Endpoint V2 work documentation-only unless a V1 implementation detail clearly blocks later additive expansion.

### 2026-07-10T16:16:44Z Active Work

- Continuing Phase B consensus execution foundations.
- Active work item: implement V1 non-streaming consensus fan-out and deterministic aggregation for approved, fresh, trusted Managed Nodes.
- Current baseline: routing already enforces Managed Node control level, local/lan-trusted consensus trust, approval, enabled/health state, heartbeat freshness, model availability, and minimum participants, but marshal execution still forwards to one selected node and does not apply configured maximum participants.
- Routing direction: apply bounded `max_participants` after eligibility and placement ordering while preserving `min_participants` enforcement and exclusion metadata.
- Execution direction: concurrently fan out non-streaming compatibility requests to selected Managed Nodes under the existing request timeout and cancellation context, buffer responses in memory only, require enough successful participants, and return one deterministic winning upstream response through the requesting inference API.
- Aggregation direction: add a pluggable local consensus engine with normalized exact-text matching, JSON structural matching, a validator hook, and a local evaluator stub. Majority agreement wins; ties and no-majority outcomes select the earliest ranked successful participant deterministically and emit warning metadata only.
- Compatibility direction: preserve the winning OpenAI/Ollama response body and content type unchanged. Consensus metadata is telemetry-only unless the request explicitly enables safe debug headers. Streaming consensus is rejected before fan-out in this foundation slice so no partial output or retry ambiguity is introduced.
- Telemetry direction: emit participant IDs/counts, success/failure counts, agreement score, comparison strategy, winner node, disagreement/consensus state, duration, cancellation/timeout state, and disabled frontier escalation recommendation only. Do not emit prompt or response content.
- Guardrail: Passive Endpoints, lan-unverified nodes, external nodes, pending/revoked nodes, stale/missing-heartbeat Managed Nodes, unhealthy/disabled nodes, and model-ineligible nodes must not participate.
- Guardrail: preserve Phase A hardening, benchmark scheduler/history behavior, benchmark workload/subscriber runner behavior, model lifecycle/action history behavior, support-bundle sanitization/versioning, normalized compatibility errors, and the V1/V2 Capability Endpoint boundary.
- Guardrail: prompt and response bodies may exist only in bounded request memory and the final requesting compatibility response; they must not enter persisted state, telemetry, management APIs, logs, or support bundles.

### 2026-07-10T16:32:39Z Completion

- Completed the V1 non-streaming consensus fan-out and deterministic aggregation foundation.
- Added `internal/consensus` pluggable local engine with:
  - normalized exact-text comparison
  - JSON structural comparison for answer content
  - regex validator hook
  - local evaluator interface with no-op default
  - strict-majority agreement scoring
  - deterministic routing-rank tie/no-majority selection
- Added safe consensus participant bounds:
  - missing minimum defaults to 2
  - missing maximum defaults to 4
  - maximum fan-out caps at 8
  - maximum applies after control/trust/approval/health/heartbeat/model/benchmark-placement ordering
  - equal-score routing order is stabilized by node ID
- Added concurrent non-streaming fan-out through existing OpenAI/Ollama compatibility paths.
- Fan-out uses the request cancellation context and configured routing timeout, caps each buffered participant response at 8 MiB, and keeps all prompt/response content in request memory only.
- Preserved winning upstream status, content type, and response body without consensus body rewriting.
- Added debug-only safe headers for consensus state, successful participant count, agreement score, and winner node when `X-Llama-Wrangler-Debug: true` or request `debug: true` is explicitly set.
- Added compatibility errors:
  - `consensus_streaming_not_supported` before any streaming fan-out
  - `consensus_insufficient_successful_participants` when successful responses do not meet the required bound
- Streaming single-route SSE/JSONL and no-retry-after-partial-output behavior remain unchanged. Streaming consensus is explicitly deferred for the next hardening decision.
- Added metadata-only `consensus` telemetry with participant IDs/counts, required/maximum bounds, success/failure counts, agreement count/score, comparison strategy, validator result, winner node, disagreement/reached state, timeout/cancellation, duration, and disabled-frontier escalation recommendation.
- `consensus_delta` can recommend unresolved escalation in metadata but does not call Frontier providers.
- Added `operation_stats.consensus` in bootstrap/metrics and a Dashboard Consensus Outcomes card for reached/no-majority/failed/timeout/cancelled/streaming-rejected counts plus last safe outcome metadata.
- Passive Endpoints, lan-unverified/external nodes, pending/revoked nodes, disabled/unhealthy nodes, stale/missing-heartbeat nodes, and model-ineligible nodes remain excluded from consensus.
- Added focused tests for:
  - exact normalized and JSON structural agreement
  - validator hook and deterministic no-majority ranking
  - default/configured min/max bounds and max fan-out
  - OpenAI and Ollama winner compatibility
  - one-participant failure with sufficient remaining agreement
  - insufficient successful participants
  - streaming rejection before fan-out
  - cancellation and configured timeout
  - metadata-only telemetry/support-bundle behavior
  - consensus operations summaries
- Updated README, marshal example config, architecture, capability model, event schema, UI/API docs, support-bundle docs, and Phase B planning docs.
- Verified:
  - focused consensus/routing/HTTP/operations tests
  - focused race detector for consensus, routing, and HTTP fan-out
  - `go test ./...`
  - `node -c internal/httpapi/static/app.js`
  - `go build -o ./llama-wrangler ./cmd/llama-wrangler`
  - `git diff --check`
  - live `GET /healthz`
  - live no-prompt `local-consensus` request safely returns `no_eligible_node` because the current fleet has only one consensus-eligible Managed Node
  - live bootstrap remains schema version 2 and exposes metadata-only consensus operation counters
  - live metrics exposes the same consensus counters
  - served UI JavaScript includes the Consensus Outcomes card
  - live support-bundle marker scan found no prompt/response/token/header/full-fixture-path values
  - in-app browser visual automation could not attach to an available tab after the service restart; tab discovery was empty and two fresh background-tab attachment attempts timed out, so no visual rendering claim is made for this slice
- Phase B is not complete.

## Next Recommended Work

1. Harden V1 consensus partial-failure reason codes, upstream 4xx/5xx classification, timeout/cancellation outcome detail, and response-size failure telemetry without adding payload data.
2. Add real-client OpenAI/Ollama non-streaming consensus compatibility checks and decide/document whether V1 streaming consensus remains explicitly unsupported or gains a separate aggregation protocol.
3. Expand Splunk dashboards and metadata panels for queue scheduling, streaming outcomes, benchmark runner/scheduler history, routing policy, consensus, and model lifecycle actions.
4. Run V1 packaging/install, acceptance/security hardening, documentation/release polish, and git stabilization.
5. Keep Capability Endpoint V2 work documentation-only unless a V1 implementation detail clearly blocks later additive expansion.

### 2026-07-10T16:58:33Z Active Work

- Continuing Phase B consensus outcome hardening.
- Active work item: add metadata-only participant failure reason codes and compatibility-aware unmet-quorum behavior for V1 non-streaming consensus.
- Required participant failure codes: `missing_proxy_url`, `connection_error`, `upstream_4xx`, `upstream_5xx`, `body_read_failure`, `response_size_limit`, `timeout`, and `cancellation`.
- Failure projection direction: expose only node ID, fixed reason code, optional numeric upstream status code, and bounded duration milliseconds. Do not expose error strings, URLs, raw headers, request/response content, extracted content, or signatures.
- Collection direction: preserve deterministic routing order for successes and failures, classify completed participant outcomes, and classify unfinished requests as timeout or cancellation when the shared context ends.
- Compatibility direction: preserve aggregation when successful participants meet quorum; when no participant succeeds and all failures are upstream 4xx, reuse normalized OpenAI/Ollama upstream error mapping; otherwise use the compatibility-shaped consensus insufficient-successes error, with HTTP 504 for timeout-driven unmet quorum.
- Operations direction: aggregate fixed participant failure reason counts into metadata-only consensus telemetry, bootstrap/metrics operation stats, and the existing Dashboard Consensus Outcomes card.
- Guardrail: streaming consensus remains rejected before fan-out in this slice.
- Guardrail: preserve deterministic V1 non-streaming aggregation, Phase A hardening, Managed Node/Passive Endpoint policy, benchmark scheduler/history behavior, benchmark workload/subscriber runner behavior, model lifecycle/action history behavior, support-bundle sanitization/versioning, and the V1/V2 Capability Endpoint boundary.
- Guardrail: prompts, responses, extracted content, comparison signatures, validator/evaluator input, raw headers, raw request bodies, credentials, URLs, arbitrary error text, and payloads must remain out of persisted state, telemetry, management APIs, logs, and support bundles.

### 2026-07-10T17:09:31Z Completion

- Completed V1 consensus partial-failure and upstream outcome hardening without marking Phase B complete.
- Added fixed participant failure reason codes:
  - `missing_proxy_url`
  - `connection_error`
  - `upstream_4xx`
  - `upstream_5xx`
  - `body_read_failure`
  - `response_size_limit`
  - `timeout`
  - `cancellation`
- Added safe per-participant failure projections containing only node ID, fixed reason code, optional numeric upstream status, and duration milliseconds.
- Preserved routing-ranked ordering for successful and failed participants and classified unfinished requests from the shared context as timeout or cancellation.
- Preserved deterministic in-memory aggregation when configured required successes are met despite partial participant failures.
- Hardened unmet-quorum behavior:
  - all-participant upstream 4xx failures reuse normalized OpenAI/Ollama upstream status and error-code mapping
  - timeout-driven insufficient successes return HTTP 504 with `consensus_insufficient_successful_participants`
  - other insufficient-success outcomes return HTTP 502 with the same compatibility-shaped safe error code
  - upstream error/response bodies and arbitrary transport error strings are not relayed
- Added `participant_failures` and `failure_reason_counts` to metadata-only consensus telemetry.
- Added fixed-vocabulary `operation_stats.consensus.failure_reasons` aggregation in bootstrap/metrics and rendered reason badges in the Dashboard Consensus Outcomes card; unknown reason labels are ignored.
- Streaming consensus remains rejected before fan-out with `consensus_streaming_not_supported`; single-route OpenAI SSE and Ollama JSONL behavior is unchanged.
- Preserved Phase A hardening, schema version 2, encrypted fallback/keychain posture, Managed Node/Passive Endpoint routing policy, benchmark scheduler/history and subscriber runner behavior, model lifecycle/action history behavior, sanitized/versioned support bundles, and the V1/V2 Capability Endpoint boundary.
- Updated README, architecture, event schema, HEC JSON Schema, UI/API, support-bundle contract, and Phase B planning docs.
- Added focused tests for all eight failure classes, safe failure projection, partial success, unmet quorum, all-upstream-4xx OpenAI/Ollama error compatibility, timeout/cancellation, operations aggregation, unknown-reason filtering, and payload/secret non-leakage.
- Verified:
  - focused consensus/operations HTTP tests
  - focused race detector across HTTP consensus, consensus engine, and routing
  - `go test ./...`
  - `node -c internal/httpapi/static/app.js`
  - `jq empty schemas/hec_events.schema.json`
  - `go build -o ./llama-wrangler ./cmd/llama-wrangler`
  - `git diff --check`
  - rebuilt standalone service at `http://localhost:11435/ui/`
  - live `GET /healthz` returns 200
  - live bootstrap/metrics expose `operation_stats.consensus.failure_reasons` as metadata only
  - live no-prompt `local-consensus` request still returns safe `no_eligible_node` because the current fleet has only one consensus-eligible Managed Node
  - live support bundle remains bundle schema version 1/state schema version 2 with privacy flags false and no prompt/response/token/header marker matches
  - in-app browser Dashboard renders Consensus Outcomes and the empty participant-failure state, preserves the Passive Endpoint consensus warning, reports no console warnings/errors, and shows no full admin/client/heartbeat token or test-secret markers
- Phase B is not complete.

## Next Recommended Work

1. Add real-client OpenAI/Ollama non-streaming consensus compatibility checks for partial success, normalized upstream 4xx, timeout/quorum errors, and deterministic winner shape; record the V1 decision to keep streaming consensus explicitly unsupported unless a separate safe aggregation protocol is designed.
2. Expand Splunk dashboards and metadata panels for consensus failure reasons, queue scheduling, streaming outcomes, benchmark runner/scheduler history, routing policy, and model lifecycle actions.
3. Run V1 packaging/install, acceptance/security hardening, documentation/release polish, and git stabilization.
4. Keep Capability Endpoint V2 work documentation-only unless a V1 implementation detail clearly blocks later additive expansion.

### 2026-07-10T17:12:16Z Active Work

- Continuing Phase B consensus compatibility hardening.
- Active work item: add real-client OpenAI/Ollama non-streaming consensus checks through public HTTP routes and real upstream test servers.
- Compatibility matrix: partial participant success with required quorum, deterministic winner body/status/content type, normalized all-upstream-4xx errors for both endpoint families, timeout-driven and non-timeout unmet-quorum errors, and streaming rejection before fan-out.
- Client direction: use `net/http` clients and `httptest.Server` network listeners rather than only direct handlers/recorders so response framing, body decoding, cancellation, and compatibility shapes are exercised at the SDK-facing boundary.
- Privacy direction: use explicit prompt/response/error markers in tests and prove they appear only in the requesting inference response when successful; they must not appear in audit state, telemetry, management APIs, logs, or support bundles.
- V1 streaming posture decision: keep consensus streaming explicitly unsupported. Safe streaming consensus would require a separately designed protocol for per-participant stream buffering, cancellation, quorum timing, deterministic aggregation, bounded memory/backpressure, and no-retry-after-partial-output guarantees; that protocol is outside this slice and not evidenced by the current non-streaming engine.
- Guardrail: preserve all Phase A hardening, schema version 2, Managed Node/Passive Endpoint control/trust/approval/freshness/model policy, fixed consensus failure-reason semantics, benchmark scheduler/history and subscriber runner behavior, model lifecycle/action history behavior, sanitized/versioned support bundles, and the V1/V2 Capability Endpoint boundary.
- Guardrail: prompts, responses, extracted content, raw upstream errors, raw headers, raw request bodies, credentials, endpoint URLs, comparison signatures, validator/evaluator input, and payloads remain excluded from persisted state, telemetry, management APIs, logs, and support bundles.
- Phase B must remain open after this compatibility slice.

### 2026-07-10T17:20:34Z Completion

- Completed the real-client OpenAI/Ollama V1 non-streaming consensus compatibility slice without marking Phase B complete.
- Added `internal/httpapi/consensus_compat_test.go` with real marshal and participant `httptest.Server` listeners and `net/http` client readers.
- Added network-level compatibility coverage for both OpenAI `/v1/chat/completions` and Ollama `/api/generate`:
  - partial participant failure with the configured success quorum still returns a deterministic routing-ranked winner
  - winner body, status, and content type retain the requesting compatibility family shape
  - participant requests receive the resolved model and expected subscriber proxy path
  - all-participant upstream 4xx outcomes retain normalized status/code/type semantics without relaying upstream error bodies
  - one success plus one upstream 5xx with required quorum two returns compatibility-shaped HTTP 502 and does not return the partial candidate
  - participant deadline expiry returns compatibility-shaped HTTP 504 at the configured bounded timeout
  - streaming consensus returns `consensus_streaming_not_supported` before any participant request
- Added explicit prompt, response, partial-response, and upstream-error markers and proved they remain absent from audit state, bootstrap, metrics, and support-bundle API output. Consensus telemetry continues to use only the existing safe participant/outcome projection, so JSON logs and HEC do not receive those content fields.
- Recorded the V1 streaming consensus posture decision:
  - streaming consensus is deliberately unsupported in V1
  - the non-streaming consensus engine must not be reused as evidence of streaming safety
  - reconsideration requires a separate protocol and acceptance suite for bounded per-participant stream collection, quorum timing, deterministic aggregation, backpressure, cancellation, response commitment, and no-retry-after-partial-output behavior
  - single-route OpenAI SSE and Ollama JSONL behavior remains unchanged
- Updated README, architecture, UI/API, and Phase B planning docs with the compatibility evidence and streaming decision.
- Preserved Phase A hardening, schema version 2, encrypted fallback/keychain posture, Managed Node/Passive Endpoint routing policy, fixed consensus failure reasons, benchmark scheduler/history and subscriber runner behavior, model lifecycle/action history behavior, sanitized/versioned support bundles, and the V1/V2 Capability Endpoint boundary.
- Verified:
  - focused real-client consensus compatibility tests
  - focused race detector across real-client compatibility, consensus handler, participant classification, and fan-out tests
  - `go test ./...`
  - `node -c internal/httpapi/static/app.js`
  - `go build -o ./llama-wrangler ./cmd/llama-wrangler`
  - `git diff --check`
  - rebuilt standalone service at `http://localhost:11435/ui/`
  - live `GET /healthz` returns 200
  - live no-prompt OpenAI and Ollama `local-consensus` requests retain their correct compatibility error envelopes for the current one-eligible-node fleet
  - live bootstrap remains schema version 2 and preserves encrypted fallback, local-only safe defaults, benchmark scheduler, model lifecycle action, Passive Endpoint exclusion, and metadata-only consensus operation state
  - live support bundle remains bundle schema version 1/state schema version 2 with privacy flags false and no real-client/test/token/header marker matches
  - in-app browser Dashboard renders Consensus Outcomes and Passive Endpoint consensus warnings with no console warnings/errors and no visible credential or test markers
- Phase B is not complete.

## Next Recommended Work

1. Expand Splunk dashboards and metadata panels for consensus failure reasons, queue scheduling, streaming outcomes, benchmark scheduler/runner history, routing policy, and model lifecycle actions.
2. Add a V1 acceptance/security matrix covering install/start/restart, setup completion, credential boundaries, Managed Node enrollment, Passive Endpoint behavior, routing/consensus, benchmark/model lifecycle flows, and support-bundle privacy.
3. Continue packaging/install hardening, documentation/release polish, and git stabilization.
4. Keep Capability Endpoint V2 work documentation-only unless a V1 implementation detail clearly blocks later additive expansion.

### 2026-07-10T17:25:19Z Active Work

- Continuing Phase B observability and operator troubleshooting surfaces.
- Active work item: expand the packaged Splunk app for consensus participant failures, queue scheduling, streaming retry/partial/cancellation outcomes, benchmark scheduler and runner history, routing policy warnings, and model lifecycle action history.
- Current baseline: the app contains one overview dashboard, request/response/health macros and eventtypes, basic props, and three disabled summary searches. The service already emits the required metadata event types, but the Splunk assets do not yet expose them.
- Dashboard direction: keep the existing Overview dashboard and add a dedicated Operations dashboard with a shared time picker and metadata-only panels for each requested operational domain.
- Knowledge-object direction: add reusable macros, eventtypes, props categories, disabled-by-default saved reports, and navigation entries using only event types and fields currently emitted by the service.
- Consensus direction: chart only the fixed failure vocabulary `missing_proxy_url`, `connection_error`, `upstream_4xx`, `upstream_5xx`, `body_read_failure`, `response_size_limit`, `timeout`, and `cancellation`.
- Routing direction: derive warning distributions from existing `routing_decision.excluded_nodes{}.reasons{}` plus node control/trust/capability-source metadata; do not invent fields or a payload-bearing routing event.
- Validation direction: parse packaged Simple XML/navigation, assert required macros/eventtypes/saved searches/props, assert all fixed consensus reasons, and reject prompt/response-body/raw-header/credential/fixture/payload field references.
- Guardrail: preserve metadata-only telemetry, fixed consensus failure semantics, all Phase A hardening, schema version 2, Managed Node/Passive Endpoint policy, benchmark scheduler/history and subscriber runner behavior, model lifecycle/action history behavior, sanitized/versioned support bundles, and the V1/V2 Capability Endpoint boundary.
- Guardrail: do not add prompt text, response text, extracted content, raw request/response bodies, raw headers, authorization data, admin/client/HEC/enrollment/heartbeat/provider credentials, fixture contents or paths, comparison signatures, validator/evaluator input, or payload fields to Splunk searches, panels, reports, schemas, docs examples, or tests.
- Phase B must remain open after this Splunk slice.

### 2026-07-10T17:37:03Z Completion

- Completed the Phase B Splunk operations visibility slice without marking Phase B or the full Splunk phase complete.
- Bumped the companion Splunk app package version from `0.1.0` to `0.2.0`.
- Added `llama_wrangler_operations.xml` with 16 metadata-only panels covering:
  - consensus run/failure totals, all eight fixed participant failure reasons, and recent safe outcomes
  - queue depth by priority and scheduling-policy/priority/status outcomes
  - streaming retry, partial-response, and cancellation trends/history
  - benchmark job/scheduler reconciliation and scheduler-policy history
  - subscriber benchmark runner tick history
  - routing exclusion reason/control/trust/capability-source distributions and recent routing decisions
  - model lifecycle action trends and queue/claim/status/rejection history
- Expanded the existing Overview dashboard with consensus participant failure, peak queue depth, and streaming outcome signals.
- Added packaged app navigation exposing both Overview and Operations dashboards.
- Added reusable macros for consensus, queue, streaming outcomes, benchmark scheduler, benchmark runner, combined benchmark operations, routing, and model lifecycle actions.
- Added matching operational eventtypes/tags and explicit props categories for emitted queue, streaming, benchmark scheduler/runner, and model lifecycle sourcetypes.
- Added six disabled-by-default operational saved reports, bringing the package total to nine; no report is scheduled/enabled automatically.
- Scoped the legacy error eventtype so its `OR status=error` clause cannot search outside `index=llama_wrangler`.
- Kept routing panels aligned with emitted metadata: `excluded_nodes` reason, node ID, control level, trust level, and capability source only; no approval field was invented for routing events.
- Updated Splunk app README, project README, Splunk scope, event schema, UI/API docs, and Phase B plan.
- Added focused `internal/hec/splunk_assets_test.go` validation for:
  - Overview, Operations, and navigation XML parsing
  - required panels/macros/eventtypes/tags/props/saved reports
  - all saved reports disabled by default
  - exact eight-reason consensus vocabulary parity with `schemas/hec_events.schema.json`
  - rejection of inference-content, raw-body/header, authorization, credential, fixture, and raw payload field references
- Preserved metadata-only telemetry, fixed consensus failure semantics, Phase A hardening, schema version 2, Managed Node/Passive Endpoint policy, benchmark scheduler/history and subscriber runner behavior, model lifecycle/action behavior, sanitized/versioned support bundles, and the V1/V2 Capability Endpoint boundary.
- Verified:
  - focused Splunk asset/privacy/schema tests
  - `xmllint --noout` for both dashboards and packaged navigation
  - all seven Splunk `.conf` files parse with unique stanzas through Python `RawConfigParser`
  - `jq empty schemas/hec_events.schema.json`
  - `go test ./...`
  - `node -c internal/httpapi/static/app.js`
  - `go build -o ./llama-wrangler ./cmd/llama-wrangler`
  - `git diff --check`
  - rebuilt standalone service at `http://localhost:11435/ui/`
  - live `GET /healthz` returns 200
  - live bootstrap preserves schema version 2, encrypted fallback, weighted queue policy, consensus operation stats, routing warnings, benchmark scheduler/runner status, model lifecycle action history, and Splunk TLS metadata
  - live support bundle remains bundle schema version 1/state schema version 2 with all privacy flags false and no test/token/header marker matches
  - in-app browser Dashboard and Splunk Settings render with no console warnings/errors, preserve Passive Endpoint warnings and TLS verification guidance, and expose no visible credentials/test markers
- Splunk Enterprise/Cloud is not installed in this workspace, so live SPL execution and visual rendering inside a real Splunk search head were not claimed. Packaged XML/conf/schema validation is complete for this slice; runtime Splunk acceptance remains part of the consolidated V1 acceptance/security matrix.
- Phase B is not complete.

## Next Recommended Work

1. Add sanitized Splunk sample events and disabled-by-default alert definitions against the fixed V1 event catalog and privacy vocabulary.
2. Build a macOS release-candidate installer/uninstaller that targets normal user paths in review/dry-run mode first; keep signing, notarization, packaged keychain, and real normal-path mutation pending explicit evidence.
3. Run package-candidate smoke tests with Cline, Continue, Open WebUI, the generic OpenAI SDK, and the Ollama CLI.
4. Continue documentation/release polish, clean-checkout acceptance, and git stabilization.
5. Keep Capability Endpoint V2 work documentation-only unless a V1 implementation detail clearly blocks later additive expansion.

### 2026-07-10T18:37:43Z Active Work

- Continuing Phase B/V1 release hardening.
- Active work item: add a consolidated acceptance/security matrix plus an executable disposable local harness.
- Matrix direction: define stable acceptance IDs, requirement/evidence/automation status, and residual environment-dependent checks for service lifecycle, safe defaults/state migration, setup/admin/client auth, encrypted secret boundaries, Managed Node enrollment/heartbeat/approval, Passive Endpoint limitations, routing/non-streaming consensus, benchmark scheduler/runner, model lifecycle actions, Splunk assets, support-bundle privacy, embedded UI, and the V1/V2 boundary.
- Harness direction: reuse existing focused Go tests by domain rather than duplicate behavior, then independently build the binary, start and restart it with disposable `HOME`/`XDG_CONFIG_HOME`, a loopback-only temporary port, and a temporary config, probe live UI/API/privacy invariants, validate Splunk XML/conf/schema assets, and remove all temporary state on exit.
- Service safety direction: the harness must not call install/uninstall helpers, `launchctl`, systemd, Windows service APIs, `sudo`, or mutate real OS services/keychains; macOS service packaging remains a later explicit acceptance slice.
- Secret direction: generated runtime credentials remain inside the disposable app-data directory/process output; the harness must not print them, persist them in the repository, include them in reports, or transmit them externally.
- Status direction: automated matrix rows may be marked implemented only when backed by a named test or harness step. Splunk search-head rendering, real OS service installation, signing/notarization, and cross-platform service-user credential checks remain explicitly pending environment-dependent acceptance.
- Guardrail: preserve all Phase A hardening, schema version 2, encrypted fallback as supported service/default credential storage, OS keychain as interactive opt-in, Managed Node/Passive Endpoint policy, fixed consensus failure semantics, metadata-only observability, V1 streaming consensus unsupported decision, benchmark/model lifecycle behavior, support-bundle privacy, Splunk app boundaries, and the V1/V2 Capability Endpoint boundary.
- Guardrail: prompts, responses, extracted content, raw HTTP material, credentials, fixture contents/full paths, comparison signatures, validator/evaluator input, and payloads must not enter acceptance artifacts, logs, repository files, support bundles, or matrix evidence.
- Phase B and V1 release acceptance must remain open after this slice unless every environment-dependent row has genuine evidence.

### 2026-07-10T19:01:18Z Completion

- Completed the consolidated Phase B/V1 acceptance and security matrix slice without marking Phase B or V1 release acceptance complete.
- Added `docs/21_v1_acceptance_security_matrix.md` with stable release-gate IDs, requirement/evidence mappings, automated status, environment-dependent status, and a release rule that does not convert skipped or unavailable checks into passes.
- Added executable `scripts/v1_acceptance.sh` with 22 grouped, static, build, disposable lifecycle, restart, persistence, authentication, node enrollment, encrypted-secret, Splunk-package, and support-bundle privacy gates.
- The harness uses a temporary loopback port, disposable `HOME` and `XDG_CONFIG_HOME`, encrypted-file secret storage, a temporary build, process-local credential variables, and unconditional cleanup. It does not install/uninstall services, invoke `launchctl`/systemd/Windows service APIs, use `sudo`, modify a keychain, retain temporary credential state, or print the service log.
- Added focused `internal/acceptance` tests that keep the matrix IDs and harness hooks synchronized, require the script to remain executable, reject real OS-service/keychain mutation commands, preserve the documentation-only V1/V2 Capability Endpoint boundary, and require every external acceptance row to remain explicitly pending.
- Updated README and MVP roadmap documentation with the V1 release-gate command, safety boundary, and distinction between automated local evidence and external packaging/runtime evidence.
- Full verification passed: `./scripts/v1_acceptance.sh` completed all 22 gates, including `go test ./...`, schema/JavaScript/Splunk asset validation, disposable build/start/setup/auth/enrollment/privacy/restart checks, and persisted schema/auth/node checks after restart.
- Development-shortcut verification passed: `LLAMA_WRANGLER_ACCEPTANCE_SKIP_TESTS=1 ./scripts/v1_acceptance.sh` completed 14 static/lifecycle gates and explicitly reported that skipped grouped/full tests are not complete release evidence.
- Cleanup verification found no remaining `/tmp/llama-wrangler-v1-acceptance.*` directories after either run.
- Rebuilt and restarted the normal standalone service at `http://localhost:11435/ui/`; live health, bootstrap, support-bundle, and repository-marker checks confirm schema version 2, safe defaults, encrypted fallback, unchanged Managed/Passive node records, fixed consensus metadata, and no disposable acceptance-state leakage.
- In-app browser verification passed for Setup, Dashboard, Settings, and Nodes. The UI rendered consensus outcomes, routing warnings, benchmark scheduler/runner status, model lifecycle action history, schema/config posture, encrypted-secret guidance, Managed Node enrollment/approval controls, and Passive Endpoint limitations with no full credentials, acceptance markers, console warnings, or console errors.
- Remaining external evidence is intentionally open: macOS/Linux/Windows service packages, packaged macOS keychain behavior, signing/notarization, real Splunk search-head execution, and package-candidate client integrations.
- Phase B and V1 release acceptance remain open.

### 2026-07-10T19:26:27Z Active Work

- Continuing Phase B/V1 macOS packaging hardening.
- Active work item: add dry-run-first user-level launchd install/start/restart/upgrade/uninstall acceptance with an explicit opt-in disposable runtime path.
- Packaging boundary: use a unique launchd label, temporary package root, temporary `HOME`/`XDG_CONFIG_HOME`, loopback-only port, temporary logs, and encrypted fallback secret storage. Do not write to the operator's real `~/Library/LaunchAgents`, system LaunchDaemons, system-wide paths, or real app-data directory.
- Mutation boundary: default execution may build, render, and lint disposable artifacts only. `launchctl bootstrap`, `kickstart`, and `bootout` require an exact opt-in flag, operate only in the current user's `gui/<uid>` domain, and must be paired with unconditional cleanup.
- Upgrade evidence direction: replace the disposable packaged binary with a separately identified build, restart the user agent, verify the process changes and health/auth/schema state survives, then boot out and remove all disposable artifacts.
- Credential boundary: encrypted fallback remains the explicit supported service backend; OS keychain remains interactive opt-in and is not exercised or promoted by this harness. Runtime credentials may exist only in process-local variables and disposable encrypted storage and must not be printed, logged as evidence, committed, or included in support artifacts.
- Acceptance status direction: successful current-host disposable launchd lifecycle evidence may close only the user-level lifecycle mechanics row. Signed/notarized package-candidate acceptance, normal-user install paths, packaged keychain behavior, Linux/Windows service packaging, Splunk runtime, and external clients remain pending.
- Guardrail: preserve all Phase A hardening, schema version 2, Managed Node/Passive Endpoint policy, fixed consensus failure semantics, metadata-only observability, support-bundle privacy, V1 streaming consensus unsupported decision, and the documentation-only V1/V2 Capability Endpoint boundary.
- Phase B and V1 release acceptance remain open.

### 2026-07-10T19:47:32Z Completion

- Completed the dry-run-first macOS user-level launchd packaging acceptance slice without marking Phase B, macOS release packaging, or V1 release acceptance complete.
- Added executable `scripts/macos_user_launchd_acceptance.sh`:
  - default M01-M02 builds an identified disposable binary, renders an absolute-path user LaunchAgent plan with explicit encrypted fallback, writes only inside a temporary home, validates plist mode `0600`, runs `plutil -lint`, and exits without calling `launchctl`
  - exact opt-in `LLAMA_WRANGLER_MACOS_LAUNCHD_ACCEPTANCE=1` adds M03-M08 for unique `gui/<uid>` bootstrap/start, setup/schema/secret posture, restart persistence, atomic identified-binary replacement, service-log privacy, bootout, endpoint removal, and artifact deletion
  - unconditional cleanup boots out only the unique temporary label and removes the temporary home/package/config/log/app-data root
- Hardened `service-dry-run` so launchd plans use absolute, optionally relocatable LaunchAgents/log paths, explicitly select `encrypted_file` by default, preserve `os_keychain` as opt-in, and reject unsafe launchd labels.
- Changed the CLI version symbol to a link-time-overridable variable so the upgrade harness can prove initial versus replacement package identity without changing runtime schemas or persisted state.
- Suppressed recovery-admin-token output when `LLAMA_WRANGLER_SERVICE_MODE` is active; interactive startup behavior remains unchanged. The launchd lifecycle scan confirmed temporary stdout/stderr logs contained no full admin/client/enrollment/heartbeat credentials or payload markers.
- Added focused CLI, service-wrapper, and acceptance safety tests for service-mode detection, encrypted fallback defaults, path relocation, unsafe labels, exact opt-in ordering, executable permissions, forbidden system-service/keychain commands, stable matrix IDs, and pending release-candidate/keychain/signing rows.
- Updated README, installation docs, service-wrapper docs, MVP roadmap, acceptance matrix, and ledger. Added `V1-PKG-MAC-DRY-001` for the non-mutating plan/lint gate, recorded current-host evidence on `V1-PKG-MAC-001`, and added separate pending `V1-PKG-MAC-003` release-candidate acceptance.
- Verification evidence:
  - default macOS dry-run passed M01-M02 and confirmed no launchd registration
  - explicit opt-in lifecycle passed M01-M08 on macOS 27.0 arm64
  - independent cleanup checks found no `com.llama-wrangler.acceptance.*` job and no `llama-wrangler-launchd-acceptance.*` temporary directory
  - `go test ./... -count=1` passed
  - `./scripts/v1_acceptance.sh` passed all 22 consolidated automated gates
  - rebuilt normal standalone service responds at `http://localhost:11435/ui/`; live bootstrap preserves schema version 2, localhost/local-only safe defaults, encrypted fallback, Managed/Passive node metadata, and fixed consensus metadata
  - in-app browser Setup/Dashboard/Settings regression passed with no full credentials, disposable labels, warnings, or console errors
- Still pending and not inferred from this evidence: signed/notarized package-candidate installation into normal user paths, upgrade migration across real released versions, operator-facing recovery/uninstall data choices, packaged OS keychain behavior, Linux/Windows service packages, real Splunk runtime, and external-client package-candidate checks.
- Phase B and V1 release acceptance remain open.

### 2026-07-10T22:20:08Z Active Work

- Packaging side quest: create the Splunk app `0.2.0` as a `.tar.gz` release artifact.
- Archive contract: use `llama_wrangler/` as the single top-level app directory and exclude macOS metadata such as `.DS_Store`, `__MACOSX`, and AppleDouble `._*` files.
- Verification direction: run focused Splunk asset validation, list and inspect the completed archive, reject unexpected root entries or macOS metadata, and record the artifact checksum.
- This packaging task does not change Splunk runtime acceptance status or mark Phase G complete.

### 2026-07-10T22:20:50Z Completion

- Packaged Splunk app version `0.2.0` as `dist/llama_wrangler-0.2.0.tar.gz`.
- Archive root contains only `llama_wrangler/`; no files or sibling directories exist at the archive root.
- Packaging used `COPYFILE_DISABLE=1` and explicit exclusions for `.DS_Store`, `__MACOSX`, and AppleDouble `._*` entries.
- Focused Splunk asset tests passed with `go test ./internal/hec -count=1`.
- Archive listing and disposable extraction verified required `app.conf`, metadata, navigation, Overview dashboard, and Operations dashboard assets; packaged `app.conf` reports version `0.2.0`.
- Gzip integrity passed, extracted metadata scan was clean, and the archive size is 5,365 bytes.
- SHA-256: `56cb128d324093641662cd13bdf34ec8e648421254c6a647bffe87ad8600c6e5`.
- Real Splunk search-head installation/runtime acceptance remains pending; Phase G and V1 release acceptance are not marked complete.

### 2026-07-13T12:54:45Z Active Work

- Operator reported that the packaged Llama Wrangler Splunk app `0.2.0` is installed.
- Active work item: update README current status and future-capabilities guidance, record Splunk installation without inferring unverified runtime behavior, run release checks, then commit and push the complete accumulated Phase B/V1 change set.
- README direction: distinguish implemented foundations, current acceptance evidence, remaining V1 release gaps, installed Splunk package status, and documentation-only future Capability Endpoint work.
- Splunk acceptance direction: installation may be recorded as evidence, while event ingestion, every dashboard/search, permissions, and performance remain pending until explicitly exercised.
- Git direction: review the full dirty worktree, include the verified `0.2.0` Splunk archive, avoid unrelated generated/runtime files, commit intentionally on `main`, and push to `origin/main` only after verification passes.
- Guardrail: do not mark Phase B, Phase G, V1 release acceptance, signed packaging, external-client compatibility, or future V2 capabilities complete without supporting evidence.

### 2026-07-13T12:58:20Z Completion

- Updated the root README to reflect current V1 hardening status, implemented service/node/routing/benchmark/model/Splunk foundations, verified acceptance evidence, and remaining release gaps.
- Added a README Future Capabilities section that keeps generalized Capability Endpoints, additional runtimes, richer subscriber control, streaming consensus, Frontier adapters, cross-platform packaging, and broader developer/build integrations explicitly additive and post-V1.
- Recorded the operator-reported Splunk app `0.2.0` installation in the acceptance matrix without inferring sanitized ingestion, dashboard/search execution, permissions, or performance evidence; `V1-SPLUNK-RUNTIME-001` remains pending those checks.
- README links the clean `dist/llama_wrangler-0.2.0.tar.gz` artifact and distinguishes service pre-release `0.1.0` from Splunk app `0.2.0`.
- Verification passed:
  - `./scripts/v1_acceptance.sh` completed all 22 gates
  - Splunk archive gzip integrity, single-root layout, macOS-metadata exclusion, and SHA-256 were revalidated
  - `git diff --check` passed
  - rebuilt standalone service is healthy and serves the embedded UI at `http://localhost:11435/ui/`
- The complete accumulated Phase B/V1 worktree is prepared for intentional commit and push; the resulting Git commit/push evidence is recorded by repository history and the session closeout rather than predicted in this ledger entry.
- Phase B, Phase G, and V1 release acceptance remain open.

### 2026-07-09T18:40:06Z Active Work

- Continuing Phase B model lifecycle action hardening.
- Current baseline: metadata-only keep-warm model lifecycle action queueing and authenticated subscriber claim/status support already exist for eligible Managed Nodes.
- Active work item: add operator-visible keep-warm/model lifecycle action policy status so the UI/API explain which nodes can receive actions, which nodes are blocked, and why, without adding Passive Endpoint local-control semantics.
- Guardrail: keep model lifecycle actions Managed Node only and require approved, enabled, healthy nodes with subscriber-reported warm-state and model-management support before action queueing.
- Guardrail: preserve Phase A hardening, schema version 2 behavior, Managed Node/Passive Endpoint behavior, benchmark scheduler behavior, benchmark workload suite metadata, opt-in subscriber benchmark runner dry-run behavior, model lifecycle/warm-state V1 surfaces, support-bundle redaction, and the V1/V2 Capability Endpoint boundary.
- Guardrail: do not persist or expose prompts, responses, secrets, fixture contents, raw headers, raw request bodies, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, future provider keys, or payloads.
- Implementation direction: expose metadata-only action policy status in API/bootstrap/metrics, render eligibility and blocked-reason UI near node lifecycle controls, add focused tests for Managed/Passive policy explanations and non-leakage, and document the support boundary.

### 2026-07-09T18:46:43Z Completion

- Completed the keep-warm/model lifecycle action policy visibility hardening slice.
- Added metadata-only `GET /wrangler/models/lifecycle/action-policies`.
- Added `model_lifecycle_actions` to UI bootstrap and metrics with:
  - action policy window
  - eligible node counts
  - blocked node counts
  - passive inventory-only node counts
  - model lifecycle action counts
  - pending action counts
  - per-node source/mode, supported action IDs, safe messages, and reason codes
- Kept the action policy read-only and aligned with the existing keep-warm queue gates: Managed Node, approved, enabled, healthy, subscriber-reported warm-state support, subscriber-reported model-management support, and safe model inventory.
- Dashboard now shows action-ready, blocked-action, and pending-action counts in the Model Lifecycle card.
- Nodes UI now shows a per-node Model actions policy row and explains why a node is ready or blocked.
- Verified current live fleet behavior:
  - local Managed Node is blocked because warm-state support is not yet subscriber-reported
  - Passive Endpoint is blocked with inventory-only/model-management unavailable reasons
  - no keep-warm action buttons render for ineligible live nodes
- Preserved Phase A hardening, schema version 2 behavior, Managed Node/Passive Endpoint behavior, benchmark scheduler behavior, benchmark workload suite metadata, opt-in subscriber benchmark runner dry-run behavior, model lifecycle/warm-state V1 surfaces, support-bundle redaction, and the V1/V2 Capability Endpoint boundary.
- No prompts, responses, secrets, fixture contents, raw headers, raw request bodies, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, future provider keys, or payloads were added to persisted state/API/support-bundle flows by this slice.
- Added focused tests for model lifecycle action policy status, Managed Node eligibility, Passive Endpoint blocking, support-pending Managed Node blocking, empty inventory blocking, bootstrap/metrics/endpoint exposure, and non-leakage.
- Updated UI/API docs, support-bundle schema docs, node-control docs, and Phase B plan docs.
- Verified:
  - `go test ./internal/httpapi -run 'TestModelLifecycle|TestManagedModelKeepWarm|TestModelKeepWarm'`
  - `go test ./...`
  - `node -c internal/httpapi/static/app.js`
  - `go build -o ./llama-wrangler ./cmd/llama-wrangler`
  - `git diff --check`
  - live `GET /healthz`
  - live `GET /wrangler/models/lifecycle/action-policies`
  - live bootstrap and metrics expose `model_lifecycle_actions`
  - live support-bundle leak grep found no forbidden prompt/response/token/header markers
  - in-app browser Dashboard and Nodes render the new action policy status with no console errors or visible secret markers
- Phase B is not complete.

## Next Recommended Work

1. Add real subscriber-side synthetic prompt execution behind local-only packaging, with prompt text packaged exclusively on the subscriber and responses discarded locally.
2. Add UI/API affordances for benchmark scheduler history or recent background tick audit summaries to make retry/timeout reconciliation easier to troubleshoot.
3. Add a small model lifecycle action history/filter surface if operators need deeper keep-warm claim/completion troubleshooting beyond the current per-node action policy status.
4. Keep future Capability Endpoint work documentation-only until V1 acceptance criteria are met, unless a current V1 implementation detail clearly blocks later additive expansion.

### 2026-07-10T02:10:04Z Active Work

- Continuing Phase B benchmark execution foundations.
- Active work item: add real subscriber-side synthetic prompt execution behind local-only packaging.
- Guardrail: prompt text must be packaged and resolved only inside subscriber runtime code. The marshal may exchange suite IDs, task IDs, benchmark IDs, model names, bounded timings, token counts, token-rate summaries, runner mode, task count, fixture manifest IDs or basename hints, and safe error codes only.
- Guardrail: responses must be discarded locally by the subscriber runner and must not be sent to the marshal, persisted in state, included in API responses, emitted in telemetry, or exported in support bundles.
- Guardrail: operator local fixture contents and full fixture paths must remain subscriber-local. This slice may keep local fixture manifests out of real execution if a safe manifest parser/storage boundary is not ready.
- Guardrail: runner execution remains Managed Node only. Passive Endpoints stay marshal-observed/probe-only and must not receive subscriber benchmark runner controls or local prompt execution.
- Guardrail: preserve Phase A hardening, schema version 2 behavior, Managed Node/Passive Endpoint behavior, benchmark scheduler behavior, benchmark workload suite metadata, opt-in dry-run behavior, model lifecycle/warm-state/action policy behavior, support-bundle redaction, and the V1/V2 Capability Endpoint boundary.
- V1 completion estimate: roughly 6-9 focused slices remain after this one, covering benchmark audit/history, optional model-action history, consensus completion, Splunk dashboard expansion, packaging/install polish, acceptance/security hardening, docs/release polish, and final git/PR stabilization.
- Implementation direction: add an explicit opt-in real synthetic runner mode, keep dry-run as the default, execute built-in synthetic suite/task IDs against the subscriber-local Ollama endpoint when enabled, discard response bodies after deriving bounded metrics, surface mode/availability in guidance/bootstrap/UI, and add focused tests for opt-in behavior, metadata-only reporting, Passive Endpoint exclusion, and non-leakage.

### 2026-07-10T02:17:29Z Completion

- Completed the real subscriber-side synthetic prompt execution slice.
- Added opt-in benchmark runner mode `synthetic_builtin_v1`.
- Preserved `dry_run_v1` as the default runner mode and fallback for unknown modes.
- Added subscriber-local synthetic execution against the subscriber's configured Ollama `/api/generate` endpoint.
- Built-in synthetic prompts are resolved from suite/task IDs inside subscriber runtime and are not exposed through marshal workload suite definitions, bootstrap, metrics, telemetry, or support bundles.
- Ollama response bodies are discarded locally after deriving bounded metric summaries.
- Subscriber benchmark result ingestion now accepts safe `runner_mode` metadata so dry-run versus synthetic execution is auditable without storing prompts or responses.
- Operator local fixture execution remains disabled in synthetic mode with safe error code `runner_fixture_execution_not_enabled` until a dedicated manifest parser/storage boundary is implemented.
- Passive Endpoints remain marshal-observed/probe-only and cannot use subscriber benchmark runners.
- Updated runner guidance, Dashboard copy, UI/API docs, service-wrapper docs, and Phase B planning docs for `synthetic_builtin_v1`.
- Added focused tests for:
  - preserving the opt-in synthetic runner config mode
  - subscriber-local Ollama execution for built-in synthetic suites
  - metric-only result reporting with safe runner mode
  - response-body discard and prompt/body non-persistence
  - local fixture execution-safe rejection without calling Ollama
  - existing dry-run behavior and Passive Endpoint runner exclusion
- Verified:
  - `go test ./internal/config ./internal/httpapi -run 'TestBenchmarkRunner|TestSubscriberBenchmarkRunner|TestMetricsIncludesBenchmarkRunner|TestBenchmarkWorkload|TestManagedBenchmarkJob|TestBenchmarkScheduler'`
  - `go test ./internal/httpapi -run 'TestSubscriberBenchmarkRunnerSyntheticBuiltin'`
  - `go test ./...`
  - `node -c internal/httpapi/static/app.js`
  - `go build -o ./llama-wrangler ./cmd/llama-wrangler`
  - `git diff --check`
  - live `GET /healthz`
  - live `GET /wrangler/benchmarks/runner/guidance`
  - live bootstrap and metrics expose the updated runner guidance
  - live support-bundle leak grep found no prompt/response/token/header/full-fixture-path markers
  - served UI assets include the synthetic runner copy without secret markers
  - in-app browser Dashboard renders the Subscriber Benchmark Runner card with dry-run default and synthetic built-in mode copy, no visible secret markers, and no console errors
- Phase B is not complete.

## Next Recommended Work

1. Add UI/API affordances for benchmark scheduler history or recent background tick audit summaries to make retry/timeout reconciliation easier to troubleshoot.
2. Add a small model lifecycle action history/filter surface if operators need deeper keep-warm claim/completion troubleshooting beyond the current per-node action policy status.
3. Complete consensus execution/aggregation behavior for V1 local fleet workflows.
4. Expand Splunk dashboards/metadata panels for the now richer queue, runner, benchmark, routing-policy, and lifecycle telemetry.
5. Keep future Capability Endpoint work documentation-only until V1 acceptance criteria are met, unless a current V1 implementation detail clearly blocks later additive expansion.

### 2026-07-09T18:36:30Z

- Completed the keep-warm model lifecycle action slice.
- Added metadata-only Managed Node keep-warm action queueing through `POST /wrangler/nodes/:id/model-actions/keep-warm`.
- Added authenticated subscriber model-action endpoints:
  - `POST /subscriber/model-actions/claim`
  - `POST /subscriber/model-actions/status`
- Model lifecycle actions require:
  - Managed Node control level
  - approved approval state
  - enabled node
  - healthy status
  - subscriber-reported warm-state support
  - subscriber-reported model-management support
  - existing subscriber-reported model inventory entry
- Passive Endpoints are explicitly rejected with `passive_no_model_management_control` and remain inventory-only.
- Action records store only safe metadata:
  - action ID
  - action type
  - policy
  - node ID
  - model name
  - desired keep-warm state
  - status
  - timestamps
  - subscriber endpoint names
  - safe error code when present
- Completing a subscriber-reported keep-warm action updates safe model metadata and recalculates model lifecycle summary counts.
- Extended `model_lifecycle` status with action counts, pending action counts, and last action status.
- Added Nodes UI keep-warm/clear keep-warm controls that render only for eligible Managed Nodes with model-management and warm-state support.
- Added compact button styling for model badge actions.
- Updated UI/API docs, node-control docs, support-bundle docs, and Phase B planning docs for the action queue and subscriber claim/status boundary.
- Preserved Phase A hardening, schema version 2 behavior, Managed Node/Passive Endpoint behavior, benchmark scheduler behavior, benchmark workload suite metadata, opt-in subscriber benchmark runner dry-run behavior, model lifecycle/warm-state V1 surfaces, support-bundle redaction, and the V1/V2 Capability Endpoint boundary.
- Kept prompts, responses, secrets, fixture contents, raw headers, raw request bodies, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, future provider keys, full fixture paths, and payloads out of persisted state, API responses, telemetry, and support bundles.
- Added focused tests for:
  - Managed Node keep-warm action queue, claim, and completion
  - safe model metadata update after completion
  - model lifecycle action summary counts
  - Passive Endpoint rejection
  - unsupported Managed Node rejection before subscriber-reported warm-state support
  - action/API non-leakage
- Verified `go test ./internal/httpapi -run 'TestManagedModelKeepWarmAction|TestModelKeepWarmAction|TestManagedHeartbeatUpdatesModelLifecycle|TestPassiveEndpointModelLifecycle|TestSubscriberBenchmarkRunner|TestBenchmarkRunner'`.
- Verified `go test ./...`.
- Verified `node -c internal/httpapi/static/app.js`.
- Verified `git diff --check`.
- Verified `go build -o ./llama-wrangler ./cmd/llama-wrangler`.
- Restarted standalone service on `http://localhost:11435/ui/` and verified live `GET /healthz`.
- Verified live `GET /wrangler/models/lifecycle`, bootstrap, and support-bundle export preserve schema version 2, expose action metadata, and do not expose secret/payload markers.
- Verified in-app browser Dashboard and Nodes views render lifecycle/runner surfaces without console errors or visible secret markers.
- Verified live UI action gating: current live state has zero eligible Managed Nodes with subscriber-reported model-management and warm-state support, and zero keep-warm action buttons were rendered.
- Scope not completed yet: real subscriber-side synthetic prompt execution packaged exclusively on the subscriber, benchmark scheduler history/audit UI, consensus completion, Splunk dashboard expansion, Frontier Delta completion, and V2 Capability Endpoint implementation.

## Next Recommended Work

1. Add real subscriber-side synthetic prompt execution behind local-only packaging, with prompt text packaged exclusively on the subscriber and responses discarded locally.
2. Add UI/API affordances for benchmark scheduler history or recent background tick audit summaries if operators need more troubleshooting visibility.
3. Add a small model lifecycle action history/filter surface if operators need to troubleshoot keep-warm action claims and completions.
4. Keep future Capability Endpoint work documentation-only until V1 acceptance criteria are met, unless a current V1 implementation detail clearly blocks later additive expansion.

### 2026-07-09T18:29:31Z

- Continuing Phase B model lifecycle control foundations.
- Active work item: add keep-warm policy controls or model lifecycle actions only after explicit Managed Node subscriber support exists.
- Guardrail: model lifecycle actions must be Managed Node only and require explicit subscriber-side claim/status support; Passive Endpoints remain inventory-only and must not receive warm-state, keep-warm, unload, pull, local load, or model-management controls.
- Guardrail: actions may store safe metadata such as action ID, action type, model name, desired keep-warm state, status, timestamps, reason/error codes, and endpoint names only.
- Guardrail: actions must not store or expose prompts, responses, raw request bodies, raw headers, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, future provider keys, fixture contents, full fixture paths, or payloads.
- Guardrail: preserve Phase A hardening, schema version 2 behavior, Managed Node/Passive Endpoint control and trust policy, benchmark workload suite metadata, configurable scheduler controls, optional background scheduler tick behavior, opt-in subscriber benchmark runner dry-run behavior, model lifecycle/warm-state V1 surfaces, support-bundle redaction, and the V1/V2 Capability Endpoint boundary.
- Implementation direction: add metadata-only keep-warm action queuing for eligible Managed Nodes, authenticated subscriber claim/status endpoints, UI controls gated by model-management support, focused tests for Managed/Passive behavior and non-leakage, and docs that make clear the marshal queues actions but does not mutate subscriber hosts directly.

### 2026-07-09T18:21:03Z

- Completed the opt-in subscriber benchmark runner dry-run harness slice.
- Added `capabilities.benchmark_runner` config with safe defaults:
  - disabled by default
  - mode `dry_run_v1`
  - result body policy `metrics_only`
  - bounded poll interval
  - bounded max jobs per tick
- Added a subscriber-mode background runner loop that starts only when explicitly enabled.
- The runner loop uses existing subscriber heartbeat credential semantics and keeps the resolved credential in runtime process memory only.
- The runner loop claims Managed Node benchmark jobs through `/subscriber/benchmarks/claim`, posts status transitions through `/subscriber/benchmarks/status`, and reports deterministic dry-run metric summaries through `/subscriber/benchmarks`.
- The dry-run harness derives metrics from suite/task metadata only. It does not load prompt text, response text, fixture contents, full fixture paths, raw headers, raw request bodies, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, future provider keys, or payloads.
- Passive Endpoints remain excluded from runner execution and stay marshal-observed/probe-only.
- Updated `GET /wrangler/benchmarks/runner/guidance`, bootstrap, metrics, and Dashboard runner card to show dry-run availability, disabled/enabled status, mode, result policy, poll interval, max jobs per tick, supported suites, and fixture storage boundary.
- Updated subscriber example config with disabled benchmark runner settings.
- Updated UI/API docs, service-wrapper dry-run docs, support-bundle docs, node-control docs, and Phase B planning docs for the opt-in dry-run runner boundary.
- Preserved Phase A hardening, schema version 2 behavior, Managed Node/Passive Endpoint control and trust policy, benchmark workload suite metadata, configurable scheduler controls, optional background scheduler tick behavior, model lifecycle/warm-state V1 surfaces, support-bundle redaction, and the V1/V2 Capability Endpoint boundary.
- Added focused tests for:
  - benchmark runner config defaults and normalization bounds
  - disabled-by-default runner guidance/bootstrap/metrics
  - subscriber dry-run claim/status/result completion flow
  - Passive Endpoint runner exclusion
  - runtime heartbeat credential handling without ordinary state/API/support-bundle leakage
- Verified `go test ./internal/config ./internal/httpapi -run 'TestBenchmarkRunner|TestSubscriberBenchmarkRunner|TestMetricsIncludesBenchmarkRunner|TestSubscriberHeartbeatCredentialEnvResolution|TestDefaultSafePosture|TestBenchmarkRunnerNormalizationBounds|TestBenchmarkScheduler|TestBenchmarkWorkload|TestManagedBenchmarkJob|TestModelLifecycle'`.
- Verified `go test ./...`.
- Verified `node -c internal/httpapi/static/app.js`.
- Verified `go build -o ./llama-wrangler ./cmd/llama-wrangler`.
- Verified `git diff --check`.
- Restarted standalone service on `http://localhost:11435/ui/` and verified live `GET /healthz`.
- Verified live `GET /wrangler/benchmarks/runner/guidance`, bootstrap, and metrics expose dry-run runner metadata while preserving schema version 2 and prior Phase A/Phase B metadata.
- Verified live support-bundle export includes runner metadata but no secret/payload markers.
- Verified in-app browser Dashboard renders Subscriber Benchmark Runner dry-run status, disabled-by-default posture, metrics-only result policy, fixture storage boundary, and no visible secret markers or console errors.
- Scope not completed yet: real subscriber-side synthetic prompt execution packaged exclusively on the subscriber, keep-warm policy/action controls backed by explicit subscriber support, benchmark scheduler history/audit UI, consensus completion, Splunk dashboard expansion, Frontier Delta completion, and V2 Capability Endpoint implementation.

## Next Recommended Work

1. Add keep-warm policy controls or model lifecycle actions only after explicit Managed Node subscriber support exists.
2. Add real subscriber-side synthetic prompt execution behind local-only packaging, with prompt text packaged exclusively on the subscriber and responses discarded locally.
3. Add UI/API affordances for benchmark scheduler history or recent background tick audit summaries if operators need more troubleshooting visibility.
4. Keep future Capability Endpoint work documentation-only until V1 acceptance criteria are met, unless a current V1 implementation detail clearly blocks later additive expansion.

### 2026-07-09T18:11:10Z

- Continuing Phase B benchmark execution foundations.
- Active work item: add a real opt-in subscriber benchmark runner loop or dry-run harness once packaging and local prompt/fixture storage boundaries are ready.
- Guardrail: this slice may add a subscriber-local runner loop and deterministic dry-run harness, but must not execute prompts on the marshal, store prompt text, store response text, store fixture contents, store full local fixture paths, capture raw request bodies, capture raw headers, store endpoint credentials, or expose admin/client/HEC/enrollment/heartbeat/future-provider secrets.
- Guardrail: runner execution must remain Managed Node only. Passive Endpoints stay marshal-observed/probe-only and must not receive local-control benchmark jobs, synthetic prompt execution, fixture execution, or subscriber runner controls.
- Guardrail: preserve Phase A hardening, schema version 2 behavior, Managed Node/Passive Endpoint control and trust policy, benchmark workload suite metadata, configurable scheduler controls, optional background scheduler tick behavior, subscriber runner guidance, model lifecycle/warm-state V1 surfaces, support-bundle redaction, and the V1/V2 Capability Endpoint boundary.
- Implementation direction: add explicit opt-in subscriber runner configuration, expose status through guidance/bootstrap/UI, implement a bounded local dry-run loop that claims jobs, marks them running/completed/failed, reports metric summaries only, and add focused tests for disabled-by-default behavior, dry-run claim/result behavior, Passive Endpoint exclusion, and non-leakage.

### 2026-07-09T18:07:01Z

- Continuing Phase B node capability visibility.
- Active work item: add model lifecycle and warm-state V1 surfaces for Managed Nodes while keeping Passive Endpoints limited to marshal-observed metadata.
- Guardrail: Managed Nodes may surface subscriber-reported model lifecycle and warm-state metadata, but must not persist or expose prompts, responses, raw request bodies, raw headers, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, future provider keys, fixture contents, full fixture paths, or payloads.
- Guardrail: Passive Endpoints remain inventory-only and marshal-observed; they must not imply local control, warm-state inspection, local load inspection, model management, benchmark job execution, or subscriber-side capabilities.
- Guardrail: preserve Phase A hardening, schema version 2 behavior, Managed Node/Passive Endpoint control and trust policy, benchmark suite metadata, configurable scheduler controls, optional background scheduler tick behavior, subscriber runner guidance, support-bundle redaction, and the V1/V2 Capability Endpoint boundary.
- Implementation direction: add a metadata-only lifecycle status surface, include it in bootstrap/metrics/UI, accept sanitized Managed Node heartbeat model state summaries, derive passive inventory-only metadata from safe `/api/tags` observations, document the boundary, and add focused tests for managed/passive behavior and non-leakage.

### 2026-07-09T18:07:01Z

- Completed the model lifecycle and warm-state V1 surfaces slice.
- Added `GET /wrangler/models/lifecycle` with current metadata-only model lifecycle status.
- Added `model_lifecycle` to UI bootstrap and metrics.
- Managed Node heartbeat/enrollment/manual-add paths now sanitize model-state metadata and record subscriber-reported lifecycle source/mode, warm-model counts, keep-warm counts, and safe timing/throughput summaries.
- Passive Endpoint add flow now records marshal-observed inventory-only lifecycle metadata from safe `/api/tags` model names without implying warm-state, local load, local benchmark execution, or model-management control.
- Dashboard now shows a Model Lifecycle card with total model counts, warm counts, keep-warm counts, inventory-only endpoint counts, and per-node lifecycle rows.
- Nodes UI now shows lifecycle mode, source, warm model counts, keep-warm counts, and sanitized model state badges where available.
- Added model lifecycle documentation to UI/API docs and Managed Node versus Passive Endpoint control-mode docs.
- Updated Phase B planning docs so next lifecycle actions remain gated on explicit Managed Node subscriber support.
- Preserved the V1/V2 Capability Endpoint boundary; no arbitrary future endpoint control, payload inspection, or non-Ollama provider integrations were added.
- Preserved metadata-only posture and support-bundle redaction; no prompt bodies, response bodies, fixture contents, raw headers, endpoint credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, future provider keys, full fixture paths, or payloads are stored or surfaced by this slice.
- Added focused tests for:
  - Managed Node heartbeat lifecycle and warm-state metadata
  - Passive Endpoint inventory-only lifecycle metadata
  - lifecycle bootstrap, metrics, and endpoint exposure
  - model-state sanitization and non-leakage
- Verified `go test ./internal/httpapi -run 'TestManagedHeartbeatUpdatesModelLifecycle|TestPassiveEndpointModelLifecycle|TestSubscriberHeartbeatUpdatesFreshness|TestPassiveAddNode|TestPassiveBenchmarkProbe|TestBenchmarkRunner|TestBenchmarkScheduler'`.
- Verified `go test ./...`.
- Verified `node -c internal/httpapi/static/app.js`.
- Verified `git diff --check`.
- Verified `go build -o ./llama-wrangler ./cmd/llama-wrangler`.
- Restarted standalone service on `http://localhost:11435/ui/` and verified live `GET /healthz`.
- Verified live `GET /wrangler/models/lifecycle`, bootstrap, and metrics expose model lifecycle metadata while preserving schema version 2 and prior Phase A/Phase B metadata.
- Verified live support-bundle export does not include secret/payload markers.
- Verified in-app browser Dashboard and Nodes views render model lifecycle/warm-state surfaces, show Passive Endpoint limitations, contain no visible secret markers, and report no console errors.
- Scope not completed yet: real subscriber benchmark runner loop/dry-run harness, keep-warm policy/action controls backed by explicit subscriber support, benchmark scheduler history/audit UI, consensus completion, Splunk dashboard expansion, Frontier Delta completion, and V2 Capability Endpoint implementation.

## Next Recommended Work

1. Add a real opt-in subscriber benchmark runner loop or dry-run harness once packaging and local prompt/fixture storage boundaries are ready.
2. Add keep-warm policy controls or model lifecycle actions only after explicit Managed Node subscriber support exists.
3. Add UI/API affordances for benchmark scheduler history or recent background tick audit summaries if operators need more troubleshooting visibility.
4. Keep future Capability Endpoint work documentation-only until V1 acceptance criteria are met, unless a current V1 implementation detail clearly blocks later additive expansion.

### 2026-07-09T18:21:03Z Current Status Pointer

- Latest completed slice: opt-in subscriber benchmark runner dry-run harness.
- Current runner state: implemented as disabled-by-default `dry_run_v1` subscriber-local plumbing that claims Managed Node jobs and reports metrics-only summaries.
- Phase B is not complete.
- Do not treat the older guidance-only runner entry as current status; it remains historical context for the previous slice.

## Next Recommended Work

1. Add keep-warm policy controls or model lifecycle actions only after explicit Managed Node subscriber support exists.
2. Add real subscriber-side synthetic prompt execution behind local-only packaging, with prompt text packaged exclusively on the subscriber and responses discarded locally.
3. Add UI/API affordances for benchmark scheduler history or recent background tick audit summaries if operators need more troubleshooting visibility.
4. Keep future Capability Endpoint work documentation-only until V1 acceptance criteria are met, unless a current V1 implementation detail clearly blocks later additive expansion.

### 2026-07-09T18:36:30Z Current Status Pointer

- Latest completed slice: keep-warm model lifecycle action queue with authenticated subscriber claim/status support.
- Current model lifecycle action state: implemented as metadata-only Managed Node keep-warm actions gated on approved, healthy, subscriber-reported warm-state and model-management support.
- Passive Endpoints remain inventory-only and have no lifecycle action controls.
- Phase B is not complete.
- Do not treat the older keep-warm next-work item above as current status; it remains historical context for the previous pointer.

## Next Recommended Work

1. Add real subscriber-side synthetic prompt execution behind local-only packaging, with prompt text packaged exclusively on the subscriber and responses discarded locally.
2. Add UI/API affordances for benchmark scheduler history or recent background tick audit summaries if operators need more troubleshooting visibility.
3. Add a small model lifecycle action history/filter surface if operators need to troubleshoot keep-warm action claims and completions.
4. Keep future Capability Endpoint work documentation-only until V1 acceptance criteria are met, unless a current V1 implementation detail clearly blocks later additive expansion.
