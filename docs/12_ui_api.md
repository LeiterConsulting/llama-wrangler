# UI API

The local UI is served at `/ui` and uses JSON management endpoints.

Current UI/API work is V1 Ollama-fleet scope. `/wrangler/nodes` remains the visible management surface for Managed Nodes and Passive Endpoints. Future versions may add a broader Capability Endpoint API or compatibility layer, but V1 should not add placeholder non-Ollama integration endpoints or UI pages before implementation starts.

## Setup

- `GET /wrangler/ui/bootstrap`
- `GET /wrangler/ui/status`
- `POST /wrangler/setup/start`
- `POST /wrangler/setup/scan-local`
- `POST /wrangler/setup/detect-ollama`
- `POST /wrangler/setup/discover-peers`
- `POST /wrangler/setup/apply-recommended`
- `POST /wrangler/setup/test-ollama`
- `POST /wrangler/setup/test-hec`
- `POST /wrangler/setup/complete`

## Configuration

- `GET /wrangler/config`
- `PUT /wrangler/config`
- `POST /wrangler/config/export`
- `POST /wrangler/support-bundle/export`

## Fleet

- `GET /wrangler/nodes`
- `POST /wrangler/enrollment-tokens`
- `POST /wrangler/nodes/manual-add`
- `POST /wrangler/nodes/passive-add`
- `GET /wrangler/benchmarks/workload-suites`
- `GET /wrangler/benchmarks/runner/guidance`
- `GET /wrangler/benchmarks/scheduler/policy`
- `PUT /wrangler/benchmarks/scheduler/policy`
- `GET /wrangler/benchmarks/scheduler/history`
- `POST /wrangler/nodes/:id/approve`
- `POST /wrangler/nodes/:id/revoke`
- `POST /wrangler/nodes/:id/trust`
- `POST /wrangler/nodes/:id/benchmark`
- `POST /wrangler/nodes/:id/enable`
- `POST /wrangler/nodes/:id/disable`
- `POST /wrangler/nodes/:id/overrides`
- `GET /wrangler/models`
- `GET /wrangler/models/lifecycle`
- `GET /wrangler/models/lifecycle/action-policies`
- `GET /wrangler/models/lifecycle/action-history`
- `POST /wrangler/nodes/:id/model-actions/keep-warm`
- `GET /wrangler/aliases`
- `PUT /wrangler/aliases`
- `POST /subscriber/enroll`
- `POST /subscriber/heartbeat`
- `POST /subscriber/model-actions/claim`
- `POST /subscriber/model-actions/status`

Fleet responses should keep control level, trust level, capability source, approval, freshness, and metadata-only capability fields explicit so future endpoint types can be added through additive migrations. Ollama-specific behavior should remain scoped to Ollama-compatible routes and node metadata.

## Model Lifecycle

`GET /wrangler/models/lifecycle` returns metadata-only model lifecycle and warm-state status. Managed Nodes can report model states through subscriber heartbeat using model fields such as name, state, keep-warm flag, token-rate summary, and load-time summary. Allowed lifecycle states are `installed`, `loading`, `loaded`, `warm`, `busy`, `unloaded`, `evicted`, `failed`, and `unknown`; invalid or suspicious values are normalized or redacted.

`GET /wrangler/ui/bootstrap` and `GET /wrangler/metrics` include `model_lifecycle`. The status summarizes model counts, warm models, loaded/loading/installed/failed states, keep-warm counts, warm-state-supported nodes, Passive Endpoint inventory-only nodes, model lifecycle action counts, and pending model lifecycle action counts. Node entries include control level, trust level, source, mode, support flags, model inventory source, safe model names/states, action counts, last action status, and reason codes.

`GET /wrangler/models/lifecycle/action-policies`, `GET /wrangler/ui/bootstrap`, and `GET /wrangler/metrics` include `model_lifecycle_actions`, a metadata-only policy status for operator visibility. The policy status reports the current action window, eligible node counts, blocked node counts, pending action counts, per-node supported action IDs, and reason codes such as `managed_subscriber_model_action_allowed`, `passive_no_model_management_control`, `node_not_approved`, `node_disabled`, `node_not_healthy`, `model_management_not_reported`, `warm_state_not_reported`, and `model_inventory_empty`. This is diagnostic policy metadata only; the queue endpoint remains the control path.

Managed Node lifecycle metadata is subscriber-reported and may include warm-state and model-management support flags. Passive Endpoints remain `inventory_only`: the marshal may observe model names from `/api/tags`, but it cannot inspect warm state, local load, eviction state, keep-warm settings, or model-management capabilities. Model lifecycle status, telemetry, bootstrap, metrics, support bundles, and UI must not include prompts, responses, raw headers, credentials, tokens, fixture contents, or payloads.

`POST /wrangler/nodes/:id/model-actions/keep-warm` queues a metadata-only keep-warm action for an eligible Managed Node. The request accepts `model_name` or `model` plus `keep_warm`. The node must be managed, approved, enabled, healthy, warm-state supported, model-management supported, and the model must already be present in subscriber-reported inventory. The response includes a safe action object with action ID, action type, model name, desired keep-warm state, status, timestamps, policy, and subscriber endpoint names. Passive Endpoints return `model_lifecycle_action_rejected` with `passive_no_model_management_control`.

`POST /subscriber/model-actions/claim` lets an authenticated Managed Node subscriber claim its next queued model lifecycle action. It accepts `node_id`, uses the same subscriber heartbeat credential semantics as heartbeats, marks the action `running`, and returns safe action metadata only. If no action is queued it returns `status: no_action`.

`POST /subscriber/model-actions/status` lets an authenticated Managed Node subscriber update a claimed action with `node_id`, `action_id`, `status`, and optional safe `error_code`. Allowed statuses are `queued`, `running`, `completed`, `failed`, and `cancelled`. Completing a keep-warm action updates the node's safe model metadata and recalculates lifecycle summary counts. Prompt text, response text, raw request bodies, raw headers, endpoint credentials, API keys, tokens, fixture contents, full fixture paths, and payloads must not be stored or returned.

`GET /wrangler/models/lifecycle/action-history` returns a newest-first metadata-only projection of the existing bounded Managed Node action records. It accepts optional `status`, `node_id`, and `limit` query parameters. Status must be one of `queued`, `running`, `completed`, `failed`, or `cancelled`; limit defaults to 20 and is bounded to 1-50. Each node retains at most eight recent actions. Summary counts describe all matches before the response limit is applied.

Action-history rows include action ID/type/policy, Managed Node control/trust/approval metadata, safe model name, desired keep-warm state, current status, queue/claim/update/completion/failure timestamps, and a normalized safe error code only. Passive Endpoints are excluded even if malformed legacy state contains action-like metadata. Unknown action fields and unsafe error strings are not projected. `GET /wrangler/ui/bootstrap` and `GET /wrangler/metrics` include the default recent view as `model_lifecycle_action_history`; the Dashboard can request filtered views without changing persisted state.

## Operations

- `GET /wrangler/routing/policies`
- `PUT /wrangler/routing/policies`
- `GET /wrangler/client-presets`
- `GET /wrangler/auth/status`
- `POST /wrangler/auth/admin-token/rotate`
- `POST /wrangler/auth/api-keys`
- `POST /wrangler/auth/api-keys/:id/rotate`
- `POST /wrangler/auth/api-keys/:id/revoke`
- `POST /wrangler/secrets/rekey`
- `GET /wrangler/telemetry/status`
- `PUT /wrangler/telemetry/splunk-hec`
- `POST /wrangler/telemetry/test-hec`
- `GET /wrangler/audit/recent`
- `GET /wrangler/metrics`

## Authentication

Before setup completion, local management endpoints remain open to keep first-run setup smooth on localhost.

After setup completion:

- management endpoints require `Authorization: Bearer <admin-token>`
- inference endpoints require `Authorization: Bearer <client-api-key>` when client auth is enabled
- generated tokens are shown once and only hints are stored in ordinary app state
- admin tokens can be rotated from Settings
- client API keys can be generated, regenerated, or revoked from IDE setup
- repeated invalid admin or client API-key attempts are rate limited per remote address
- rate-limited responses return `429` with `Retry-After`

Management rate-limit responses use UI-friendly JSON such as `{"error":"admin_auth_rate_limited","message":"..."}`. Inference rate-limit responses preserve API-family compatibility: `/v1/*` returns OpenAI-style `rate_limit_error` objects and `/api/*` returns Ollama-style string `error` plus safe `type` and `code` metadata.

## Network Exposure

`GET /wrangler/ui/bootstrap` includes `safe_defaults` fields for LAN posture:

- `marshal_listen`
- `lan_access_by_default`
- `lan_access_enabled`
- `lan_requires_explicit_enablement`
- `lan_access_warning`

Localhost and loopback listen addresses are treated as the safe default. Any all-interface, non-loopback IP, or non-local hostname listen address is surfaced as LAN exposure so Settings can show an explicit warning.

## Peer Discovery

`POST /wrangler/setup/discover-peers` performs an explicit operator-initiated discovery pass. Current behavior is conservative:

- runs a short, one-shot mDNS/Bonjour query for Llama Wrangler and Ollama service names
- returns discovery candidates as metadata only
- does not persist candidates as nodes
- does not approve candidates
- does not route traffic to candidates
- does not send prompt or response payloads
- does not perform subnet scanning

The response includes:

- `mode`: currently `operator_initiated_mdns`
- `mdns`: enabled/status/services/timeout/message metadata
- `subnet_scan`: always disabled until a future separate opt-in flow exists
- `candidates`: possible services with ID, display name, service name, host/port/address metadata when advertised, default `trust_level: lan_unverified`, `approval_state: not_added`, and adoption path
- `adoption`: operator guidance for Managed Node enrollment or Passive Endpoint add
- `warnings`: safety notes

Discovery candidates must be adopted through the existing Managed Node enrollment-token flow or Passive Endpoint add flow. Discovery responses, telemetry, support bundles, and UI state must not include endpoint credentials, auth headers, enrollment tokens, heartbeat credentials, token hashes, admin tokens, client API keys, HEC tokens, prompts, responses, request bodies, or payloads.

## Splunk TLS Warning

`GET /wrangler/ui/bootstrap` and `GET /wrangler/telemetry/status` include Splunk HEC TLS posture metadata under `telemetry.splunk_hec` / `splunk_hec`:

- `verify_ssl`
- `tls_verification_disabled`
- `tls_warning`
- `has_token`

When `verify_ssl` is `false`, the Splunk UI must show an explicit warning that certificate verification is disabled and should only be used for trusted self-signed Splunk lab certificates. The warning metadata must not include HEC token values, admin tokens, client API keys, raw headers, prompts, responses, request bodies, or payloads.

## Splunk Operations Assets

The companion Splunk app packages Overview and Operations Simple XML dashboards. Operations panels cover fixed consensus failure reasons, queue scheduling, streaming retry/partial/cancellation outcomes, benchmark scheduler and runner history, routing policy exclusions, and model lifecycle action history. Packaged macros, eventtypes, props categories, navigation, and disabled-by-default saved reports support the same domains.

Splunk searches use only service-emitted metadata fields. Routing policy panels unpack `routing_decision.excluded_nodes` reason/control/trust/approval metadata; consensus panels reference only the fixed participant failure vocabulary. No Splunk asset may query inference content, raw request/response material, raw headers, authorization data, credentials, local fixture contents or full paths, comparison signatures, validator/evaluator input, or arbitrary payload fields.

## Client Presets

`GET /wrangler/ui/bootstrap` includes `client_presets`, and `GET /wrangler/client-presets` returns the same preset list for the current request host.

Preset cards cover:

- Cline
- Continue
- Open WebUI
- generic OpenAI SDK clients

Preset metadata includes the derived OpenAI-compatible base URL, selected model alias, display fields, and copyable snippet bodies. API keys are represented by `<client-api-key>` placeholders in API responses. The UI may substitute the browser-local one-time generated key for display/copy, but stored client API keys, admin tokens, HEC tokens, prompts, responses, request bodies, and headers must not be returned by preset APIs.

## Node Control Metadata

`GET /wrangler/ui/bootstrap`, `GET /wrangler/nodes`, and support bundles include node control metadata for Phase B planning and UI badges.

Node records can include:

- `control_level`: `managed` or `passive`
- `trust_level`: `local`, `lan_trusted`, `lan_unverified`, or `external`
- `capability_source`: `subscriber_reported`, `marshal_observed`, or `manual`
- `approval_state`: `pending`, `approved`, `rejected`, or `revoked`
- `health_source`
- `model_inventory_source`
- `benchmark_source`
- `warm_state_supported`
- `management_supported`
- `telemetry_level`
- `last_observed_at`
- `last_reported_at`

These fields are metadata only. They must not contain enrollment tokens, API keys, HEC tokens, future provider keys, raw headers, prompt bodies, response bodies, request bodies, or payloads.

Current behavior is intentionally conservative: existing local and previously manually added subscriber records migrate as Managed Node records. New manual subscriber adds remain available only as a compatibility fallback through `POST /wrangler/nodes/manual-add`; they validate the subscriber URL, fetch metadata-only capabilities when available, and create pending Managed Nodes that require explicit operator approval before routing. Passive Endpoint records can be added through `POST /wrangler/nodes/passive-add`, which requires endpoint URL, explicit trust level, and successful safe `/api/tags` validation. Passive Endpoint records use marshal-observed metadata, pending approval, no warm-state support, and no model-management support.

Managed Node subscriber enrollment uses a short-lived token flow:

- `POST /wrangler/enrollment-tokens` creates a one-time enrollment token for operator display
- the raw token is returned only in that response
- ordinary app state stores only token hash, token hint, expiry, control/trust metadata, and approval metadata
- `GET /wrangler/ui/bootstrap` and support bundles expose sanitized enrollment queue metadata without token hashes
- `POST /subscriber/enroll` consumes the token and registers a Managed Node as `approval_state: pending`
- pending Managed Nodes must be explicitly approved before routing
- successful subscriber enrollment provisions a heartbeat shared-secret credential in the encrypted secret backend and exposes only safe auth metadata plus token hint

`POST /wrangler/enrollment-tokens` accepts:

- `node_id`: optional expected node ID
- `subscriber_url`: optional absolute `http` or `https` subscriber URL without embedded credentials
- `trust_level`: optional trust level, defaulting to `lan_unverified`
- `ttl_minutes`: optional token lifetime; invalid, omitted, or over-24-hour values fall back to a short default

`POST /subscriber/enroll` accepts subscriber metadata such as token, node ID, subscriber URL, hostname, platform, architecture, Ollama availability, version, tags, and model inventory. The token must not be persisted in node records, telemetry, support bundles, bootstrap responses, or API responses after the one-time generation response.

On successful enrollment, the marshal derives a heartbeat shared-secret credential from the one-time enrollment token and final node ID, stores that derived credential only in the configured secret backend, and marks the node with safe metadata such as `heartbeat_auth_method`, `heartbeat_auth_required`, `heartbeat_token_hint`, and `heartbeat_credential_derivation`. The raw heartbeat credential must not be returned by bootstrap, support bundles, node state, telemetry, or UI warning metadata. Subscribers can derive the same credential before discarding the enrollment token.

`POST /wrangler/nodes/:id/heartbeat-credential/rotate` provisions or rotates a Managed Node heartbeat shared secret. It is intended for legacy/manual Managed Nodes and for explicit operator rotation of already provisioned nodes. The endpoint returns the raw `credential` only in the immediate rotation response, stores it only in the configured secret backend, and updates node metadata with safe fields such as `heartbeat_auth_method`, `heartbeat_auth_required`, `heartbeat_token_hint`, `heartbeat_credential_derivation`, and `heartbeat_reprovisioning_required`. Passive Endpoints are rejected. After rotation, subscribers must use the new credential in `X-Llama-Wrangler-Subscriber-Token` or `Authorization: Bearer <credential>`; old credentials and missing credentials are rejected. Bootstrap, support bundles, telemetry, node state, and UI metadata must not include the raw credential.

Rotation responses also include a `subscriber_install` guidance object with safe installation metadata:

- `environment_variable`: `LLAMA_WRANGLER_SUBSCRIBER_HEARTBEAT_CREDENTIAL`
- `config_key`: `registration.heartbeat_credential_env`
- `env_file_path` and placeholder-based `env_file_template`
- placeholder-based shell export, launchd dry-run, launchd plist template, install commands, validation commands, uninstall commands, and heartbeat-check commands
- `service_wrapper` metadata for launchd, including label, plist path, log directory, mode, config path, environment variables, and wrapper notes
- restart and verification notes

The `subscriber_install` commands and service-wrapper artifacts use `<credential-from-rotation-response>` placeholders and must not duplicate the raw credential. The UI may offer copy actions that substitute the immediate-response credential in browser memory, but the raw value must still not be persisted in ordinary app state, bootstrap responses, telemetry, support bundles, or node metadata. These artifacts are manual operator guidance for the subscriber host; the marshal must not remotely mutate subscriber machines, write subscriber config files, install launchd plists, or start services.

`POST /wrangler/nodes/manual-add` accepts:

- `node_id`: optional operator-selected node ID
- `url`: absolute `http` or `https` subscriber URL without embedded credentials

Manual add is an approval-gated compatibility path, not the preferred enrollment path. It creates a Managed Node with `approval_state: pending`, `approved: false`, and metadata marking it as manually added. It must not forward client authorization headers to the subscriber capability probe, and it must not persist endpoint credentials, raw headers, prompts, responses, request bodies, or payloads. Operators should prefer the one-time enrollment-token flow for new subscribers.

`POST /subscriber/heartbeat` accepts metadata-only updates from an already registered Managed Node. Heartbeats refresh `last_reported_at`, subscriber-reported health, Ollama availability/version, model inventory, tags, active jobs, queue depth, memory totals, and host/platform metadata. Enrolled subscribers with a stored heartbeat credential must authenticate heartbeats with `X-Llama-Wrangler-Subscriber-Token` or `Authorization: Bearer <credential>`. Heartbeat payloads must not include enrollment tokens, admin tokens, client API keys, HEC tokens, future provider keys, raw headers, prompt bodies, response bodies, request bodies, or payload content.

Managed Nodes registered through subscriber enrollment are heartbeat-required. A heartbeat-required Managed Node with no `last_reported_at` is treated as missing, and a heartbeat older than the marshal freshness window is treated as stale. Missing or stale heartbeat-required Managed Nodes are excluded from routing until a fresh heartbeat arrives. Legacy/manual Managed Nodes without `observed.heartbeat_required: true` remain eligible under the existing approval, health, model, control, and trust policies until an operator explicitly provisions a heartbeat credential. Legacy/manual Managed Nodes without a stored heartbeat credential remain compatibility-unverified until they re-enroll or receive credential rotation/provisioning.

Subscriber config can reference the rotated credential through an environment variable instead of storing plaintext in YAML:

```yaml
registration:
  marshal_url: "http://<marshal-host>:11435"
  heartbeat_credential_env: LLAMA_WRANGLER_SUBSCRIBER_HEARTBEAT_CREDENTIAL
```

The subscriber runtime resolves `registration.heartbeat_credential_env` from the process environment. Support bundles and sanitized config exports may include the env-var name, but not the resolved credential.

`POST /wrangler/nodes/passive-add` accepts:

- `endpoint_url`: absolute `http` or `https` URL without embedded credentials
- `display_name`: optional UI label
- `trust_level`: required explicit trust level

The validation request must not forward client authorization headers or request payloads. Client-facing validation failures should be generic and must not echo raw headers, tokens, prompts, responses, request bodies, or payloads.

`POST /wrangler/nodes/:id/approve` marks a node or endpoint as `approval_state: approved`, sets `approved: true`, and enables it for routing if other health/model checks pass.

`POST /wrangler/nodes/:id/revoke` marks a node or endpoint as `approval_state: revoked`, sets `approved: false`, disables it, and removes it from routing eligibility.

`POST /wrangler/nodes/:id/trust` updates an existing node or endpoint `trust_level`. It accepts only:

- `local`
- `lan_trusted`
- `lan_unverified`
- `external`

The Nodes UI must warn for `lan_unverified` and `external` trust levels. Trust updates are metadata-only and must not include or persist raw headers, tokens, prompts, responses, request bodies, or payloads.

Routing must require both `enabled: true` and approved state. Pending, rejected, revoked, disabled, failed, or otherwise unapproved nodes must not be selected for inference routes, including Passive Endpoints that validated successfully but have not been approved.

Routing also applies control/trust metadata:

- `external` trust is excluded by default unless a future explicit policy allows it
- heartbeat-required Managed Nodes with missing or stale subscriber reports are excluded until a fresh heartbeat arrives
- `lan_unverified` trust remains eligible for approved single-route requests but is de-prioritized and reported in routing metadata
- Passive Endpoints remain eligible for approved single-route requests but are de-prioritized versus comparable Managed Nodes
- consensus-mode candidates must be Managed Nodes with `local` or `lan_trusted` trust
- Passive Endpoints and `lan_unverified` nodes are excluded from consensus by default

Routing telemetry may include node IDs, control level, trust level, capability source, and policy reason codes. It must not include endpoint credentials, auth headers, prompts, responses, request bodies, or payloads.

## V1 Non-Streaming Consensus

An alias with `execution_mode: consensus` or `consensus_delta`, or a consensus strategy, uses bounded non-streaming fan-out. Missing participant bounds default to `min_participants: 2` and `max_participants: 4`; maximum fan-out is capped at 8. Routing first applies the existing approval, enabled/health, model, Managed Node, trust, and heartbeat-freshness policy. `max_participants` is applied after routing and benchmark-placement ordering.

The marshal concurrently buffers successful participant responses in request memory, with an 8 MiB limit per response and the configured routing timeout. Normalized text or JSON structural matches form agreement groups. A strict majority is consensus. When successful responses meet `min_participants` but no majority exists, the earliest routing-ranked successful participant is returned deterministically and the disagreement is recorded in metadata-only telemetry. Regex validator and local evaluator interfaces are additive hooks; the default evaluator is a no-op.

The winning upstream OpenAI/Ollama body, status, and content type are returned through the requesting inference API without consensus body rewriting. Safe consensus headers are returned only when `X-Llama-Wrangler-Debug: true` or request field `debug: true` is explicitly set. Prompt/response bodies, extracted content, comparison signatures, validator input, and evaluator input are never written to state, telemetry, management APIs, logs, or support bundles.

V1 streaming consensus deliberately returns compatibility error code `consensus_streaming_not_supported` before upstream fan-out. This is a product safety decision, not a claim that the existing non-streaming buffer can be reused for streams. A future implementation requires a separate bounded protocol for per-participant stream collection, quorum timing, deterministic aggregation, backpressure, cancellation, response commitment, and no-retry-after-partial-output guarantees. If fewer than the required non-streaming participants succeed, the response uses `consensus_insufficient_successful_participants`. Request cancellation cancels in-flight participants. `consensus_delta` may set `escalation_recommended`, but no Frontier request is performed.

## Routing Policy Status

`GET /wrangler/ui/bootstrap` and `GET /wrangler/metrics` include `routing_policy_status`, a metadata-only summary for Dashboard and Nodes UI warnings.

The status object includes:

- `window`: currently `current_node_metadata`
- `summary`: counts by severity, scope, and reason code
- `warnings`: per-node policy warnings with node ID, severity, scope, code, message, control level, trust level, approval state, and capability source

Warning codes can include:

- `node_not_approved`
- `node_disabled`
- `trust_external_excluded`
- `passive_consensus_excluded`
- `trust_lan_unverified_deprioritized`
- `trust_lan_unverified_consensus_excluded`
- `heartbeat_missing`
- `heartbeat_stale`

Dashboard and Nodes render these warnings so operators can see why a node is blocked, de-prioritized, or excluded from consensus without generating an inference request. This metadata must not include endpoint credentials, auth headers, enrollment tokens, token hashes, admin tokens, client API keys, HEC tokens, prompts, responses, request bodies, or payloads.

`operation_stats.consensus` in bootstrap and metrics summarizes recent metadata-only consensus events: total, reached, no-majority, failed, timed-out, cancelled, streaming-rejected, last participant/success counts, last agreement score, and last winner node. The Dashboard renders these counts in Consensus Outcomes.

## Benchmark Policy Status

`GET /wrangler/ui/bootstrap` and `GET /wrangler/metrics` include `benchmark_policy_status`, a metadata-only summary for Dashboard and Nodes UI benchmark posture.

The status object includes:

- `window`: currently `current_node_metadata`
- `summary`: counts by benchmark eligibility, placement eligibility, benchmark mode, and reason code
- `nodes`: per-node benchmark policy metadata with node ID, control level, trust level, approval state, benchmark source, benchmark eligibility, mode, reason codes, operator message, placement eligibility, placement freshness window, placement reason codes, and placement operator message

Benchmark policy modes currently include:

- `subscriber_reported`: eligible Managed Node benchmark queue work, after approval and health checks
- `marshal_observed_probe_only`: Passive Endpoint benchmark posture; no local benchmark control is implied or available

Benchmark-derived routing placement applies only after normal routing gates have already accepted a candidate. Placement uses fresh subscriber-reported Managed Node benchmark summaries from approved, enabled, trusted nodes. Current freshness window is 24 hours. Placement ignores Passive Endpoint marshal-observed probes, `lan_unverified` and `external` trust levels, stale summaries, incomplete summaries, and summaries without token-rate metadata. Reason codes include:

- `benchmark_placement_applied`
- `benchmark_placement_fresh`
- `benchmark_placement_passive_probe_ignored`
- `benchmark_placement_untrusted_ignored`
- `benchmark_placement_source_ignored`
- `benchmark_placement_summary_missing`
- `benchmark_placement_summary_stale`
- `benchmark_placement_status_ignored`
- `benchmark_placement_freshness_missing`
- `benchmark_placement_rate_missing`

`GET /wrangler/benchmarks/workload-suites` returns safe benchmark workload suite definitions. Definitions contain suite IDs, display names, source, input policy, fixture policy, task IDs/categories, expected result metric names, and warnings. They must not contain prompt text, response text, raw request bodies, raw headers, endpoint credentials, API keys, tokens, or payload content.

Current workload suite IDs:

- `synthetic_smoke_v1`: built-in subscriber synthetic smoke tasks
- `synthetic_code_v1`: built-in subscriber synthetic code-oriented tasks
- `operator_local_fixtures_v1`: operator-provided local fixture manifest, referenced by manifest ID only

`POST /wrangler/nodes/:id/benchmark` creates a Managed Node benchmark job only for eligible Managed Nodes. The optional body accepts:

- `suite_id`: benchmark workload suite ID, defaulting to `synthetic_smoke_v1`
- `fixture_manifest_id`: required when `suite_id` is `operator_local_fixtures_v1`
- `fixture_id`: accepted as an alternate local fixture manifest reference
- `fixture_path`: accepted only to derive a basename hint; the full path and fixture contents must not be persisted

The response includes a safe `job` object with benchmark ID, node ID, job type, source, mode, status, requested/updated timestamps, model candidates, workload suite metadata, and subscriber endpoints for status/result reporting. Workload suite metadata includes suite ID, source, input policy, fixture policy, task count, task IDs, result metric names, and optional local fixture manifest ID or basename hint. It must not include prompt text, response text, raw request bodies, raw headers, endpoint credentials, API keys, tokens, or payload content. Passive Endpoints return `benchmark_policy_rejected` with `passive_no_local_benchmark_control` and remain limited to marshal-observed probes. Benchmark policy telemetry must not include endpoint credentials, auth headers, enrollment tokens, heartbeat credentials, token hashes, admin tokens, client API keys, HEC tokens, prompts, responses, request bodies, or payloads.

`POST /subscriber/benchmarks/claim` lets an authenticated Managed Node subscriber claim its next queued benchmark job. It accepts `node_id`, requires the same subscriber heartbeat credential semantics when a stored credential exists, marks the job `running`, and returns the safe job metadata, including workload suite IDs and task IDs only. If no job is queued it returns `status: no_job`.

`POST /subscriber/benchmarks/status` lets an authenticated Managed Node subscriber update a claimed benchmark job status with `node_id`, `benchmark_id`, `status`, and optional safe `error_code`. Allowed statuses are `queued`, `running`, `completed`, `failed`, and `cancelled`.

`POST /subscriber/benchmarks` accepts authenticated Managed Node subscriber benchmark results. Nodes with stored heartbeat credentials must authenticate this endpoint the same way as heartbeats, using `X-Llama-Wrangler-Subscriber-Token` or `Authorization: Bearer <credential>`. Accepted fields are limited to benchmark ID, model name, status, timestamps, duration, token counts, token-rate metrics, error code, suite ID, task count, local fixture manifest ID, and safe runner mode. Extra prompt, response, raw request, raw header, or payload fields must not be stored. The marshal stores only summarized benchmark metadata under the node, may update model tokens/sec, and completes the matching benchmark job when `benchmark_id` matches a claimed or queued job.

`GET /wrangler/ui/bootstrap` includes `benchmark_workload_suites` plus `benchmark_workload` summary metadata so the UI can show suite cards and choose a suite when queueing a Managed Node benchmark. `GET /wrangler/metrics` includes only `benchmark_workload` summary metadata. Both surfaces must keep prompts, responses, request bodies, raw headers, endpoint credentials, API keys, tokens, and payloads out.

`GET /wrangler/benchmarks/runner/guidance` returns subscriber-side benchmark runner guidance, opt-in runner status, and packaging-hook metadata. The current implementation is `subscriber_local_runner_loop_available`: it can run in subscriber mode only when explicitly enabled, claim Managed Node benchmark jobs, send status transitions, and report metric summaries only. `dry_run_v1` remains the default and derives deterministic metric summaries from suite/task metadata without loading prompt text. `synthetic_builtin_v1` resolves built-in synthetic suite/task IDs to packaged subscriber-local prompts, sends those prompts only to the subscriber-local Ollama `/api/generate` endpoint, derives bounded timing/token metrics, and discards response bodies before reporting. Operator local fixture execution remains disabled until a separate safe manifest parser/storage boundary is implemented. The guidance includes supported suite IDs, subscriber claim/status/result endpoints, placeholder-only commands, environment variable names, config snippets, bounded runner settings, expected inputs/outputs, local fixture manifest guidance, and warnings. It must not include prompt text, response text, fixture contents, full fixture paths, raw request bodies beyond placeholder examples, raw credentials, admin tokens, client API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, future provider keys, or payloads.

`GET /wrangler/ui/bootstrap` and `GET /wrangler/metrics` include `benchmark_runner` so operators can see subscriber runner status, mode, result body policy, poll interval, max jobs per tick, and packaging boundaries. The marshal does not remotely install packages, write fixture manifests, mutate subscriber service wrappers, execute prompt workloads, or inspect Passive Endpoints through this guidance. Real synthetic execution is opt-in subscriber-side behavior and must report metric summaries only.

`GET /wrangler/benchmarks/scheduler/policy` returns the active benchmark scheduler policy config plus allowed numeric limits. `PUT /wrangler/benchmarks/scheduler/policy` accepts:

- `policy`: currently only `bounded_retry_timeout_v1`
- `max_attempts`: bounded retry attempt count
- `lease_timeout_seconds`: bounded running-job lease timeout
- `retry_delay_seconds`: bounded retry-wait delay after retryable failures/timeouts
- `background_enabled`: optional background reconciliation toggle, disabled by default
- `tick_interval_seconds`: bounded background reconciliation interval

Invalid, missing, too-small, too-large, or unknown values are normalized into safe bounds before persistence. Defaults are `max_attempts: 3`, `lease_timeout_seconds: 600`, `retry_delay_seconds: 60`, `background_enabled: false`, and `tick_interval_seconds: 60`. Current bounds are max attempts 1-10, lease timeout 30-3600 seconds, retry delay 5-1800 seconds, and background tick interval 10-3600 seconds. Policy responses and telemetry are metadata-only and must not include prompts, responses, request bodies, raw headers, endpoint credentials, API keys, tokens, fixture contents, full fixture paths, or payloads.

`GET /wrangler/ui/bootstrap` and `GET /wrangler/metrics` include `benchmark_scheduler`, a metadata-only summary of durable Managed Node benchmark jobs, scheduler policy config, numeric limits, and background reconciliation status. Background status includes enablement, tick interval, next tick timestamp, last tick timestamp, last reason, and last-run summary counts only. The scheduler policy is `bounded_retry_timeout_v1`. Jobs include safe metadata such as benchmark ID, node ID, status, scheduler state/reason, attempt count, max attempts, lease timeout seconds, retry delay seconds, workload suite ID/source, next-attempt timestamp, timeout timestamp, and updated timestamp.

`POST /wrangler/benchmarks/scheduler/reconcile` performs an admin-triggered scheduler reconciliation. It updates timed-out running jobs, moves due retry-wait jobs back to queued, and marks jobs exhausted when max attempts are reached. The response includes only summary counts and the same sanitized scheduler status. Subscriber claims also reconcile that node before selecting the next queued job, so persisted jobs recover across process restarts without storing payloads.

`GET /wrangler/benchmarks/scheduler/history` returns a bounded, newest-first audit of recent automatic background ticks and operator-triggered reconciliations. Each entry contains only the recorded timestamp, trigger, reason, normalized scheduler policy metadata, and changed/timed-out/retried/exhausted counts. The history is process-local, retains at most 24 entries, and resets on service restart. The same history summary is included under `benchmark_scheduler.history` in bootstrap and metrics for the Dashboard.

When `background_enabled` is true, the service periodically runs the same reconciliation logic at the bounded `tick_interval_seconds`. Background ticks do not create new benchmark jobs, execute prompt workloads, inspect Passive Endpoints, or mutate remote subscribers. They only update existing Managed Node benchmark job scheduler metadata and emit metadata-only telemetry with changed/timed-out/retried/exhausted counts.

Scheduler behavior:

- queued jobs are claimable only when `next_attempt_at` is absent or due
- claimed jobs move to `running`, increment `attempt`, and receive a `timeout_at` lease deadline based on their job policy metadata
- failed jobs with remaining attempts enter `retry_wait` until `next_attempt_at` based on their job policy metadata
- timed-out running jobs retry while attempts remain
- failed or timed-out jobs at max attempts become `failed` with `scheduler_state: exhausted`
- completed and cancelled jobs become terminal
- Passive Endpoints remain excluded from local-control benchmark jobs and scheduler reconciliation

`POST /wrangler/nodes/:id/benchmark-probe` is available only for Passive Endpoints. It performs a marshal-observed `/api/tags` availability/latency probe and records metadata such as duration, model count, status, and error code. It must not imply hardware inspection, local benchmark execution, local load inspection, warm-state inspection, model management, prompt workloads, or subscriber control. The existing `POST /wrangler/nodes/:id/benchmark` route continues to reject Passive Endpoints.

Benchmark workload suite, job, scheduler, reconciliation-history, execution, status, result, and placement metadata must not include endpoint credentials, auth headers, enrollment tokens, heartbeat credentials, token hashes, admin tokens, client API keys, HEC tokens, prompts, responses, fixture contents, full fixture paths, request bodies, raw headers, or payloads.

## Inference Proxy Semantics

For OpenAI-compatible and Ollama-compatible inference endpoints, marshal retries fallback nodes only before client-visible output begins.

- streaming requests may retry after upstream connection failures, 5xx responses, or body-read failures before the first token
- streaming requests must not retry after any partial output has been written to the client
- non-streaming responses are buffered before commit so upstream read failures can still retry safely
- cancellation, retry, and partial-output events are emitted as metadata-only telemetry
- OpenAI-compatible streaming preserves upstream `text/event-stream` SSE chunks so SDK-style clients can read `data: ...` lines before upstream completion
- Ollama-compatible streaming preserves upstream newline-delimited JSON chunks so Ollama-style clients can read one JSON object per line before upstream completion

## Queue Metadata

`GET /wrangler/ui/bootstrap` and `GET /wrangler/metrics` include queue metadata for UI visibility:

- `max_depth`
- `active`
- `waiting`
- `available`
- `current`
- `recent`
- `priorities`
- `scheduling`

Queue entries may include request ID, priority, status, API surface, requested model, session ID, stream flag, timestamps, and capacity/depth metadata. Queue entries must not include prompt text, response text, request bodies, headers, API keys, or other secrets.

Inference clients can set `X-Llama-Wrangler-Priority: high|normal|low`. If no header is present, JSON request fields `priority` or `queue_priority` may provide the same metadata.

Queue dispatch can use:

- `weighted_priority`, the default, which dispatches waiting requests by configured high/normal/low weights
- `fifo`, which dispatches waiting requests in arrival order

Dashboard queue scheduling controls write to `PUT /wrangler/routing/policies` using `queue_scheduling_policy` and `queue_priority_weights`. Priority, scheduling policy, and weights are metadata-only; they must not include payloads, prompts, responses, headers, or secrets.

## Operation Stats

`GET /wrangler/ui/bootstrap` and `GET /wrangler/metrics` include `operation_stats` derived from recent metadata-only audit events:

- `window`
- `audit_events`
- `retries.total`
- `retries.before_first_token`
- `retries.last_at`
- `partials.total`
- `partials.after_partial`
- `partials.last_at`
- `cancellations.total`
- `cancellations.before_first_token`
- `cancellations.after_partial_output`
- `cancellations.before_queue`
- `cancellations.last_at`
- `consensus.total`
- `consensus.reached`
- `consensus.no_majority`
- `consensus.failed`
- `consensus.timed_out`
- `consensus.cancelled`
- `consensus.streaming_rejected`
- `consensus.failure_reasons`

These counters summarize `upstream_retry`, `response_partial`, `request_cancelled`, and `consensus` events for the operations UI. Consensus failure reasons are aggregated only from the fixed vocabulary `missing_proxy_url`, `connection_error`, `upstream_4xx`, `upstream_5xx`, `body_read_failure`, `response_size_limit`, `timeout`, and `cancellation`; unknown keys are ignored. The counters must not include prompt text, response text, extracted content, arbitrary error text, endpoint URLs, request bodies, raw headers, API keys, HEC tokens, provider keys, or other secrets.

### V1 non-streaming consensus outcomes

Consensus fan-out continues deterministic aggregation when at least the configured minimum number of participants succeeds, even when other participants fail. Safe participant failure metadata contains node ID, fixed reason code, optional numeric upstream status, and duration only.

When required successes are unmet, `/v1/*` returns the OpenAI error object and `/api/*` returns the Ollama error object. If every participant fails with an upstream 4xx before any success, the normalized upstream status/code is retained. Timeout-driven unmet quorum returns HTTP 504 with `consensus_insufficient_successful_participants`; other unmet quorum returns HTTP 502 with the same safe code. Upstream response/error bodies are never relayed in these failure responses.

Streaming consensus remains explicitly unsupported for V1 and returns `consensus_streaming_not_supported` before participant fan-out. Real-client tests prove rejection for OpenAI and Ollama compatibility routes without participant requests. Single-route OpenAI SSE and Ollama JSONL streaming behavior is unchanged. Streaming consensus may be reconsidered only through a separate protocol with bounded buffering/backpressure, quorum timing, deterministic aggregation, cancellation, response-commit, and no-retry-after-partial-output acceptance tests.

Real-client non-streaming consensus compatibility tests start real marshal and participant HTTP listeners and use `net/http` response readers. They cover OpenAI and Ollama partial-success aggregation, routing-ranked winner shape, normalized all-upstream-4xx responses, immediate HTTP 502 quorum failure, timeout-driven HTTP 504 quorum failure, and prompt/response/upstream-error marker exclusion from audit state, bootstrap, metrics, and support bundles.

## Error Compatibility

Inference endpoint errors are normalized by API family:

- `/v1/*` returns OpenAI-style errors: `{"error":{"message","type","param","code"}}`
- `/api/*` returns Ollama-style string errors with safe metadata: `{"error":"...","type":"...","code":"..."}`
- management/UI endpoints keep the existing UI-friendly JSON error maps

Error responses must not echo prompt bodies, response bodies, raw headers, API keys, HEC tokens, upstream URLs, or other secrets. Upstream transport failures are reported with generic client-facing messages and metadata-safe error codes.

## Support Bundle

`POST /wrangler/support-bundle/export` returns a sanitized JSON troubleshooting bundle for local support workflows.

Each bundle includes versioned support-bundle schema metadata:

- `bundle_schema.name`: `llama-wrangler.support-bundle`
- `bundle_schema.version`: support-bundle contract version
- `bundle_schema.json_schema`: `schemas/support_bundle.schema.json`
- `bundle_schema.documentation`: `docs/13_support_bundle_schema.md`
- `bundle_schema.compatibility`: `additive_backward_compatible`

The bundle can include:

- generated timestamp and service version
- support-bundle schema metadata
- role, node ID, setup status, schema version, config version, and migration history
- sanitized runtime config
- node and session metadata
- queue snapshot
- metadata-only audit events
- secret-storage status metadata
- privacy flags describing excluded data classes

The bundle must not include admin tokens, client API keys, HEC tokens, provider keys, enrollment token hashes, prompt bodies, response bodies, raw request bodies, raw headers, payload fields, or other secrets.

Downstream tooling should parse `bundle_schema` first. `bundle_schema.version` is distinct from the top-level service `version`, `service.schema_version`, and `service.config_version`.

## State Metadata

`GET /wrangler/ui/bootstrap` returns:

- `schema_version`
- `config_version`
- `migration_history`
- `secret_storage`

Settings displays the active schema and config versions so persisted app-data upgrades are visible during support and troubleshooting.

`secret_storage` is status metadata only. It can include the storage backend, encrypted status, legacy migration status, and key source, but it must not include admin tokens, client API keys, HEC tokens, provider keys, or secret payloads.

Secret-storage status can include backup/restore guidance metadata:

- `fallback_backend`
- `fallback_available`
- `backup_required_files`
- `backup_description`
- `restore_description`
- `backup_warnings`
- `os_keychain_status`
- `os_keychain_plan`
- `os_keychain_next_step`
- `os_keychain_migrated`
- `os_keychain_platform`
- `os_keychain_runtime`
- `os_keychain_service_mode`
- `os_keychain_warning`

For `key_source: file`, `backup_required_files` names `secrets.enc.json` and `secrets.key`. For `key_source: env`, it names `secrets.enc.json` and `LLAMA_WRANGLER_SECRETS_KEY`. These are instructional names only; responses must not include plaintext secret values, local absolute paths, admin tokens, client API keys, HEC tokens, provider keys, raw headers, prompts, responses, request bodies, or payloads.

The OS keychain fields are status metadata only. When `LLAMA_WRANGLER_SECRET_BACKEND=os_keychain` is set and the backend is usable, `secret_storage.backend` can report `os_keychain` while `fallback_available` remains true. If the keychain backend is unavailable, `secret_storage.backend` reports `encrypted_file` and `os_keychain_status` reports `unavailable`. Service-like runtime warnings are advisory metadata so operators know keychain access may differ under launchd, systemd, Windows service users, or future install helpers. These fields must never include keychain item contents or encrypted fallback file contents.

`POST /wrangler/secrets/rekey` rotates the local encrypted fallback key when `secret_storage.rekey_supported` is true. The endpoint returns metadata only:

- `rekeyed`
- `rotated_at`
- `status.backend`
- `status.encrypted`
- `status.key_source`
- `status.rekey_supported`

If the key source is external, such as `LLAMA_WRANGLER_SECRETS_KEY`, the endpoint returns `409 secret_rekey_unsupported` and does not modify local secret files. Rekey responses must not include admin tokens, client API keys, HEC tokens, provider keys, or secret payloads.
