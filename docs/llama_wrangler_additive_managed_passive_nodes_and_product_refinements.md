# Llama Wrangler Additive Requirements: Managed Nodes, Passive Endpoints, Product Polish, and Fleet Control Refinements

Last updated: 2026-07-09

This document is an additive requirements package for the existing Llama Wrangler project. It should be added to the project docs and treated as a binding product/design refinement alongside the existing UI-first and hardening additive prompts.

It captures additional product details discussed after the original package and ledger were created, especially:

- Managed Node versus Passive Endpoint support
- active deploy versus passive endpoint mode
- capability-aware participation across both node types
- UI/product polish expectations
- app chrome/logo guidance
- onboarding flow changes
- trust/control-aware routing
- friend-shareable product framing

The goal is to preserve the current project direction while expanding Llama Wrangler into a more flexible, mature, easy-to-adopt local AI fleet manager.

---

## 1. Product Framing

Llama Wrangler is a local-first AI fleet manager and control plane for Ollama-compatible runtimes.

It presents one friendly endpoint to IDEs, agents, local apps, and automation tools while coordinating many local or remote Ollama workers behind the scenes.

Short product description:

> Llama Wrangler lets multiple Macs, PCs, servers, and existing Ollama endpoints behave like one smarter local inference backend, with routing, sessions, queueing, safety controls, Splunk observability, and optional frontier-model escalation only when explicitly permitted.

Short tagline candidates:

- One endpoint. Many llamas.
- Wrangle your local AI herd.
- A friendly control plane for local LLM fleets.
- Local-first inference, coordinated.

Core positioning:

- Llama Wrangler does not split one model across multiple machines.
- Llama Wrangler does not replace Ollama.
- Llama Wrangler does not require Kubernetes.
- Llama Wrangler does not default to cloud/frontier inference.
- Llama Wrangler makes multiple Ollama-capable assets easier to install, observe, route, secure, and use.

---

## 2. New Core Concept: Managed Nodes and Passive Endpoints

Llama Wrangler should support two first-class ways to add compute to the fleet:

1. **Managed Node**
2. **Passive Endpoint**

These should be visible in the UI, reflected in config/state/API metadata, and considered during routing, benchmarking, consensus, policy enforcement, and telemetry.

### 2.1 Managed Node

A Managed Node is an asset where Llama Wrangler is installed alongside Ollama.

Architecture:

```text
Machine / Asset
  ├── Ollama runtime
  └── Llama Wrangler subscriber
```

The subscriber provides full-control integration with the marshal.

Managed Node capabilities:

- local hardware detection
- OS and architecture detection
- local Ollama detection
- installed model inventory
- model state reporting
- Ollama health checks
- model pull/remove/warm controls, where policy allows
- benchmark execution
- request proxying
- cancellation telemetry
- local queue/load reporting
- local model load/warm/busy/failed state
- richer per-node operational metrics
- subscriber heartbeat
- secure enrollment
- future OS service lifecycle management
- future OS keychain integration

Managed Nodes are preferred for:

- trusted local fleet assets
- sensitive/local-only workloads
- consensus participant selection
- task-level parallelism
- benchmark-driven routing
- deterministic validation workflows
- long-running agentic sessions
- policy-controlled frontier pre-review

Example assets:

```text
M4 Mac mini       → marshal / light local worker
M3 Pro MacBook    → medium MLX worker
M1 Max MacBook    → memory-rich Apple Silicon worker
RTX 4090 desktop  → heavy coding/model worker
Linux lab server  → managed always-on worker
```

### 2.2 Passive Endpoint

A Passive Endpoint is an existing Ollama-compatible endpoint that Llama Wrangler can use, but where the Wrangler subscriber is not installed.

Architecture:

```text
Existing asset
  └── Ollama-compatible API endpoint

Marshal
  └── knows only the endpoint URL and optional auth/policy metadata
```

Passive Endpoint capabilities:

- health check
- model inventory through `/api/tags` where available
- request proxying
- basic latency/error metrics observed by the marshal
- model alias assignment
- routing eligibility
- participation in consensus if policy allows

Passive Endpoint limitations:

- no reliable hardware detection
- no local RAM/VRAM/load telemetry
- no subscriber heartbeat
- no local model warm-state certainty
- no local benchmark execution except via prompt-based tests
- no secure local secret store on that node
- no robust cancellation beyond what the endpoint/API supports
- limited model lifecycle management
- limited trust and observability

Passive Endpoints are useful for:

- existing Ollama servers
- temporary lab endpoints
- remote boxes where installation is not desired
- shared endpoints
- cloud/VPN-hosted Ollama-compatible runtimes
- friends/team members exposing a trusted endpoint
- quick onboarding before installing a full subscriber

---

## 3. UI Flow: Add Node

The Nodes page should support two visibly different flows.

### Add Node Option 1: Install / Enroll Managed Node

UI copy:

```text
Managed Node
Install Llama Wrangler on this asset for full control, health reporting, model management, benchmarking, and richer telemetry.
Recommended for machines you own or trust.
```

Flow:

1. User clicks **Add Managed Node**.
2. UI displays install instructions or a copyable command for the target OS.
3. Subscriber starts on the target machine.
4. Subscriber discovers marshal through mDNS or manual URL/token.
5. Marshal shows pending enrollment.
6. User approves node.
7. Node reports capabilities.
8. Wrangler recommends roles.
9. User accepts or overrides roles.
10. Node appears with **Managed Node** badge.

### Add Node Option 2: Add Existing Ollama Endpoint

UI copy:

```text
Passive Endpoint
Use an existing Ollama-compatible endpoint without installing Llama Wrangler on that machine.
This is easier to add, but provides limited control and observability.
```

Flow:

1. User clicks **Add Passive Endpoint**.
2. User enters endpoint URL.
3. Optional: user enters auth/API key if needed.
4. Wrangler tests `/api/tags` and health behavior.
5. Wrangler displays detected models and limitations.
6. User assigns trust level and allowed roles.
7. Endpoint appears with **Passive Endpoint** badge.

---

## 4. Node Metadata Requirements

Each node should include explicit mode, trust, and control metadata.

Suggested shape:

```yaml
nodes:
  - id: m1-max
    display_name: "M1 Max MacBook Pro"
    mode: managed
    control_level: full
    trust_level: managed
    subscriber_url: "http://m1-max.local:11436"
    ollama_url: "http://localhost:11434"
    roles:
      - long-context
      - code-fallback

  - id: spare-ollama
    display_name: "Existing Ollama Endpoint"
    mode: passive
    control_level: inference_only
    trust_level: trusted_lan
    ollama_url: "http://sparebox.local:11434"
    roles:
      - general
      - overflow
```

Required fields/concepts:

- `mode`: `managed`, `passive`, `marshal`, or `standalone`
- `control_level`: `full`, `limited`, `inference_only`
- `trust_level`: `managed`, `trusted_lan`, `untrusted`, `experimental`
- `subscriber_url`: only for managed subscribers
- `ollama_url`: required for passive endpoints and used internally by subscribers
- `roles`: explicit or recommended workload roles
- `capability_source`: `reported`, `observed`, `estimated`, or `manual`
- `allowed_execution_modes`: list of allowed modes such as `single`, `race`, `consensus`, `frontier_precheck`
- `policy_constraints`: node-specific safety or routing limits

---

## 5. Capability Matrix

The UI and routing engine should reflect that Managed Nodes and Passive Endpoints have different capabilities.

| Capability | Managed Node | Passive Endpoint |
|---|---:|---:|
| Proxy inference | Yes | Yes |
| Health check | Yes | Yes |
| Model inventory | Yes | Usually |
| Hardware detection | Yes | No |
| OS/architecture detection | Yes | No |
| Model pull/remove | Yes, policy-limited | Maybe, usually no |
| Keep-warm control | Yes | Limited/unknown |
| Benchmark execution | Yes | Limited/observed only |
| Load/queue telemetry | Yes | No/estimated |
| Cancellation tracking | Better | Limited |
| Subscriber heartbeat | Yes | No |
| Secure enrollment | Yes | URL/API key based |
| Splunk node telemetry | Rich | Marshal-observed only |
| Trust/control scoring | High | Depends on policy |
| Recommended for sensitive work | Yes | Usually no |

---

## 6. Routing and Policy Changes

Routing must consider node integration mode and control level.

### 6.1 Managed Nodes Preferred for High-Control Work

Prefer Managed Nodes for:

- sensitive prompts
- local-only mode
- long-running sessions
- code review
- local repo analysis
- deterministic validation
- consensus judging
- workload benchmarking
- frontier-delta pre-review
- workloads requiring reliable cancellation

### 6.2 Passive Endpoints Allowed for Lower-Control Work

Passive Endpoints may be used for:

- generic chat
- summarization
- overflow capacity
- non-sensitive background jobs
- consensus participation when explicitly allowed
- experiments

### 6.3 Suggested Routing Rules

Example policy logic:

```text
If request is local-only and sensitive:
  use managed nodes only.

If request is generic and passive endpoints are allowed:
  include passive endpoints if healthy and model-compatible.

If request requires benchmarking data:
  prefer nodes with reported/validated capability data.

If request is consensus_delta:
  include passive endpoints only if trust_level allows participation.

If request requires cancellation safety:
  prefer managed nodes.
```

### 6.4 UI Routing Explanation

Request history should show why a node was selected.

Example:

```json
{
  "selected_node": "rtx4090",
  "node_mode": "managed",
  "reason": [
    "matched alias local-code",
    "highest observed code-generation score",
    "model warm",
    "managed node preferred for code task"
  ],
  "excluded_nodes": [
    {
      "node": "spare-ollama",
      "mode": "passive",
      "reason": "passive endpoints disabled for sensitive code tasks"
    }
  ]
}
```

---

## 7. Consensus and Parallel Participation Updates

Consensus mode should support both Managed Nodes and Passive Endpoints, but with policy controls.

### 7.1 Participation Rules

Participant selection should consider:

- task type
- node mode
- trust level
- model availability
- current load
- model warmth
- historical performance
- local-only policy
- passive endpoint participation policy
- whether deterministic validation is possible

### 7.2 Managed Node Consensus

Managed Nodes can provide richer participation:

- local execution metrics
- better cancellation
- model state
- benchmark-informed scoring
- deterministic validators installed locally in future versions
- reliable node identity

### 7.3 Passive Endpoint Consensus

Passive Endpoints can contribute candidate outputs but should be treated differently:

- lower confidence by default unless historically validated
- no trusted hardware/load reporting
- observed metrics only
- participation may be disabled for sensitive tasks

### 7.4 Recommended Default

Default consensus policy:

```yaml
consensus:
  allow_passive_endpoints: false
  allow_passive_for_non_sensitive: true
  require_managed_for_sensitive: true
  minimum_managed_participants: 1
```

---

## 8. Frontier Delta Updates

Frontier Delta must also understand node trust/control mode.

Before generating a frontier payload, Wrangler should know whether the local analysis came from:

- fully managed nodes
- passive endpoints
- mixed fleet
- untrusted/experimental nodes

Frontier Delta payload metadata should include node contribution posture without leaking secrets or payloads.

Example:

```json
{
  "frontier_delta_context": {
    "local_participants": 4,
    "managed_participants": 3,
    "passive_participants": 1,
    "passive_outputs_included": false,
    "redaction_applied": true,
    "policy_mode": "approval_required"
  }
}
```

Suggested policy:

- Do not include passive endpoint outputs in frontier payloads by default.
- Allow passive output inclusion only if explicitly enabled.
- Require approval before sending source-like or customer-like content externally.
- Preserve local-only and metadata-only defaults.

---

## 9. Telemetry and Splunk Updates

Splunk HEC telemetry should include node mode and control metadata.

### 9.1 Event Field Additions

Add these fields where relevant:

```text
node_mode
node_control_level
node_trust_level
capability_source
managed_node_count
passive_endpoint_count
selected_node_mode
selected_node_control_level
selected_node_trust_level
passive_endpoint_allowed
passive_endpoint_used
passive_endpoint_excluded_reason
```

### 9.2 New Event Types

Add or extend telemetry for:

- managed node enrollment requested
- managed node approved
- managed node rejected
- passive endpoint added
- passive endpoint health changed
- passive endpoint model inventory updated
- node control level changed
- node trust level changed
- passive endpoint excluded from routing
- passive endpoint included in consensus

### 9.3 Splunk Dashboard Updates

The Splunk app should distinguish Managed Nodes from Passive Endpoints.

Dashboard additions:

- Fleet Composition
  - managed nodes
  - passive endpoints
  - standalone/marshal nodes
- Node Trust and Control Overview
- Passive Endpoint Health
- Managed Node Capability Detail
- Routing by Node Mode
- Consensus Participation by Node Mode
- Passive Endpoint Exclusion Reasons
- Security Posture Overview

Alert candidates:

- passive endpoint repeatedly failing
- passive endpoint used for sensitive task
- unmanaged endpoint added
- passive endpoint included in consensus unexpectedly
- node trust level changed
- LAN-exposed marshal with passive endpoints enabled

---

## 10. UI and Product Polish Requirements

The application should feel like a friendly desktop/local-lab app, not a utilitarian admin console.

### 10.1 Logo and App Chrome

The app should have a distinct Llama Wrangler visual identity.

Design direction:

- playful but not childish
- tongue-in-cheek
- inspired by the idea of wrangling a herd of local llamas
- visually compatible with Ollama/local AI culture but completely unique
- usable in app chrome, sidebar, favicon, installer, README, and docs
- must work at small sizes
- should have icon-only and full-lockup variants

Possible visual motifs:

- llama herd
- lasso
- route lines/network nodes
- small cowboy/wrangler theme
- terminal/endpoint hints
- friendly local control-plane vibe

Required logo variants:

- icon-only square
- horizontal lockup
- sidebar mark
- favicon/app icon
- monochrome/single-color version
- transparent background version

Chrome guidance:

- add the logo to the sidebar/header
- preserve readable contrast
- avoid excessive visual clutter
- make status badges friendly and obvious
- maintain trust/safety cues in the UI

### 10.2 Node Badges

UI badges should clearly distinguish:

- Marshal
- Managed Node
- Passive Endpoint
- Standalone
- Localhost Only
- LAN Exposed
- Local Only Mode
- Frontier Disabled
- Frontier Enabled
- Metadata Only Logging
- TLS Verification Disabled

### 10.3 Friendly Setup Language

Use simple language in the UI.

Instead of:

```text
Configure subscriber node with capability metadata.
```

Prefer:

```text
Add a managed worker so Wrangler can see its models, health, and performance.
```

Instead of:

```text
Register passive endpoint.
```

Prefer:

```text
Use an existing Ollama endpoint.
```

---

## 11. Installation and Adoption Path

The project should support an easy adoption ladder.

### Step 1: Standalone / Localhost

User runs Wrangler locally and uses local Ollama.

```text
One machine. One endpoint. Safe defaults.
```

### Step 2: Add Passive Endpoint

User adds an existing Ollama server without installing anything else.

```text
Quickly use another endpoint with limited control.
```

### Step 3: Upgrade to Managed Node

User installs Wrangler subscriber on that machine.

```text
Gain full health, model, benchmark, and telemetry control.
```

### Step 4: Fleet Mode

Multiple managed nodes plus optional passive endpoints.

```text
Capability-aware routing, consensus, queueing, and observability.
```

### Step 5: Hybrid Mode

Optional Frontier Delta with explicit policy, approval, and redaction.

```text
Local-first, frontier-assisted only when allowed.
```

---

## 12. API Additions

Suggested management endpoints:

```text
GET  /wrangler/nodes
POST /wrangler/nodes/managed/enrollment/start
POST /wrangler/nodes/managed/enrollment/approve
POST /wrangler/nodes/managed/enrollment/reject
POST /wrangler/nodes/passive
PUT  /wrangler/nodes/{id}/trust
PUT  /wrangler/nodes/{id}/roles
PUT  /wrangler/nodes/{id}/policy
POST /wrangler/nodes/{id}/test
POST /wrangler/nodes/{id}/promote-to-managed
```

Passive endpoint create body example:

```json
{
  "display_name": "Basement Ollama",
  "ollama_url": "http://basement.local:11434",
  "trust_level": "trusted_lan",
  "allowed_roles": ["general", "overflow"],
  "allow_consensus": false,
  "allow_sensitive_tasks": false
}
```

Node response example:

```json
{
  "id": "basement-ollama",
  "display_name": "Basement Ollama",
  "mode": "passive",
  "control_level": "inference_only",
  "trust_level": "trusted_lan",
  "health": "online",
  "capability_source": "observed",
  "models": ["llama3.1:8b", "qwen2.5-coder:14b"],
  "limitations": [
    "hardware not reported",
    "load state unavailable",
    "model warm state estimated",
    "limited cancellation control"
  ]
}
```

---

## 13. Documentation Updates Needed

Update or add docs for:

- Managed Nodes versus Passive Endpoints
- node control levels
- node trust levels
- passive endpoint limitations
- onboarding flows
- routing implications
- consensus participation policy
- Frontier Delta interaction with passive endpoints
- Splunk telemetry additions
- UI/logo/app chrome guidance
- adoption ladder

Suggested new doc file:

```text
docs/14_managed_nodes_and_passive_endpoints.md
```

Suggested ledger update item:

```text
Added Managed Node and Passive Endpoint as first-class node modes. Managed Nodes run the Wrangler subscriber alongside Ollama and provide full control, capability reporting, benchmarking, model state, and richer telemetry. Passive Endpoints are existing Ollama-compatible URLs that can be used for inference with limited control and marshal-observed telemetry only. Routing, consensus, Splunk telemetry, and UI badges must account for node mode, trust level, and control level.
```

---

## 14. Non-Goals and Guardrails

This additive scope must not weaken existing safe defaults.

Do not:

- make passive endpoints trusted by default
- use passive endpoints for sensitive workloads by default
- include passive endpoint outputs in Frontier Delta payloads by default
- require passive endpoints to install anything
- require managed nodes for basic onboarding
- expose LAN access without explicit approval
- log prompt/response bodies by default
- treat passive endpoint model inventory as equivalent to managed node capability reporting
- silently send external/frontier requests
- use passive endpoints to execute server-side arbitrary tools

Preserve all existing non-negotiables from the ledger.

---

## 15. Acceptance Criteria

Implementation should be considered successful when a user can:

1. Open the Llama Wrangler UI.
2. See a clear distinction between Managed Nodes and Passive Endpoints.
3. Add an existing Ollama endpoint passively by URL.
4. Test the passive endpoint.
5. See detected passive endpoint models and limitations.
6. Assign trust level and allowed roles to the passive endpoint.
7. Install/enroll a Managed Node separately.
8. See richer capabilities for Managed Nodes than Passive Endpoints.
9. Use both node types through the same marshal endpoint.
10. See routing decisions reflect node mode and trust level.
11. Exclude passive endpoints from sensitive/local-only work by default.
12. Include passive endpoints in low-risk routing only when policy allows.
13. See node mode fields in telemetry and support bundles without secrets or payloads.
14. See Splunk dashboards distinguish Managed Nodes from Passive Endpoints.
15. Use the product without editing YAML or `.env` for normal setup.

---

## 16. Suggested Codex Prompt

Use the following prompt to implement this additive requirement in the existing Llama Wrangler project.

```text
You are continuing the existing Llama Wrangler project. Read the project ledger, existing docs, UI-first additive prompt, hardening additive prompt, and all current implementation files before coding.

Implement Managed Node and Passive Endpoint as first-class node integration modes.

A Managed Node is a machine where the Llama Wrangler subscriber is installed alongside Ollama and can report hardware, model state, health, load, benchmarks, and richer telemetry.

A Passive Endpoint is an existing Ollama-compatible endpoint URL that the marshal can use for inference without installing Wrangler on that asset. Passive endpoints provide limited control and marshal-observed telemetry only.

Requirements:

1. Extend node models/state/API responses to include mode, control_level, trust_level, capability_source, limitations, and allowed execution/role metadata.
2. Preserve existing manual subscriber behavior as Managed Node behavior where appropriate.
3. Add a UI/API flow for adding an existing Ollama endpoint as a Passive Endpoint.
4. Test passive endpoints through health/model inventory calls without exposing secrets.
5. Show clear UI badges for Managed Node versus Passive Endpoint.
6. Update routing so node mode, control level, and trust level can affect eligibility.
7. Prefer Managed Nodes for sensitive/local-only/code/consensus/validation workflows by default.
8. Allow Passive Endpoints for general/overflow/non-sensitive tasks when policy allows.
9. Add telemetry fields for node_mode, node_control_level, node_trust_level, capability_source, passive_endpoint_used, and passive_endpoint_excluded_reason.
10. Update support bundle output to include node mode/control/trust metadata while preserving all existing redaction guarantees.
11. Update docs and README to explain Managed Nodes versus Passive Endpoints.
12. Add tests for passive endpoint creation, health/model detection, routing eligibility, default exclusion from sensitive tasks, telemetry field shape, and support-bundle redaction preservation.

Guardrails:

- Do not require YAML or `.env` edits for normal use.
- Do not expose admin tokens, client API keys, HEC tokens, auth headers, prompts, responses, request bodies, or payloads.
- Do not weaken localhost-by-default behavior.
- Do not enable frontier providers by default.
- Do not treat Passive Endpoints as fully trusted by default.
- Do not split a single model across machines.
- Do not replace Ollama.
- Do not execute arbitrary tools server-side.

Verify with:

- go test ./...
- go build ./cmd/llama-wrangler
- live /healthz check
- live /ui/ check
- UI browser check for Nodes page badges/forms
- support-bundle privacy check
```

---

## 17. Summary

This additive requirement makes Llama Wrangler easier to adopt and more mature.

The user should be able to start simple by adding an existing Ollama endpoint, then upgrade that endpoint into a fully managed Wrangler subscriber later.

The product should clearly distinguish:

```text
Managed Node = full control, rich telemetry, benchmark-aware routing.
Passive Endpoint = easy onboarding, inference-only, limited trust/control.
```

This distinction improves onboarding, security, routing accuracy, observability, and long-term product clarity.
