# Phase B Managed Node and Passive Endpoint Plan

Phase B begins with a clear node-control model instead of treating every reachable Ollama URL as the same kind of asset.

## Goals

Phase B should add:

- persistent node control metadata
- persistent trust metadata
- separate UI flows for Managed Nodes and Passive Endpoints
- badges and warnings that match the asset's control and trust level
- routing and safety hooks that Phase C can use without a schema reset

Phase B should not add prompt or response logging, background subnet scanning by default, or hidden trust escalation.

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

## Managed Node Flow

The Nodes UI should expose an Install/enroll Wrangler subscriber flow for full-control Managed Nodes.

The flow should:

- generate or display a short-lived enrollment token or equivalent enrollment instruction
- show install guidance for running a subscriber beside Ollama
- accept subscriber registration or manual subscriber URL entry
- place new subscribers in `pending` approval state
- require explicit operator approval before routing production traffic
- show hardware, Ollama version, model inventory, health, load, warm-model state, and benchmark metadata when reported
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
- allow Passive Endpoints for single-route requests only after explicit addition
- exclude Passive Endpoints from consensus by default
- exclude `lan_unverified` and `external` endpoints from sensitive workloads unless policy explicitly allows them
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

Implemented in the initial schema slice:

- add state schema fields and migration coverage for control/trust metadata
- preserve existing state migrations and config-version behavior
- keep existing manually added subscribers working as Managed Node candidates or approved managed records, depending on the least disruptive migration
- add initial UI badges without claiming unavailable controls
- expose sanitized metadata through bootstrap and support bundles

The next implementation slice should:

- add a Passive Endpoint add form with safe validation and warnings, or explicitly document any deferred part in the ledger
- add focused tests for migration, API shape, and UI bootstrap metadata
- verify live `/healthz`, bootstrap, support-bundle privacy, and browser rendering
