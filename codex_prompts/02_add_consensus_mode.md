# Codex Prompt: Add Consensus Mode

Extend Llama Wrangler with local consensus execution.

## Goal

Allow the marshal to fan out a request to multiple eligible subscribers, collect responses, compare them, and return the best result when sufficient agreement exists.

## Required execution modes

- `single`
- `race`
- `consensus`
- `consensus_delta` with frontier escalation stubbed but disabled

## Requirements

1. Add execution mode selection to model aliases and request metadata.
2. Implement participant selection:
   - min_participants
   - max_participants
   - capability match
   - health check required
   - model availability required
3. Implement response collection with timeout handling.
4. Add simple comparison strategies:
   - exact normalized match
   - JSON structural match
   - regex/test-case validator hook
   - local evaluator stub
5. Emit consensus events using the schema in `docs/07_event_schema.md`.
6. Return consensus metadata in debug mode only.
7. If no consensus is reached, return best candidate with warning metadata unless frontier escalation is enabled later.

## Constraints

- Do not call frontier models yet.
- Keep all prompt/response content out of Splunk telemetry unless explicitly configured.
- Consensus logic should be pluggable.

## Deliverables

- Consensus engine package/module
- Participant selection tests
- Agreement scoring tests
- Updated config examples
- Updated README
