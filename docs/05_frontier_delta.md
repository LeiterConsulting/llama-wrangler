# Frontier Delta

Frontier Delta is an optional hybrid mode where local models handle the majority of work and external frontier models receive only unresolved deltas.

## Principle

Local-first, not local-only.

The system should avoid sending full repositories, documents, prompts, secrets, or customer data to external providers by default.

## Escalation triggers

- Local model disagreement
- Output validation failure
- No majority consensus
- Test failure
- Schema failure
- High-risk task
- User requested best-possible mode
- Local context limit exceeded
- Local endpoints unavailable

## Delta payload contents

A frontier payload should include only:

- The unresolved question
- Candidate local outputs
- Disagreement summary
- Relevant snippets
- Test results or validation failures
- Required decision
- Output format constraints

## Policy controls

- local_only hard mode
- per-client provider permission
- per-project provider permission
- source code allowed/disallowed
- summaries-only mode
- secrets detection
- redaction
- max per-request cost
- max daily cost
- approval required for sensitive data

## Example policy

```yaml
frontier:
  enabled: true
  default_mode: frontier_delta
  providers:
    - openai
    - anthropic
  require_approval_for:
    - source_code
    - secrets_detected
    - customer_data
  redaction:
    enabled: true
    detect_secrets: true
    strip_env_values: true
  budget:
    daily_limit_usd: 5.00
    per_request_limit_usd: 0.50
```
