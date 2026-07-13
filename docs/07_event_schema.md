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

## Queue state event

Sourcetype: `llama_wrangler:queue_state`

```json
{
  "event_type": "queue_state",
  "request_id": "req_abc123",
  "status": "active",
  "priority": "high",
  "queue_depth": 2,
  "queue_capacity": 128,
  "surface": "openai_chat_completions",
  "stream": true,
  "scheduling_policy": "weighted_priority"
}
```

## Streaming outcome events

Sourcetypes: `llama_wrangler:upstream_retry`, `llama_wrangler:response_partial`, and `llama_wrangler:request_cancelled`.

Allowed operational fields include request ID, selected/previous/next node ID, reason code, retry phase, retry allowed, partial-output flag, bytes written, stream flag, priority, queue status, scheduling policy, and timestamp. These events describe retry/cancellation state only.

## Benchmark scheduler and runner events

Sourcetypes include:

- `llama_wrangler:benchmark_job_claimed`
- `llama_wrangler:benchmark_job_status`
- `llama_wrangler:benchmark_scheduler_reconcile`
- `llama_wrangler:benchmark_scheduler_manual_reconcile`
- `llama_wrangler:benchmark_scheduler_background_tick`
- `llama_wrangler:benchmark_scheduler_policy_updated`
- `llama_wrangler:subscriber_benchmark_runner_tick`

Allowed fields include node/benchmark/suite IDs, status, scheduler policy/state, attempt bounds, changed/timed-out/retried/exhausted counts, runner mode/enablement/result policy, task count, claimed/completed/failed/no-job flags, bounded intervals, and safe error codes.

## Model lifecycle action events

Sourcetypes include:

- `llama_wrangler:model_lifecycle_action_queued`
- `llama_wrangler:model_lifecycle_action_claimed`
- `llama_wrangler:model_lifecycle_action_status`
- `llama_wrangler:model_lifecycle_action_rejected`

Allowed fields include node/control/trust/approval metadata, action ID/type, model name, desired keep-warm flag, status, safe error code, and policy reason codes.

Queue, streaming, benchmark, routing, consensus, and model lifecycle operational events are metadata-only. They must not contain inference content, extracted content, raw request/response material, raw headers, authorization data, secret values, local fixture contents or full paths, comparison signatures, validator/evaluator input, or arbitrary payload fields.

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
  "required_participants": 2,
  "max_participants": 4,
  "successful_participants": ["rtx4090", "m1-max"],
  "successful_count": 2,
  "failed_participants": ["m3-pro"],
  "failed_count": 1,
  "participant_failures": [
    {
      "node_id": "m3-pro",
      "reason_code": "upstream_5xx",
      "status_code": 503,
      "duration_ms": 412
    }
  ],
  "failure_reason_counts": {"upstream_5xx": 1},
  "agreement_score": 0.67,
  "agreement_count": 2,
  "comparison_strategy": "exact_normalized",
  "validator_passed": false,
  "winner_node": "rtx4090",
  "consensus_reached": true,
  "disagreement_detected": true,
  "timed_out": false,
  "client_cancelled": false,
  "duration_ms": 1840,
  "escalation_recommended": false,
  "escalation_reason": "",
  "frontier_used": false,
  "content_recorded": false
}
```

Consensus events are metadata-only. Participant failure reason codes are limited to `missing_proxy_url`, `connection_error`, `upstream_4xx`, `upstream_5xx`, `body_read_failure`, `response_size_limit`, `timeout`, and `cancellation`. A participant failure may include only node ID, fixed reason code, optional numeric upstream status, and duration. Events must not contain request bodies, prompt text, response text, extracted content, arbitrary error text, upstream URLs, raw headers, credentials, token values, validator input, evaluator input, or response signatures. Participant IDs, counts, timing, agreement score, strategy ID, winner node, fixed failure reasons, and safe outcome flags are allowed.

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
