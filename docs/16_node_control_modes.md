# Node Control Modes

Llama Wrangler supports two node-control concepts in planning and future UI/API work: Managed Node and Passive Endpoint.

## Managed Node

A Managed Node has the Llama Wrangler subscriber installed on the same asset as Ollama.

Managed Nodes can report:

- hardware and OS details
- Ollama version and local model inventory
- model state, including installed, loading, warm, busy, unloaded, evicted, or failed
- warm-model and keep-warm state
- current load, queue depth, and health
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
- benchmark execution unless explicitly triggered from marshal-observed probes
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

## Routing and Consensus Policy

Default policy should be conservative:

- Prefer Managed Nodes when equivalent model and health signals exist.
- Allow Passive Endpoints for basic single-route requests after explicit user addition.
- Exclude Passive Endpoints from consensus by default unless the user marks them trusted.
- Do not use Passive Endpoints for benchmark-derived placement unless benchmarks are clearly marshal-observed.
- Do not show warm-model controls for Passive Endpoints.
- Treat Passive Endpoint health and model state as stale-prone and marshal-observed.

## UI Flows

The Nodes UI should support two separate flows:

1. Install/enroll Wrangler subscriber
   - For full-control Managed Nodes.
   - Shows enrollment token, install guidance, subscriber status, approval, hardware, model state, warm state, load, health, and benchmarks.

2. Add existing Ollama endpoint
   - For limited-control Passive Endpoints.
   - Asks for endpoint URL, optional display name, trust level, and intended use.
   - Shows explicit limitations and safety warnings.

Both flows must keep secrets, raw headers, prompt bodies, response bodies, and payloads out of ordinary app state, support bundles, and telemetry by default.

## Phase Placement

This is a Phase B/C scope expansion:

- Phase B should add the two UI enrollment/add flows and persist control/trust metadata.
- Phase C should use control/trust level in routing, consensus, benchmark eligibility, warm-model controls, and safety policy.

It does not replace the current Phase A hardening work.
