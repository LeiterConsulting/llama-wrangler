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
```

## Recommended index

`llama_wrangler`

## Recommended sourcetypes

See `docs/07_event_schema.md` in the project package.

## Initial validation SPL

```spl
index=llama_wrangler | stats count by sourcetype
```

```spl
index=llama_wrangler sourcetype=llama_wrangler:response
| stats count avg(latency_ms) avg(tokens_per_second) by selected_node resolved_model
```
