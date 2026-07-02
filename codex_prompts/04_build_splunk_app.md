# Codex Prompt: Build Splunk App for Llama Wrangler

You are building the companion Splunk app for Llama Wrangler observability.

## Goal

Create a Splunk app that ingests JSON HEC events from Llama Wrangler and provides dashboards for operational visibility.

## App identity

- Folder: `llama_wrangler_observability`
- Label: `Llama Wrangler Observability`
- Default index: `llama_wrangler`

## Required sourcetypes

- `llama_wrangler:request`
- `llama_wrangler:response`
- `llama_wrangler:node_health`
- `llama_wrangler:routing_decision`
- `llama_wrangler:consensus`
- `llama_wrangler:frontier_delta`
- `llama_wrangler:validation`
- `llama_wrangler:error`

## Required dashboards

1. Overview
   - total requests
   - success/error rate
   - average latency
   - frontier calls
   - local vs frontier
   - requests by execution mode

2. Node Health
   - latest status by node
   - memory availability
   - active jobs
   - model inventory
   - health over time

3. Model Performance
   - latency by node/model
   - tokens/sec by node/model
   - p95 latency
   - failures by model

4. Routing Decisions
   - selected nodes
   - fallback usage
   - routing reasons
   - model aliases

5. Consensus and Frontier Delta
   - participant count
   - agreement score
   - validation pass/fail
   - escalation reasons
   - estimated frontier cost

## Deliverables

- Valid Splunk app structure
- props.conf
- macros.conf
- eventtypes.conf
- savedsearches.conf where useful
- dashboard XML files
- README explaining HEC setup
- sample SPL searches
