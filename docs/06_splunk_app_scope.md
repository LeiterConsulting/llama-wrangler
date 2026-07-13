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

## Implemented V1 dashboards

### Llama Wrangler Overview

The packaged overview retains request, latency, execution-mode, node-health, and frontier summaries and adds compact metadata-only signals for consensus participant failures, peak queue depth, and streaming outcome events.

### Llama Wrangler Operations

The packaged operations dashboard includes:

- consensus run/failure totals, the eight fixed participant failure reasons over time, and recent safe outcome metadata
- queue depth by priority plus scheduling policy/priority/status summaries
- streaming retry, partial-response, and cancellation trends plus recent safe outcome rows
- benchmark job/scheduler reconciliation history and subscriber runner tick history
- routing policy exclusion reason distributions derived from `routing_decision.excluded_nodes` control/trust/capability-source metadata
- recent routing decisions with alias, strategy, selected node, execution mode, and consensus bounds
- model lifecycle action event trends and safe queue/claim/status/rejection history

Both dashboards share a time-range input and are exposed through packaged app navigation. The Operations dashboard uses reusable macros so index changes can remain localized to `llama_wrangler_index`.

## Knowledge objects

V1 packages macros and eventtypes for consensus, queue, streaming outcomes, benchmark scheduler, benchmark runner, routing, and model lifecycle actions. `props.conf` assigns event categories to each operational sourcetype. Disabled-by-default saved reports provide the same summary/history searches without imposing scheduling load on installation.

## Metadata-only contract

Operational Splunk searches may use request/node/action/job IDs, model and alias names, control/trust/approval metadata, fixed reason/error codes, status, timestamps, durations, counts, queue depth/capacity/priority/policy, participant counts, agreement score, scheduler state, runner mode, and result policy metadata. They must not query or derive inference content, extracted content, raw request/response material, raw headers, authorization data, secret values, local fixture contents or full paths, comparison signatures, validator/evaluator input, or arbitrary payload fields.

Consensus participant failure panels are limited to `missing_proxy_url`, `connection_error`, `upstream_4xx`, `upstream_5xx`, `body_read_failure`, `response_size_limit`, `timeout`, and `cancellation`.

## Field strategy

Use indexed JSON and search-time extractions. Keep event metadata flat where possible, but allow nested metadata for candidates, participant failures, benchmark reports, and frontier policy. Routing exclusion panels use `spath` only on the safe `excluded_nodes` metadata array.
