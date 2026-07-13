# Future Direction: Capability Endpoints

This document captures the future-facing Capability Endpoint expansion path for Llama Wrangler. It is not part of the current MVP implementation target. Current work remains focused on making Llama Wrangler functional as a friendly Ollama fleet control plane with Managed Nodes, Passive Endpoints, discovery, enrollment, routing, sessions, model management, benchmark metadata, Splunk telemetry, and safe defaults.

## Source Documents

This plan incorporates:

- `docs/llama_wrangler_additive_managed_passive_nodes_and_product_refinements.md`
- `docs/llama_wrangler_additive_future_capability_endpoints.md`

These documents are binding product direction. They modify planning and ordering, but they do not require immediate implementation of non-Ollama integrations.

## V1 Boundary

V1 remains Ollama-first.

V1 should complete:

- local-first setup and operations UI
- standalone, marshal, and subscriber modes
- Managed Node enrollment, approval, heartbeat, trust, and benchmark metadata
- Passive Endpoint add, validation, approval, trust, and marshal-observed metadata
- OpenAI-compatible and Ollama-compatible inference APIs
- routing, sessions, queue visibility, and safe streaming behavior
- model inventory, aliasing, and model lifecycle foundations
- metadata-only observability and Splunk HEC integration
- support bundles that exclude secrets and payloads
- optional Frontier Delta foundations with explicit policy, redaction, and approval

V1 should not implement:

- GitHub, Xcode, Docker, CI/CD, build-runner, or TestFlight integrations
- arbitrary tool execution
- plugin execution
- marketplace or third-party extension systems
- non-Ollama runtime management beyond Passive Endpoint compatibility
- cloud/frontier provider use without explicit policy and approval

## V2 Direction

After V1 is functional, Llama Wrangler can evolve from an Ollama fleet control plane into a broader Capability Endpoint control plane.

Future endpoint types may include:

- Ollama Managed Nodes
- Ollama Passive Endpoints
- LM Studio or MLX-compatible local runtimes
- vLLM or OpenAI-compatible inference servers
- frontier providers
- Codex-style agent runtimes
- IDE agents such as Cline or Continue
- GitHub, Xcode, Docker, CI/CD, Splunk, and other automation surfaces

The conceptual hierarchy should remain:

```text
Fleet / Registry
  -> Capability Endpoint
       -> Ollama Managed Node
       -> Ollama Passive Endpoint
       -> Future endpoint type
```

## Compatibility Rules For Current Work

Current V1 code and docs should keep future support possible by following these rules:

- Treat visible "Nodes" terminology as acceptable V1 product language.
- Keep Managed Node and Passive Endpoint behavior first-class, not hidden YAML-only concepts.
- Keep control level, trust level, capability source, approval state, and freshness metadata in node-facing APIs.
- Keep telemetry fields generic where practical: endpoint or node ID, control level, trust level, capability source, capability, task type, routing policy, status, and latency.
- Keep Ollama-specific behavior scoped to Ollama routes, node fields, or nested metadata instead of making generic policy/telemetry depend on Ollama-only assumptions.
- Keep support bundles additive and self-describing so future endpoint metadata can be added without weakening redaction guarantees.
- Avoid adding empty UI navigation for future integrations before implementation begins.
- Avoid schema churn until a real V2 capability endpoint implementation starts.

## Current Compatibility Assessment

The current Phase B foundation is compatible with the future direction:

- state schema version 2 already stores control level, trust level, capability source, approval state, source/freshness metadata, and support flags
- Managed Node and Passive Endpoint UI/API flows are separate
- routing already uses approval, control, trust, heartbeat freshness, and benchmark metadata
- Passive Endpoints remain marshal-observed and do not imply local control
- benchmark orchestration and scheduler metadata are Managed Node only
- telemetry, support bundles, bootstrap metadata, and UI warnings remain metadata-only
- secret storage is separate from ordinary app state

No runtime implementation changes are required solely for the future Capability Endpoint documents at this stage. The needed current action is documentation, task ordering, and continued discipline around generic policy/telemetry boundaries.

## Task Ordering

The near-term order remains:

1. Finish Phase B Managed Node and Passive Endpoint foundations.
2. Finish benchmark workload definitions and scheduler policy controls for V1.
3. Complete model lifecycle and warm-state surfaces for Managed Nodes without implying Passive Endpoint control.
4. Complete consensus behavior with Managed Node defaults and explicit Passive Endpoint policy gates.
5. Expand Splunk dashboards around node mode, trust, routing, consensus, and benchmark posture.
6. Finish Frontier Delta policy/redaction/approval foundations.
7. Package/install/service lifecycle for a normal desktop/local-lab user.
8. Only then begin V2 Capability Endpoint implementation.

## Deferred V2 Backlog

Keep these as future backlog items, not active V1 scope:

- generic `endpoint_type` and capability registry migrations
- `/wrangler/endpoints` compatibility layer or alias over `/wrangler/nodes`
- endpoint-type-specific capability adapters
- generic workflow/policy engine for non-inference capabilities
- GitHub, Xcode, Docker, CI/CD, TestFlight, and build-runner surfaces
- plugin or marketplace packaging
- arbitrary tool execution with sandbox and approval controls
- broader Splunk sourcetype evolution for non-Ollama endpoint classes

## Risks And Guardrails

- Risk: broad capability language could derail V1. Guardrail: label V2 features as future scope and keep current work Ollama-first.
- Risk: current API names could become too node-specific. Guardrail: keep `/wrangler/nodes` for V1, but use generic metadata names where they already fit.
- Risk: telemetry could become Ollama-only. Guardrail: add generic endpoint/control/trust/capability fields as events evolve.
- Risk: Passive Endpoints could be treated as Managed Nodes. Guardrail: keep limitations, badges, approval, trust warnings, and probe-only benchmark posture explicit.
- Risk: future automation surfaces could create payload and secret leakage. Guardrail: preserve metadata-only defaults, approval gates, redaction, and support-bundle exclusions.

## Acceptance Criteria

This planning slice is complete when:

- the new additive docs are listed as binding inputs in the project ledger
- future Capability Endpoints are documented as V2 scope
- current V1 task ordering remains Ollama-first
- Managed Node and Passive Endpoint work remains the active Phase B foundation
- no non-Ollama integrations are implemented prematurely
- next ledger steps continue V1 work while preserving the future compatibility guardrails
