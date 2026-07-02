# Event Schema

## Common fields

All events should include:

```json
{
  "timestamp": "2026-07-01T12:00:00.000Z",
  "event_type": "request",
  "request_id": "req_abc123",
  "trace_id": "trace_abc123",
  "marshal_node_id": "m4-mini",
  "version": "0.1.0"
}
```

## Request event

Sourcetype: `llama_wrangler:request`

```json
{
  "event_type": "request",
  "request_id": "req_abc123",
  "client_name": "cline",
  "client_type": "ide",
  "api_surface": "openai_chat_completions",
  "model_requested": "local-code",
  "model_alias": "local-code",
  "execution_mode": "consensus_delta",
  "stream": true,
  "prompt_tokens_est": 8124,
  "context_tokens_est": 8124,
  "task_type": "code",
  "privacy_mode": "local_first"
}
```

## Routing decision event

Sourcetype: `llama_wrangler:routing_decision`

```json
{
  "event_type": "routing_decision",
  "request_id": "req_abc123",
  "model_alias": "local-code",
  "resolved_model": "qwen3-coder:30b",
  "selected_node": "rtx4090",
  "candidate_nodes": ["rtx4090", "m1-max"],
  "fallback_nodes": ["m1-max"],
  "routing_strategy": "weighted_best_available",
  "routing_reasons": [
    "requested_model_available",
    "highest_observed_throughput",
    "role_primary_code"
  ]
}
```

## Response event

Sourcetype: `llama_wrangler:response`

```json
{
  "event_type": "response",
  "request_id": "req_abc123",
  "status": "success",
  "selected_node": "rtx4090",
  "resolved_model": "qwen3-coder:30b",
  "latency_ms": 18422,
  "time_to_first_token_ms": 1380,
  "completion_tokens_est": 1420,
  "tokens_per_second": 77.1,
  "fallback_used": false,
  "frontier_used": false
}
```

## Node health event

Sourcetype: `llama_wrangler:node_health`

```json
{
  "event_type": "node_health",
  "node_id": "m1-max",
  "hostname": "m1max.local",
  "platform": "darwin",
  "arch": "arm64",
  "status": "healthy",
  "ollama_available": true,
  "memory_total_gb": 64,
  "memory_available_gb": 38,
  "active_jobs": 1,
  "max_jobs": 2,
  "models": ["qwen3-coder:30b", "gemma4:12b-mlx"]
}
```

## Consensus event

Sourcetype: `llama_wrangler:consensus`

```json
{
  "event_type": "consensus",
  "request_id": "req_abc123",
  "execution_mode": "consensus_delta",
  "participants": ["rtx4090", "m1-max", "m3-pro"],
  "participant_count": 3,
  "agreement_score": 0.67,
  "validator_passed": false,
  "winner_node": "rtx4090",
  "disagreement_detected": true,
  "escalation_recommended": true,
  "escalation_reason": "no_majority"
}
```

## Frontier Delta event

Sourcetype: `llama_wrangler:frontier_delta`

```json
{
  "event_type": "frontier_delta",
  "request_id": "req_abc123",
  "provider": "openai",
  "frontier_model": "configured-model-name",
  "escalation_reason": ["no_majority", "validation_failure"],
  "payload_type": "summaries_and_snippets",
  "source_code_sent": false,
  "secrets_detected": false,
  "redaction_applied": true,
  "estimated_cost_usd": 0.08,
  "approved": true
}
```

## Error event

Sourcetype: `llama_wrangler:error`

```json
{
  "event_type": "error",
  "request_id": "req_abc123",
  "node_id": "m3-pro",
  "error_class": "ollama_unavailable",
  "error_message": "connection refused",
  "retryable": true,
  "fallback_used": true
}
```
