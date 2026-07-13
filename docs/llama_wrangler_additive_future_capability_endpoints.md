# Additive Future-Facing Requirement: Capability Endpoint Expansion

## Purpose

This additive document captures a future-facing architectural direction for Llama Wrangler.

The intent is **not** to expand the current MVP implementation scope. The current implementation priority remains:

1. Make Llama Wrangler functional as currently designed.
2. Complete the friendly local-first platform experience.
3. Support managed/active nodes and passive Ollama endpoints.
4. Finish discovery, enrollment, routing, model management, observability, and Splunk integration.
5. Only after that foundation is working, begin expanding toward broader capability endpoints.

This document exists so current API, data model, routing, policy, telemetry, and UI decisions do not accidentally paint the project into an Ollama-only corner.

## Executive Summary

Llama Wrangler should remain **Ollama-first for the MVP**, but it should be designed so Ollama nodes are one type of a broader future concept: a **Capability Endpoint**.

A capability endpoint is any local, network, or cloud-accessible system that can perform useful work for an AI-assisted workflow.

Examples may eventually include:

- Ollama
- LM Studio
- MLX / MLStudio
- vLLM
- OpenAI-compatible APIs
- Frontier model providers
- Codex-style coding agents
- Cline / Continue / IDE agents
- GitHub
- local build runners
- Xcode services
- Docker
- CI/CD systems
- Splunk
- TestFlight workflows
- other local or remote automation surfaces

For now, this should influence architecture only. It should not distract from shipping the Ollama fleet manager.

## Current Priority Remains Unchanged

The near-term goal is still to complete Llama Wrangler as a friendly, installable Ollama control plane with:

- embedded setup and operations UI
- marshal, subscriber, and standalone service modes
- managed/active nodes
- passive Ollama endpoint support
- OpenAI-compatible and Ollama-compatible endpoints
- stateful sessions
- safe routing
- node enrollment
- node discovery
- model aliases
- model inventory
- model lifecycle controls
- queueing
- cancellation and streaming safety
- local-only safe defaults
- metadata-only observability by default
- optional Splunk HEC telemetry
- optional Frontier Delta only after explicit configuration and redaction controls

Do **not** defer these core items in favor of future capability endpoint work.

## Non-Goal for Current MVP

Do not implement broad capability endpoints in the current MVP.

Specifically, do not implement the following yet:

- GitHub actions or repo mutation workflows
- Xcode build/test runners
- TestFlight automation
- Docker orchestration
- CI/CD control
- server-side arbitrary tool execution
- general plugin execution
- cloud agent orchestration
- non-Ollama local model runtimes beyond passive OpenAI/Ollama-compatible endpoint abstractions
- full capability marketplace or plugin system

These are future possibilities, not immediate deliverables.

## Architectural Guidance for Current Work

While implementing the current Llama Wrangler platform, avoid hard-coding assumptions that make future capability expansion difficult.

### Preferred Mental Model

Use this conceptual hierarchy:

```text
Llama Wrangler
  Fleet / Registry
    Capability Endpoint
      Ollama Managed Node
      Ollama Passive Endpoint
      Future Capability Types
```

For MVP, the only fully implemented endpoint categories should be:

```text
Managed Ollama Node
Passive Ollama Endpoint
Local Marshal / Standalone Ollama Runtime
```

But the internal naming, schema boundaries, and policy engine should be capable of evolving.

## Capability Endpoint Concept

A future Capability Endpoint may have:

```yaml
id: string
display_name: string
endpoint_type: ollama | openai_compatible | lm_studio | mlx | vllm | github | xcode | docker | splunk | codex_agent | custom
mode: managed | passive | external
trust_level: local | managed | trusted_lan | vpn | external | untrusted
control_level: full | limited | inference_only | read_only | approval_required
capabilities:
  - inference.chat
  - inference.embeddings
  - model.inventory
  - model.lifecycle
  - code.review
  - code.edit
  - build.run
  - test.run
  - deploy.prepare
  - splunk.search
  - telemetry.emit
policy:
  local_only_allowed: true
  frontier_allowed: false
  requires_approval: false
telemetry_profile:
  metadata_only: true
  payload_logging_allowed: false
```

This is a future model. The current implementation does not need to support every field, but current schema choices should not prevent these concepts later.

## Managed Node vs Passive Endpoint Still Comes First

The immediate next product distinction remains:

### Managed Node

Wrangler subscriber is installed on the asset with Ollama.

Provides:

- full local hardware detection
- local Ollama detection
- model inventory
- model lifecycle management
- keep-warm controls
- benchmark execution
- subscriber heartbeat
- richer telemetry
- stronger trust model
- stronger routing confidence

### Passive Endpoint

Wrangler only knows about an existing Ollama-compatible endpoint.

Provides:

- endpoint health check
- model inventory where available
- inference proxying
- marshal-observed telemetry
- limited routing participation

Limitations:

- no local hardware detection
- no full model lifecycle control
- no trusted subscriber heartbeat
- limited cancellation/control
- limited load awareness
- weaker benchmark/control posture

This active/passive model should be completed before broad capability endpoint expansion begins.

## API Design Guidance

Current APIs may still use practical Ollama/node language where necessary, but new management APIs should avoid unnecessarily narrow design.

Prefer names like:

```text
/wrangler/nodes
/wrangler/endpoints
/wrangler/capabilities
/wrangler/routing/policies
/wrangler/sessions
/wrangler/telemetry/status
```

Avoid future-hostile names like:

```text
/wrangler/ollama-only-workers
/wrangler/model-hosts-only
```

When possible, model request/response fields should separate:

```text
node identity
endpoint type
runtime type
capabilities
models
control level
trust level
policy
telemetry
```

This allows Ollama-specific details to remain nested under an Ollama-specific block while preserving a generic outer structure.

Example future-compatible shape:

```json
{
  "id": "m1-max",
  "display_name": "M1 Max MacBook Pro",
  "endpoint_type": "ollama",
  "mode": "managed",
  "trust_level": "managed",
  "control_level": "full",
  "status": "healthy",
  "capabilities": ["inference.chat", "model.inventory", "model.lifecycle"],
  "ollama": {
    "url": "http://localhost:11434",
    "version": "0.31.x",
    "models": ["gemma4:12b-mlx", "qwen3-coder:30b"]
  }
}
```

## Routing Design Guidance

The router should eventually route by capability, not only by model name.

Current routing can remain model-centric:

```text
model alias → eligible Ollama node → selected model
```

But future routing should be able to support:

```text
task requirement → required capability → eligible endpoint → policy check → dispatch
```

Examples:

```text
Need chat inference           → inference.chat
Need embeddings               → inference.embeddings
Need repo review              → code.review
Need tests run                → test.run
Need Splunk search            → splunk.search
Need app build                → build.run
Need frontier review          → inference.chat + external + frontier policy
```

For MVP, only inference-related capabilities need to function. The routing model should simply not block future capability-based routing.

## Policy Design Guidance

Llama Wrangler’s policy system should eventually apply to any capability endpoint.

Policy dimensions should remain generic where possible:

- local-only
- external allowed
- approval required
- payload logging allowed
- prompt logging allowed
- source code allowed externally
- customer data allowed externally
- max cost
- max latency
- trust-level requirements
- control-level requirements
- endpoint allowlist/denylist

For now, these policies apply primarily to local Ollama nodes, passive endpoints, and future Frontier Delta calls.

## Telemetry and Splunk Guidance

Splunk telemetry should be designed to handle future endpoint types without requiring a complete sourcetype redesign.

Current Ollama-related events should remain supported, but core event fields should include:

```text
endpoint_id
endpoint_type
endpoint_mode
trust_level
control_level
capability
task_type
client_id
session_id
request_id
execution_mode
routing_policy
status
latency_ms
```

Ollama-specific fields should remain available but not be required for every future event.

Example:

```json
{
  "event_type": "routing_decision",
  "request_id": "req_123",
  "endpoint_id": "rtx4090",
  "endpoint_type": "ollama",
  "endpoint_mode": "managed",
  "capability": "inference.chat",
  "task_type": "code",
  "model_alias": "local-code",
  "resolved_model": "qwen3-coder:30b",
  "status": "selected"
}
```

Future endpoint types should be able to emit similar events without pretending to be Ollama nodes.

## UI Guidance

The UI should remain focused on the current product:

- Dashboard
- Nodes
- Models
- Routing
- Splunk
- IDE Setup
- Audit
- Settings

For now, it is acceptable to use “Nodes” as the visible product term.

However, the UI should not make it impossible to later introduce broader sections such as:

- Capability Endpoints
- Integrations
- Tools
- Build Runners
- Providers
- Policies
- Workflows

### Do Not Add Future UI Yet

Do not add empty navigation items for GitHub, Xcode, Docker, CI/CD, or other future systems unless implementation is actually beginning.

The only acceptable near-term UI hints are subtle architectural language in docs or internal schemas.

## Documentation Guidance

Add this future-facing concept to architecture documentation, but clearly label it as future scope.

Recommended doc section title:

```text
Future Direction: Capability Endpoints
```

Include clear language:

```text
This section describes a future expansion path. It is not part of the current MVP implementation target. Current work remains focused on making Llama Wrangler functional as a friendly Ollama fleet control plane with managed and passive endpoint support.
```

## Development Order

The desired implementation order is:

1. Complete current Llama Wrangler MVP foundation.
2. Complete managed/active node support.
3. Complete passive Ollama endpoint support.
4. Complete node enrollment and discovery.
5. Complete model lifecycle and model state tracking.
6. Complete benchmark and capability scoring for Ollama nodes.
7. Complete consensus mode.
8. Complete Splunk app expansion.
9. Complete Frontier Delta with safe policy/redaction.
10. Package/install/service lifecycle.
11. Only then begin implementing broader capability endpoint types.

## Acceptance Criteria for This Additive Requirement

This additive requirement is satisfied when:

- the current implementation continues focusing on Ollama fleet control
- managed/active nodes remain the next priority
- passive Ollama endpoints are supported as limited-control endpoints
- API/data model choices do not unnecessarily block future endpoint types
- telemetry includes generic endpoint/capability fields where practical
- docs explicitly label broader capability endpoints as future-facing
- no unrelated capability systems are implemented prematurely
- no MVP scope is derailed by future GitHub/Xcode/Docker/Codex ambitions

## Suggested Ledger Entry

```text
Added future-facing Capability Endpoint expansion guidance.

Decision: Llama Wrangler remains Ollama-first for the current MVP. The immediate implementation goal is still to complete the friendly local-first Ollama control plane with managed/active nodes, passive endpoints, discovery, routing, sessions, model management, queueing, Splunk telemetry, and safe Frontier Delta foundations.

Future direction: after the core platform is functional, Llama Wrangler may evolve from an Ollama fleet manager into a broader capability endpoint control plane. Potential future endpoints include LM Studio, MLX/MLStudio, vLLM, frontier providers, Codex-style agents, GitHub, Xcode, Docker, CI/CD, Splunk, and other automation surfaces.

Current implementation guidance: do not build those future systems now. Only keep APIs, schemas, policy, routing, and telemetry adaptable enough that future capability endpoint support can be added without a major rewrite.
```

## Suggested Codex Prompt

Use this prompt only after adding this document to the project.

```text
Read the new additive document for future-facing Capability Endpoint expansion.

Do not implement GitHub, Xcode, Docker, CI/CD, Codex-agent, or other broad capability endpoint integrations yet.

The current priority remains completing Llama Wrangler as a functional, friendly Ollama fleet control plane with managed/active nodes, passive Ollama endpoints, discovery, enrollment, routing, session handling, model lifecycle, Splunk telemetry, and safe defaults.

Your task is to review current API, schema, telemetry, routing, and documentation language and make only low-risk adjustments that keep the system adaptable to future capability endpoints.

Allowed changes:
- add endpoint_type, endpoint_mode, trust_level, control_level, or capability metadata where it naturally fits
- document Capability Endpoints as future scope
- avoid hard-coding Ollama-only assumptions in generic routing/policy/telemetry structures
- keep Ollama-specific implementation nested or clearly scoped

Disallowed changes:
- do not implement new non-Ollama integrations
- do not add UI pages for future integrations
- do not replace the current node model with an unfinished generic abstraction
- do not derail managed/passive Ollama node work
- do not expand MVP scope beyond architectural adaptability

After changes, update the project ledger with:
- what was adjusted
- what was intentionally deferred
- confirmation that current MVP priorities remain unchanged
```
