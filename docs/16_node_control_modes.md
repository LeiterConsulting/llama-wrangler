# Node Control Modes

Llama Wrangler supports two node-control concepts in planning and future UI/API work: Managed Node and Passive Endpoint.

These remain the V1 control modes. Broader Capability Endpoint support is a future V2 direction and should not add non-Ollama integrations, placeholder navigation, or arbitrary tool execution to the current MVP. See `docs/19_capability_endpoint_future_plan.md`.

## Managed Node

A Managed Node has the Llama Wrangler subscriber installed on the same asset as Ollama.

Managed Nodes can report:

- hardware and OS details
- Ollama version and local model inventory
- model state, including installed, loading, warm, busy, unloaded, evicted, or failed
- warm-model and keep-warm state
- current load, queue depth, and health
- registration heartbeat and freshness timestamps
- benchmark results and observed tokens/sec
- richer metadata-only telemetry

Managed Nodes are the preferred participants for:

- consensus
- benchmark-driven routing
- warm-model placement
- controlled failover
- detailed operational troubleshooting
- future safe model-management actions

## Passive Endpoint

A Passive Endpoint is an existing Ollama-compatible endpoint URL known only to the marshal.

Passive Endpoints can support:

- request routing
- marshal-observed latency, availability, and error-rate telemetry
- model inventory if the endpoint exposes compatible `/api/tags`
- limited health checks

Passive Endpoints cannot be assumed to support:

- hardware inspection
- local load inspection beyond marshal-observed request behavior
- warm-model state
- benchmark execution unless explicitly implemented as marshal-observed probes
- model pull/unload/keep-warm control
- subscriber-side telemetry

## Control Level

Planned control levels:

- `managed`: Llama Wrangler subscriber installed and approved
- `passive`: existing Ollama-compatible endpoint URL only

The UI should badge every node or endpoint with its control level. Routing and operations views should avoid implying that Passive Endpoints are fully inspectable or controllable.

## Trust Level

Planned trust levels:

- `local`: loopback or same-host endpoint
- `lan_trusted`: user-approved LAN endpoint
- `lan_unverified`: LAN endpoint added but not fully trusted
- `external`: non-local endpoint, disabled by default unless explicitly enabled by policy

Trust level affects:

- whether the endpoint can participate in consensus
- whether it can receive sensitive workloads
- whether it is eligible for Frontier Delta comparisons
- whether prompt/response payload logging remains forbidden by default
- how prominently the UI warns the operator

The Nodes UI supports explicit trust-level updates for existing Managed Nodes and Passive Endpoints. Updates are metadata-only. `lan_unverified` and `external` must be visibly warned because later routing, consensus, benchmark, and safety policy slices will use these values.

## Routing and Consensus Policy

Default policy should be conservative:

- Prefer Managed Nodes when equivalent model and health signals exist.
- Allow Passive Endpoints for basic single-route requests after explicit user addition.
- Require explicit approval before any Managed Node or Passive Endpoint participates in routing.
- Exclude pending, rejected, revoked, disabled, failed, or otherwise unapproved nodes from routing.
- Exclude `external` trust from routing by default unless a later explicit policy allows it.
- De-prioritize `lan_unverified` and Passive Endpoint candidates for approved single-route requests.
- Exclude Passive Endpoints from consensus by default.
- Exclude `lan_unverified` and `external` trust levels from consensus by default.
- Do not use Passive Endpoints for benchmark-derived placement unless benchmarks are clearly marshal-observed.
- Do not show warm-model controls for Passive Endpoints.
- Treat Passive Endpoint health and model state as stale-prone and marshal-observed.
- Treat heartbeat-required Managed Nodes with missing or stale subscriber reports as ineligible until a fresh heartbeat arrives.
- Emit routing policy metadata with node ID, control level, trust level, capability source, and reason codes only.

## Benchmark Policy

Benchmark controls are intentionally conservative:

- Managed Nodes can queue subscriber-reported benchmark work only after they are approved, enabled, and healthy.
- Managed Node benchmark jobs can reference benchmark workload suite definitions by ID, including built-in synthetic suites and operator-provided local fixture manifests.
- Workload suite metadata may include suite IDs, task IDs, result metric names, and local fixture manifest IDs or basename hints only.
- Prompt text, response text, fixture contents, raw request bodies, raw headers, endpoint credentials, API keys, tokens, and payloads must never be stored in benchmark jobs, node observed metadata, telemetry, bootstrap, metrics, or support bundles.
- Managed Nodes can report benchmark result metadata through the authenticated subscriber benchmark endpoint.
- Managed Nodes can claim marshal-created benchmark jobs through authenticated subscriber job endpoints and update job status/result metadata without carrying prompts, responses, raw headers, request bodies, or credentials.
- Managed Node benchmark scheduler policy supports bounded metadata controls for max attempts, running-job lease timeout, and retry delay. Invalid or out-of-range values normalize to safe bounds, and jobs persist only numeric policy metadata.
- Optional background scheduler reconciliation is disabled by default. When explicitly enabled, it only reconciles existing Managed Node benchmark job metadata at a bounded interval and emits metadata-only tick counts.
- Subscriber benchmark runner support is available as an explicit opt-in dry-run loop for subscriber mode plus packaging-hook metadata. It may claim Managed Node jobs, post status transitions, and report deterministic metric summaries from suite/task metadata only. It may expose suite IDs, task IDs, endpoints, env var names, placeholder commands, bounded runner settings, and metric field names, but no prompt text, response text, fixture contents, full fixture paths, raw credentials, raw request bodies, or payloads.
- Passive Endpoints are probe-only for benchmark planning. The marshal may record `/api/tags` latency/availability metadata, but it must not imply hardware inspection, local benchmark execution, warm-state inspection, load inspection, prompt workload benchmarking, or model-management control.
- The UI must hide local benchmark controls for Passive Endpoints and show their benchmark source as `none` or marshal-observed probe metadata only.
- Benchmark policy status must be metadata-only and must not include secrets, raw headers, prompts, responses, request bodies, endpoint credentials, API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, or payloads.

## Model Lifecycle And Warm State

- Managed Nodes can report model lifecycle and warm-state metadata through subscriber heartbeat.
- Managed model metadata may include safe model names, lifecycle state, keep-warm flag, token-rate summary, and load-time summary.
- Managed model lifecycle status is exposed through `model_lifecycle` in bootstrap/metrics and `GET /wrangler/models/lifecycle`.
- Managed Nodes that are approved, enabled, healthy, warm-state supported, and model-management supported can receive metadata-only keep-warm actions queued by the marshal and claimed by the subscriber through authenticated model-action endpoints.
- Keep-warm action metadata may include action ID, action type, model name, desired keep-warm state, status, timestamps, policy, endpoint names, and safe error code only.
- Model lifecycle action policy status is exposed as `model_lifecycle_actions` in bootstrap/metrics and through `GET /wrangler/models/lifecycle/action-policies` so operators can see eligible/blocked nodes and safe reason codes before queueing actions.
- Passive Endpoints are inventory-only. Their model list is marshal-observed from `/api/tags`, and warm state, local load, eviction state, keep-warm configuration, and model-management control must remain unavailable.
- Model lifecycle surfaces and actions must stay metadata-only and must not expose prompts, responses, raw headers, raw request bodies, endpoint credentials, API keys, HEC tokens, enrollment tokens, heartbeat credentials, token hashes, fixture contents, full fixture paths, or payloads.

## UI Flows

The Nodes UI should support two separate flows:

1. Install/enroll Wrangler subscriber
   - For full-control Managed Nodes.
   - Shows enrollment token, install guidance, subscriber status, approval, hardware, model state, warm state, load, health, and benchmarks.
   - Enrollment tokens are shown once, expire quickly, and are stored only as hashes plus safe hints.
   - Subscriber registration creates a pending Managed Node until the operator approves it.
   - Registered subscribers post metadata-only heartbeats after enrollment.
   - Subscriber enrollment derives a heartbeat shared-secret credential from the one-time enrollment token and final node ID.
   - The marshal stores that credential in the encrypted secret backend and exposes only safe method/hint metadata.
   - Heartbeats refresh subscriber-reported health, model inventory, load, and freshness metadata without carrying secrets or request/response payloads.
   - Enrolled subscribers authenticate heartbeats with `X-Llama-Wrangler-Subscriber-Token` or `Authorization: Bearer`.
   - Legacy/manual Managed Nodes can receive an explicit admin-rotated heartbeat credential; the raw credential is returned once, stored only in the secret backend, and then required on future heartbeats.
   - Rotation responses include subscriber-side install guidance with the `LLAMA_WRANGLER_SUBSCRIBER_HEARTBEAT_CREDENTIAL` env var, `registration.heartbeat_credential_env` config key, placeholder env-file template, launchd dry-run command, launchd plist template, install/validation/uninstall commands, and heartbeat verification command.
   - The install guidance must use placeholders and safe hints; only the existing immediate rotation response may contain the raw credential.
   - The marshal does not remotely mutate subscriber machines, install service wrappers, write subscriber configs, or start subscriber services.
   - Missing or stale heartbeats remove heartbeat-required Managed Nodes from routing eligibility until a fresh heartbeat arrives.
   - Manual subscriber add remains a compatibility fallback only and creates pending Managed Nodes that still require explicit approval.

2. Add existing Ollama endpoint
   - For limited-control Passive Endpoints.
   - Asks for endpoint URL, optional display name, and explicit trust level.
   - Validates compatibility with a safe `/api/tags` probe.
   - Shows explicit limitations and safety warnings.
   - Adds the endpoint in pending approval state until the operator approves it.

Both flows must keep secrets, raw headers, prompt bodies, response bodies, and payloads out of ordinary app state, support bundles, and telemetry by default.

## Phase Placement

This is a Phase B/C scope expansion:

- Phase B should add the two UI enrollment/add flows and persist control/trust metadata. The initial Passive Endpoint add flow validates `/api/tags` and stores marshal-observed metadata.
- Phase B should enforce approval/revocation state for routing eligibility before deeper trust policy work.
- Phase B should expose trust-level update controls with warnings before deeper trust-aware routing work.
- Phase B should add token-based Managed Node enrollment and a sanitized marshal approval queue.
- Phase B should add Managed Node heartbeat and freshness handling for registered subscribers.
- Phase B should keep manual subscriber add behind approval semantics and warn that token-based enrollment is preferred.
- Phase B should harden Managed Node heartbeat identity with a shared subscriber credential while keeping raw credentials out of state, telemetry, support bundles, and UI metadata.
- Phase B should add explicit heartbeat credential rotation/re-provisioning for legacy/manual Managed Nodes.
- Phase B should add operator-initiated mDNS discovery as review-only candidate metadata, with subnet scanning disabled unless a future explicit opt-in workflow exists.
- Phase C should use control/trust level in routing, consensus, benchmark eligibility, warm-model controls, and safety policy.

Future Capability Endpoint work should begin only after the V1 Ollama fleet control plane is functional. Until then, keep the model generic enough for later endpoint types but continue implementing concrete Managed Node and Passive Endpoint behavior first.

It does not replace the current Phase A hardening work.
