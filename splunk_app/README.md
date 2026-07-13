# Llama Wrangler Observability Splunk App

## Setup

1. Install this app folder into `$SPLUNK_HOME/etc/apps/llama_wrangler_observability`.
2. Restart Splunk or reload apps.
3. Create/enable an HEC token.
4. Configure Llama Wrangler marshal telemetry with:

```yaml
telemetry:
  splunk_hec:
    enabled: true
    url: "https://splunk.localdomain:8088/services/collector"
    token_env: SPLUNK_HEC_TOKEN
    index: "llama_wrangler"
    source: "llama-wrangler"
    verify_ssl: true
```

Disable `verify_ssl` only for a trusted self-signed lab deployment. The Llama Wrangler UI shows an explicit warning while certificate verification is disabled.

## Recommended index

`llama_wrangler`

## Recommended sourcetypes

See `docs/07_event_schema.md` in the project package.

## Dashboards

- **Llama Wrangler Overview**: requests, latency, execution modes, node health, consensus participant failures, peak queue depth, and streaming outcome counts.
- **Llama Wrangler Operations**: consensus failure reasons/outcomes, queue scheduling, streaming retries/partials/cancellations, benchmark scheduler and runner history, routing policy exclusions, and model lifecycle action history.

The packaged navigation exposes both dashboards. Operational panels use metadata-only fields emitted by Llama Wrangler; they do not inspect inference content, raw HTTP material, credentials, local fixtures, or arbitrary payload data.

## Saved reports

Packaged summary/history searches are disabled by default. Operators can review and enable scheduling for consensus participant failures, queue scheduling, streaming outcomes, benchmark scheduler/runner history, routing policy exclusions, and model lifecycle actions according to local retention and search-head policy.

## Initial validation SPL

```spl
index=llama_wrangler | stats count by sourcetype
```

```spl
index=llama_wrangler sourcetype=llama_wrangler:response
| stats count avg(latency_ms) avg(tokens_per_second) by selected_node resolved_model
```

```spl
`llama_wrangler_consensus`
| stats sum(failed_count) as failed_participants
```

```spl
`llama_wrangler_streaming_outcomes`
| stats count by event_type reason retry_phase
```
