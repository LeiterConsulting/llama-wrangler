# Splunk App Scope

## App name

Suggested package id: `llama_wrangler_observability`

Display name: **Llama Wrangler Observability**

## Purpose

The Splunk app provides observability for Llama Wrangler operations, including request routing, endpoint health, model performance, consensus behavior, frontier usage, failures, queue depth, and cost controls.

## Data ingestion

Primary ingestion method: Splunk HEC.

Recommended index: `llama_wrangler`

Recommended sourcetypes:

- `llama_wrangler:request`
- `llama_wrangler:response`
- `llama_wrangler:node_health`
- `llama_wrangler:routing_decision`
- `llama_wrangler:consensus`
- `llama_wrangler:frontier_delta`
- `llama_wrangler:validation`
- `llama_wrangler:error`
- `llama_wrangler:config`

## Dashboards

### Executive Overview

- Total requests
- Successful requests
- Error rate
- Local vs frontier percentage
- Average latency
- Average tokens/sec
- Top clients
- Top model aliases
- Cost estimate

### Node Health

- Endpoint up/down status
- CPU/memory/VRAM pressure
- Active jobs
- Queue depth
- Ollama availability
- Model inventory drift

### Model Performance

- Tokens/sec by node/model
- Latency by model alias
- Prompt/completion token estimates
- Error rate by model
- Load by endpoint

### Routing Decisions

- Selected node by task type
- Fallback usage
- Routing reason distribution
- Policy blocks
- Alias resolution

### Consensus and Delta

- Consensus mode usage
- Participant count
- Agreement rate
- Validator pass/fail rate
- Frontier escalation rate
- Common disagreement types

### Frontier Usage

- Provider usage
- Estimated cost
- Escalation reasons
- Payload class: summary/snippet/source-code
- Approval events
- Blocked escalations

### Client / IDE Usage

- Requests by client
- Model alias by client
- Average latency by client
- Failures by client

## Field strategy

Use indexed JSON and search-time extractions. Keep event payloads flat where possible, but allow nested fields for candidates, participants, benchmark reports, and frontier policy.
