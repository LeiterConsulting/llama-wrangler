# Capability Model

Each subscriber reports capabilities to the marshal. The marshal uses these reports to assign roles, route requests, and select participants for parallel execution.

Managed Nodes report capabilities through the Llama Wrangler subscriber. Passive Endpoints provide only marshal-observed capability signals from an existing Ollama-compatible endpoint URL. Any capability field not directly reported by a subscriber or observed by the marshal must be treated as unknown.

## Future Direction: Capability Endpoints

Current capability work remains focused on Ollama Managed Nodes and Ollama-compatible Passive Endpoints. Future versions may generalize these records into broader Capability Endpoints for additional runtimes, providers, agent surfaces, build runners, or automation tools.

That future direction should not expand the V1 implementation scope. The current model should simply avoid blocking later additions by keeping control level, trust level, capability source, approval state, freshness, policy, and telemetry metadata generic where practical. Ollama-specific behavior should remain clearly scoped to Ollama routes, model inventory, and node metadata.

See `docs/19_capability_endpoint_future_plan.md` for the V1/V2 boundary.

## Node capability fields

- node_id
- hostname
- platform
- architecture
- CPU details
- GPU details when available
- total memory
- available memory
- battery state when available
- thermal state when available
- Ollama URL
- Ollama version
- available models
- loaded models
- benchmark results
- observed tokens/sec by model
- observed error rate
- current jobs
- tags
- manual overrides
- control level: `managed` or `passive`
- trust level: `local`, `lan_trusted`, `lan_unverified`, or `external`
- capability source: `subscriber_reported`, `marshal_observed`, or `manual`

Future endpoint records may add fields such as `endpoint_type`, `capabilities`, or endpoint-specific nested metadata, but those fields are deferred until a real V2 integration needs them.

## Auto roles

Possible auto roles:

- marshal
- router
- light-chat
- code
- primary-code
- code-fallback
- long-context
- summarizer
- json-extractor
- embedding
- batch
- evaluator
- critic
- reducer

## Scoring inputs

- requested model availability
- model alias match
- current load
- historical throughput
- historical quality score
- memory pressure
- endpoint health
- latency
- task type
- context size
- streaming requirement
- policy constraints
- node control level
- node trust level
- capability source and freshness

Default scoring prefers Managed Nodes when model availability and health are otherwise comparable. Passive Endpoints can be eligible for approved single-route requests, but are de-prioritized. `lan_unverified` endpoints are also de-prioritized for single-route requests. `external` trust is excluded from routing by default. Heartbeat-required Managed Nodes are excluded when their subscriber report is missing or stale. Consensus eligibility requires Managed Nodes with `local` or `lan_trusted` trust until a later explicit policy allows broader participation. Benchmark controls are Managed Node only at this stage; Passive Endpoints may only receive future marshal-observed probe metadata and must not be treated as locally benchmarkable assets.

V1 consensus applies model alias `min_participants` and `max_participants` after capability and policy scoring. Missing bounds default to 2 and 4; fan-out is capped at 8. Equal-score participants are ordered by stable node ID so no-majority winner selection remains deterministic. Benchmark placement may influence participant rank only when the summary is approved, fresh, trusted, subscriber-reported metadata.

## Example capability report

```json
{
  "node_id": "rtx4090",
  "platform": "windows",
  "arch": "amd64",
  "memory_total_gb": 64,
  "gpu": "NVIDIA RTX 4090 24GB",
  "ollama_url": "http://localhost:11434",
  "models": ["qwen3-coder:30b", "gemma4:12b-mlx"],
  "roles": ["primary-code", "heavy", "batch"],
  "load": {
    "active_jobs": 1,
    "max_jobs": 2
  },
  "benchmarks": {
    "qwen3-coder:30b": {
      "output_tokens_per_second": 78.2,
      "prefill_tokens_per_second": 920.5,
      "max_recommended_context": 131072
    }
  }
}
```

## Passive endpoint report shape

```json
{
  "node_id": "studio-ollama-url",
  "url": "http://studio.local:11434",
  "control_level": "passive",
  "trust_level": "lan_unverified",
  "capability_source": "marshal_observed",
  "models": ["llama3.1:8b"],
  "observed": {
    "last_health_check": "ok",
    "latency_ms": 42,
    "error_rate": 0.0
  }
}
```
