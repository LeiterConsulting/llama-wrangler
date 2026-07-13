# Phase B Managed Node and Passive Endpoint Plan

Phase B begins with a clear node-control model instead of treating every reachable Ollama URL as the same kind of asset.

This plan now incorporates the additive Managed Node/Passive Endpoint product refinements and the future Capability Endpoint guidance. The future guidance changes the architecture posture, not the immediate implementation target: Phase B remains focused on Ollama Managed Nodes and Ollama-compatible Passive Endpoints.

## Goals

Phase B should add:

- persistent node control metadata
- persistent trust metadata
- separate UI flows for Managed Nodes and Passive Endpoints
- badges and warnings that match the asset's control and trust level
- routing and safety hooks that Phase C can use without a schema reset

Phase B should not add prompt or response logging, background subnet scanning by default, or hidden trust escalation.

Phase B should also not add GitHub, Xcode, Docker, CI/CD, Codex-agent, build-runner, marketplace, plugin, or arbitrary tool-execution surfaces. Those are V2 Capability Endpoint candidates after the V1 Ollama fleet control plane is functional.

## Planned Data Model

Each node-like record should carry these fields or equivalent structured metadata:

- `node_id`: stable local identifier
- `display_name`: operator-facing label
- `control_level`: `managed` or `passive`
- `trust_level`: `local`, `lan_trusted`, `lan_unverified`, or `external`
- `capability_source`: `subscriber_reported`, `marshal_observed`, or `manual`
- `endpoint_url`: Ollama-compatible URL for Passive Endpoints or subscriber-proxied Ollama URL for Managed Nodes
- `subscriber_url`: Wrangler subscriber URL for Managed Nodes
- `approval_state`: `pending`, `approved`, `rejected`, or `revoked`
- `health_source`: `subscriber_reported` or `marshal_observed`
- `model_inventory_source`: `subscriber_reported`, `marshal_observed`, or `manual`
- `benchmark_source`: `subscriber_reported`, `marshal_observed`, or `none`
- `warm_state_supported`: boolean
- `management_supported`: boolean
- `telemetry_level`: `rich_metadata` or `marshal_observed_metadata`
- `last_observed_at`: marshal observation timestamp
- `last_reported_at`: subscriber report timestamp

The implementation should add this through the existing app-state schema/versioning and migration path. It should not place enrollment secrets, API keys, raw headers, prompt bodies, response bodies, request bodies, or payloads in node records.

The current schema does not need a generic V2 `endpoint_type` migration yet. If future non-Ollama Capability Endpoints begin implementation, add that field through the normal additive state migration path and keep Ollama-specific metadata nested or clearly scoped.

## Managed Node Flow

The Nodes UI should expose an Install/enroll Wrangler subscriber flow for full-control Managed Nodes.

The flow should:

- generate or display a short-lived enrollment token or equivalent enrollment instruction
- store only token hashes, hints, expiry, and metadata in the marshal queue
- show install guidance for running a subscriber beside Ollama
- accept subscriber registration or manual subscriber URL entry
- place new subscribers in `pending` approval state
- require explicit operator approval before routing production traffic
- show hardware, Ollama version, model inventory, health, load, warm-model state, and benchmark metadata when reported
- accept metadata-only subscriber heartbeats after registration
- track `last_reported_at` freshness for heartbeat-required Managed Nodes
- badge the node as `managed`
- badge trust level and approval state separately

Managed Node telemetry can be richer than Passive Endpoint telemetry, but it must remain metadata-only by default.

## Passive Endpoint Flow

The Nodes UI should expose an Add existing Ollama endpoint flow for limited-control Passive Endpoints.

The flow should:

- request endpoint URL and optional display name
- require an explicit trust-level choice
- default LAN endpoints to `lan_unverified` unless the operator marks them trusted
- validate compatibility with safe metadata probes such as `/api/tags`
- show that hardware, warm-model state, and local load are unknown unless directly observed
- badge the endpoint as `passive`
- show clear limitations before enabling routing

Passive Endpoint records should use marshal-observed metadata only. They must not imply that Llama Wrangler can inspect or control the underlying asset.

## Routing and Safety Hooks

Phase B should store enough metadata for Phase C policies:

- prefer Managed Nodes when model availability and health are comparable
- allow Passive Endpoints for single-route requests only after explicit addition and approval
- exclude `external` trust from routing by default unless later policy explicitly allows it
- exclude heartbeat-required Managed Nodes when subscriber heartbeats are missing or stale
- de-prioritize `lan_unverified` and Passive Endpoint candidates for approved single-route requests
- exclude Passive Endpoints from consensus by default
- exclude `lan_unverified` and `external` endpoints from consensus and sensitive workloads unless policy explicitly allows them
- queue subscriber-reported benchmark work only for eligible Managed Nodes
- treat Passive Endpoint benchmark posture as marshal-observed/probe-only without local-control claims
- hide warm-model controls for Passive Endpoints
- treat Passive Endpoint health, model inventory, and benchmarks as marshal-observed and stale-prone
- record routing telemetry with control/trust metadata only

## UI Badges

The UI should show compact, consistent badges for:

- control level: Managed Node or Passive Endpoint
- trust level: Local, LAN trusted, LAN unverified, or External
- approval state: Pending, Approved, Rejected, or Revoked
- capability source: Subscriber reported, Marshal observed, or Manual

Badges should appear in Nodes first, then routing and operations surfaces when those views consume the metadata.

## Acceptance Criteria for the Next Slice

Near-term task ordering after the future Capability Endpoint intake is:

- keep the Capability Endpoint material documented as future scope
- continue V1 Phase B benchmark and scheduler work
- avoid adding non-Ollama integration UI or APIs until V1 acceptance criteria are met
- preserve generic control/trust/capability-source metadata so the later endpoint model can be added without a rewrite

Implemented in the initial schema slice:

- add state schema fields and migration coverage for control/trust metadata
- preserve existing state migrations and config-version behavior
- keep existing manually added subscribers working as Managed Node candidates or approved managed records, depending on the least disruptive migration
- add initial UI badges without claiming unavailable controls
- expose sanitized metadata through bootstrap and support bundles

Implemented in the Passive Endpoint add-flow slice:

- add a Passive Endpoint add form with safe validation and warnings, or explicitly document any deferred part in the ledger
- add API handling for endpoint URL, display name, explicit trust-level selection, and safe `/api/tags` validation
- store Passive Endpoint records with marshal-observed metadata defaults, pending approval, no warm-state support, and no model-management support
- keep Managed Node subscriber add behavior separate
- add focused tests for passive endpoint API behavior, support-bundle privacy, and Managed Node preservation
- verify live `/healthz`, bootstrap, support-bundle privacy, and browser rendering

Implemented in the approval/revocation slice:

- add approval/revocation actions and routing eligibility rules for pending versus approved nodes
- keep pending, rejected, revoked, disabled, failed, and otherwise unapproved nodes out of routing
- surface approve/revoke controls in the Nodes UI
- preserve Passive Endpoint add-flow metadata and Managed Node subscriber behavior

Implemented in the trust-level update slice:

- make trust level changes explicit from the UI
- add `POST /wrangler/nodes/:id/trust`
- warn for `lan_unverified` and `external` trust levels
- preserve approval/revocation routing eligibility and Passive Endpoint add-flow behavior

Implemented in the routing/trust policy slice:

- use control/trust metadata in routing candidate scoring and eligibility
- exclude `external` trust from routing by default
- de-prioritize Passive Endpoints and `lan_unverified` candidates for single-route requests
- require Managed Node plus `local` or `lan_trusted` trust for consensus eligibility
- include metadata-only candidate/exclusion reason codes in routing telemetry
- prevent strict/task session affinity from selecting a policy-excluded node

Implemented in the Managed Node enrollment-token slice:

- add `POST /wrangler/enrollment-tokens` for one-time short-lived token generation
- expose sanitized enrollment queue metadata through bootstrap/support bundles without token hashes
- add `POST /subscriber/enroll` for token-based subscriber registration
- create registered subscribers as pending Managed Nodes requiring approval before routing
- keep manual subscriber add available for compatibility

Implemented in the Managed Node heartbeat/freshness slice:

- add `POST /subscriber/heartbeat` for metadata-only Managed Node heartbeats
- refresh `last_reported_at`, subscriber-reported health/model/load metadata, and heartbeat state
- preserve approval state, trust level, and existing schema version 2 behavior during heartbeat updates
- exclude heartbeat-required Managed Nodes from routing when the heartbeat is missing or stale
- keep legacy/manual Managed Nodes eligible unless they explicitly opt into heartbeat-required semantics
- show heartbeat freshness in the Nodes UI

Implemented in the Managed Node heartbeat identity slice:

- derive a heartbeat shared-secret credential from the enrollment token and final node ID
- store the derived heartbeat credential only in the configured encrypted secret backend
- expose safe heartbeat auth method, token hint, and derivation metadata without returning the raw credential
- require the stored credential on heartbeats for nodes that have one
- preserve compatibility for legacy/manual Managed Nodes without a stored heartbeat credential
- show safe heartbeat auth status in the Nodes UI

Implemented in the Managed Node heartbeat credential rotation slice:

- add `POST /wrangler/nodes/:id/heartbeat-credential/rotate` for explicit admin provisioning and rotation
- return the raw generated heartbeat credential only in the immediate rotation response
- store the raw credential only in the configured secret backend
- expose only safe token hint, auth method, derivation, provisioning, and re-provisioning-required metadata in node state and UI
- reject Passive Endpoint credential provisioning
- require the new credential on subsequent heartbeats and reject old or missing credentials

Implemented in the subscriber credential install guidance slice:

- add a subscriber-side `registration.heartbeat_credential_env` config hook, with `LLAMA_WRANGLER_SUBSCRIBER_HEARTBEAT_CREDENTIAL` as the documented env var
- extend rotation responses with a `subscriber_install` guidance object containing placeholder shell export, launchd dry-run, and heartbeat verification commands
- surface a one-time Nodes UI install card after rotation with copy actions for the immediate-response credential and credential-substituted commands
- keep the guidance object placeholder-based and keep raw credentials out of app state, bootstrap, support bundles, telemetry, and node metadata
- keep full remote subscriber config mutation and installer packaging as future work

Implemented in the subscriber service-wrapper follow-through slice:

- extend `subscriber_install` with placeholder env-file template, launchd plist template, service-wrapper metadata, install commands, validation commands, and uninstall commands
- keep service-wrapper artifacts as manual operator guidance for the subscriber host with no remote mutation from the marshal
- update Nodes UI copy actions for env-file, launchd plist, install, validation, and uninstall artifacts while substituting the one-time credential only in browser memory
- keep raw heartbeat credentials out of ordinary app state, bootstrap, telemetry, support bundles, node metadata, docs examples, and service-wrapper metadata

Implemented in the routing/operations warning slice:

- expose metadata-only `routing_policy_status` through bootstrap and metrics
- explain unapproved, disabled, external, Passive Endpoint consensus, `lan_unverified`, missing-heartbeat, and stale-heartbeat routing policy states with reason codes
- render policy warnings on Dashboard and Nodes without requiring a failed inference request
- keep warnings limited to node ID, control/trust/approval/source metadata, severity, scope, code, and concise operator messages

Implemented in the manual subscriber hardening slice:

- keep `POST /wrangler/nodes/manual-add` as a compatibility fallback
- validate manual subscriber URLs with the same safe URL rules used by other endpoint add flows
- reject embedded credentials, relative URLs, and unsupported schemes
- create manually added Managed Nodes as pending approval instead of immediately approved
- show stronger UI warning copy that token-based enrollment is preferred

Implemented in the benchmark policy wiring slice:

- expose metadata-only `benchmark_policy_status` through bootstrap and metrics
- allow `POST /wrangler/nodes/:id/benchmark` only for eligible Managed Nodes
- reject Passive Endpoint benchmark actions with `passive_no_local_benchmark_control`
- render Dashboard and Nodes benchmark policy indicators without implying Passive Endpoint local-control capabilities
- keep benchmark policy telemetry and API responses limited to node/control/trust/approval/source/reason metadata

Implemented in the safe peer discovery slice:

- replace the discovery placeholder with explicit operator-initiated mDNS/Bonjour candidate discovery
- return candidate services as review-only metadata with `approval_state: not_added` and default `trust_level: lan_unverified`
- keep subnet scanning disabled and documented as requiring future separate opt-in
- do not persist, approve, or route to discovered candidates automatically
- direct operators to Managed Node enrollment or Passive Endpoint add flows for adoption

Implemented in the benchmark execution/result ingestion slice:

- add authenticated `POST /subscriber/benchmarks` ingestion for Managed Node subscriber-reported benchmark metrics
- keep accepted benchmark result fields limited to model/status/timing/token-count/rate/error-code metadata
- keep Managed Node benchmark action as queued/requested metadata until full subscriber job orchestration is added
- add Passive Endpoint `benchmark-probe` action for marshal-observed `/api/tags` availability/latency metadata
- continue rejecting Passive Endpoints from local benchmark control through `POST /wrangler/nodes/:id/benchmark`
- show benchmark result/probe metadata in Nodes UI without exposing prompts, responses, request bodies, raw headers, endpoint credentials, or secrets

Implemented in the Managed Node benchmark job orchestration slice:

- convert `POST /wrangler/nodes/:id/benchmark` into a metadata-only benchmark job creation action for eligible Managed Nodes
- add authenticated subscriber job claim and status endpoints for queued/running/completed/failed/cancelled job lifecycle metadata
- complete matching benchmark jobs when `POST /subscriber/benchmarks` reports a result for the benchmark ID
- show benchmark job state in the Nodes UI alongside benchmark result summaries
- keep Passive Endpoints limited to marshal-observed `/api/tags` probes and no local-control benchmark jobs

Implemented in the benchmark-derived routing placement slice:

- rank otherwise eligible routing candidates with fresh subscriber-reported Managed Node benchmark summaries
- require approved, enabled, trusted Managed Nodes before benchmark summaries can influence placement
- ignore Passive Endpoint marshal-observed probes, stale summaries, incomplete summaries, untrusted summaries, and summaries without token-rate metadata for placement
- surface placement eligibility and reason codes in metadata-only `benchmark_policy_status` and the Dashboard/Nodes UI
- keep placement metadata limited to node/control/trust/approval/source/freshness/reason posture and avoid prompts, responses, request bodies, raw headers, endpoint credentials, or secrets

Implemented in the durable benchmark scheduler slice:

- add bounded retry/timeout metadata to Managed Node benchmark jobs with policy `bounded_retry_timeout_v1`
- persist attempt count, max attempts, lease timeout, retry delay, next attempt, timeout deadline, scheduler state, and reason with the job metadata
- reconcile timed-out running jobs, due retry-wait jobs, and exhausted max-attempt jobs without storing payloads
- reconcile a Managed Node's jobs before subscriber claim and expose an admin reconcile endpoint for operator-driven recovery
- expose metadata-only `benchmark_scheduler` through bootstrap, metrics, and Dashboard UI
- keep Passive Endpoints limited to marshal-observed probes and outside local-control benchmark scheduler jobs
- keep benchmark job metadata free of prompts, responses, raw request bodies, raw headers, endpoint credentials, enrollment tokens, heartbeat credentials, token hashes, admin tokens, client API keys, HEC tokens, and payloads

Implemented in the prompt workload benchmark suite definition slice:

- add safe benchmark workload suite definitions for built-in synthetic smoke/code suites and operator-provided local fixture manifests
- expose suite definitions through `GET /wrangler/benchmarks/workload-suites` and bootstrap without prompt or response bodies
- allow Managed Node benchmark job creation to select a suite by ID and, for local fixtures, a local fixture manifest ID or basename hint
- carry workload suite metadata on Managed Node benchmark jobs and subscriber claims as suite/task/result-metric metadata only
- keep fixture contents and full local paths out of persisted app state, bootstrap, metrics, telemetry, support bundles, and scheduler metadata
- keep Passive Endpoints probe-only and outside prompt workload benchmark execution

Implemented in the configurable benchmark scheduler policy slice:

- add bounded config-backed scheduler policy controls for max attempts, lease timeout seconds, and retry delay seconds
- expose scheduler policy config and numeric limits through `benchmark_scheduler` status and `GET /wrangler/benchmarks/scheduler/policy`
- add `PUT /wrangler/benchmarks/scheduler/policy` with normalization for invalid, missing, too-small, too-large, or unknown values
- apply configured values to new Managed Node benchmark jobs, subscriber claims, retry waits, and scheduler default handling
- preserve existing job-local policy metadata once a job is created
- render Dashboard controls for the scheduler policy
- keep Passive Endpoints probe-only and keep scheduler policy telemetry/API responses metadata-only

Implemented in the optional background scheduler tick slice:

- add disabled-by-default `background_enabled` and bounded `tick_interval_seconds` controls under the existing benchmark scheduler policy config
- reuse existing benchmark job reconciliation logic for background ticks instead of creating a parallel scheduler path
- expose background enablement, interval, next tick, last tick, last reason, and last-run summary counts through `benchmark_scheduler`
- render Dashboard controls and status for background reconciliation
- emit metadata-only `benchmark_scheduler_background_tick` telemetry
- keep background ticks limited to existing Managed Node benchmark job metadata; they do not create jobs, run prompts, inspect Passive Endpoints, mutate subscribers, or store payloads

Implemented in the subscriber benchmark runner guidance slice:

- add `GET /wrangler/benchmarks/runner/guidance` with placeholder-only subscriber runner guidance and packaging-hook metadata
- expose `benchmark_runner` through bootstrap so the Dashboard can show runner status and boundaries
- clearly mark runner status as guidance-only/not implemented until a real subscriber runner exists
- document claim/status/result endpoint flow for future subscriber-side runner packaging
- keep built-in synthetic prompt text and operator local fixture contents on the subscriber side only
- keep Passive Endpoints outside subscriber runner execution
- keep guidance, UI, telemetry, and support posture free of prompt text, response text, fixture contents, full fixture paths, raw credentials, and payloads

Implemented in the opt-in subscriber benchmark runner dry-run slice:

- add explicit `capabilities.benchmark_runner` config for subscriber-local runner enablement
- keep the runner disabled by default with mode `dry_run_v1`, result body policy `metrics_only`, bounded poll interval, and bounded max jobs per tick
- start a subscriber-mode background loop only when explicitly enabled
- claim Managed Node benchmark jobs through `/subscriber/benchmarks/claim` using the existing heartbeat credential semantics
- send status updates through `/subscriber/benchmarks/status`
- report deterministic dry-run metric summaries through `/subscriber/benchmarks` without loading prompt text, response text, fixture contents, full fixture paths, raw headers, or payloads

Implemented in the subscriber-side synthetic prompt execution slice:

- add opt-in runner mode `synthetic_builtin_v1` while preserving `dry_run_v1` as the default
- execute built-in synthetic suite/task IDs only inside subscriber runtime against the subscriber-local Ollama `/api/generate` endpoint
- discard Ollama response bodies locally and report bounded timing/token metric summaries only
- persist/report safe runner mode metadata so dry-run versus synthetic execution is auditable without storing prompts or responses
- keep operator local fixture manifests execution-disabled until a dedicated safe manifest parser/storage boundary exists
- keep Passive Endpoints probe-only and outside subscriber runner execution
- keep prompt text, response text, fixture contents, full fixture paths, raw headers, raw request bodies, endpoint credentials, enrollment tokens, heartbeat credentials, token hashes, admin tokens, client API keys, HEC tokens, and payloads out of marshal state, API responses, telemetry, and support bundles
- keep Passive Endpoints outside runner execution; Passive Endpoints remain marshal-observed/probe-only
- expose runner status/config through guidance, bootstrap, metrics, and the Dashboard runner card
- preserve runtime heartbeat credentials in process memory only; ordinary app state, UI/API responses, telemetry, and support bundles must not include raw credentials

Implemented in the model lifecycle and warm-state V1 slice:

- add `GET /wrangler/models/lifecycle` and `model_lifecycle` bootstrap/metrics status
- derive model lifecycle summaries from existing node model metadata without a schema bump
- accept sanitized Managed Node heartbeat model lifecycle states, keep-warm flags, token-rate summaries, and load-time summaries
- stamp node observed metadata with lifecycle source/mode, model count, warm count, keep-warm count, and lifecycle summary
- render Dashboard and Nodes UI model lifecycle/warm-state surfaces
- keep Passive Endpoints inventory-only from marshal-observed `/api/tags`; do not imply warm-state, load, eviction, keep-warm, or model-management control
- redact suspicious model names and keep prompts, responses, raw headers, credentials, tokens, fixture contents, full fixture paths, and payloads out of lifecycle surfaces

Implemented in the keep-warm model lifecycle action slice:

- add metadata-only `POST /wrangler/nodes/:id/model-actions/keep-warm` for eligible Managed Nodes
- require approved, enabled, healthy Managed Nodes with subscriber-reported warm-state and model-management support before queueing actions
- add authenticated subscriber action endpoints:
  - `POST /subscriber/model-actions/claim`
  - `POST /subscriber/model-actions/status`
- keep actions limited to safe metadata: action ID, action type, model name, desired keep-warm state, status, timestamps, policy, endpoint names, and safe error code
- update model lifecycle status summaries with action counts and pending action counts
- render Nodes UI keep-warm controls only for eligible Managed Nodes; Passive Endpoints remain inventory-only with no lifecycle action controls
- keep prompts, responses, raw headers, raw request bodies, credentials, tokens, fixture contents, full fixture paths, and payloads out of action state, API responses, telemetry, and support bundles

Implemented in the keep-warm action policy visibility hardening slice:

- add metadata-only `GET /wrangler/models/lifecycle/action-policies`
- include `model_lifecycle_actions` in bootstrap and metrics with eligible node counts, blocked node counts, supported action IDs, pending action counts, and safe reason codes
- render Dashboard and Nodes UI action policy status so operators can see why Managed Nodes are action-ready or blocked
- keep the policy surface read-only and aligned with the existing queue eligibility gates; Passive Endpoints remain inventory-only and never gain lifecycle action controls

Implemented in the model lifecycle action history/filter slice:

- add metadata-only `GET /wrangler/models/lifecycle/action-history` with bounded `status`, `node_id`, and `limit` filters
- derive newest-first history from existing per-node action records without a second persisted log or app-state schema change
- include queue, claim, update, completion, failure, and cancellation timestamps plus normalized safe error codes only
- include the default recent history in bootstrap and metrics and render Dashboard status/node filters with compact action rows
- exclude Passive Endpoints and normalize unknown action types, policies, statuses, and unsafe error-code strings
- retain at most eight actions per Managed Node and return at most 50 filtered rows

Implemented in the V1 non-streaming consensus foundation slice:

- apply default and configured minimum/maximum participants after routing eligibility and cap fan-out at eight
- deterministically order equal-score participants and preserve benchmark-derived placement ranking when eligible
- fan out concurrently to approved, fresh, trusted, model-eligible Managed Nodes only
- exclude Passive Endpoints, unverified/external trust, pending/revoked, disabled/unhealthy, stale/missing-heartbeat, and model-ineligible nodes
- buffer responses in request memory only with an 8 MiB per-participant cap and bounded timeout/cancellation
- compare normalized text and structurally equivalent JSON through a pluggable local engine with regex validator and evaluator hooks
- return the deterministic majority or earliest ranked successful candidate while preserving OpenAI/Ollama winner shape
- emit metadata-only consensus telemetry and Dashboard operations summaries
- reject streaming consensus before fan-out and keep Frontier escalation disabled

Implemented in the V1 consensus partial-failure hardening slice:

- classify participant failures as `missing_proxy_url`, `connection_error`, `upstream_4xx`, `upstream_5xx`, `body_read_failure`, `response_size_limit`, `timeout`, or `cancellation`
- project failure metadata as node ID, fixed reason code, optional numeric upstream status, and duration only
- preserve deterministic aggregation when the required-success bound is met despite partial participant failures
- return OpenAI/Ollama compatibility-shaped errors when required successes are unmet, including normalized all-upstream-4xx outcomes and HTTP 504 for timeout-driven quorum failure
- aggregate fixed reason-code counts in consensus operation stats and the Dashboard without exposing arbitrary error text, endpoint URLs, headers, prompts, responses, extracted content, credentials, or payloads
- keep streaming consensus explicitly unsupported before fan-out

Implemented in the V1 real-client consensus compatibility slice:

- exercise OpenAI and Ollama consensus through real marshal and participant HTTP listeners with `net/http` clients
- verify partial-success quorum and deterministic routing-ranked winner body/status/content type for both compatibility families
- verify normalized all-upstream-4xx error shapes without relaying upstream error bodies
- verify immediate HTTP 502 and timeout-driven HTTP 504 insufficient-success errors for both compatibility families
- prove streaming consensus rejection occurs before participant fan-out for both compatibility families
- prove prompt, response, and upstream error markers remain outside audit state, bootstrap, metrics, and support bundles
- record V1 streaming consensus as deliberately unsupported until a separate bounded aggregation/backpressure/cancellation/response-commit protocol is designed and tested

Implemented in the V1 Splunk operations visibility slice:

- add a dedicated metadata-only Operations dashboard while preserving the Overview dashboard
- add overview signals and operations panels for consensus failures, queue scheduling, streaming outcomes, benchmark scheduler/runner history, routing exclusions, and model lifecycle action history
- add packaged navigation, reusable macros/eventtypes, operational props categories, and disabled-by-default saved reports
- derive routing policy warnings from existing safe `routing_decision.excluded_nodes` metadata
- constrain consensus failure panels to the fixed eight-reason vocabulary
- validate XML, knowledge-object coverage, schema reason parity, default-disabled reports, and absence of payload-bearing field references

The next implementation slice should:

- add a consolidated V1 acceptance/security matrix spanning service lifecycle, setup/auth, enrollment, Managed/Passive policy, routing/consensus, benchmark/model lifecycle flows, Splunk assets, and support-bundle privacy
- continue packaging/install hardening, documentation/release polish, and git stabilization
