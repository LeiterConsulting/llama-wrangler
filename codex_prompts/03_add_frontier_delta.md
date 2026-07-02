# Codex Prompt: Add Frontier Delta

Add optional Frontier Delta escalation to Llama Wrangler.

## Goal

When local consensus fails, validation fails, or policy requests external review, build a minimized payload and call a configured frontier provider.

## Requirements

1. Add frontier provider abstraction.
2. Implement at least one provider as a generic OpenAI-compatible HTTP client.
3. Add policy engine:
   - enabled/disabled
   - local_only hard block
   - require approval flags
   - max per-request cost
   - max daily cost
   - source_code_allowed
   - summaries_only
4. Add redaction hooks:
   - detect API keys/secrets with regexes
   - strip `.env` values
   - detect likely private keys
5. Build delta payloads from:
   - candidate outputs
   - disagreement summary
   - validation results
   - minimal relevant snippets if allowed
6. Emit `llama_wrangler:frontier_delta` telemetry events.
7. Never send full raw request body by default.

## Constraints

- Frontier usage must be opt-in.
- If secrets are detected, block by default.
- Store only metadata in Splunk by default.

## Deliverables

- Provider interface
- OpenAI-compatible provider implementation
- Policy tests
- Redaction tests
- Frontier event tests
