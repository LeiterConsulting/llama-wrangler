# Capability Model

Each subscriber reports capabilities to the marshal. The marshal uses these reports to assign roles, route requests, and select participants for parallel execution.

Managed Nodes report capabilities through the Llama Wrangler subscriber. Passive Endpoints provide only marshal-observed capability signals from an existing Ollama-compatible endpoint URL. Any capability field not directly reported by a subscriber or observed by the marshal must be treated as unknown.

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

Default scoring should prefer Managed Nodes when model availability and health are otherwise comparable. Passive Endpoints can be eligible for single-route requests after explicit user addition, but should be excluded from consensus, warm-model controls, and sensitive workloads unless trust policy explicitly allows them.

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
