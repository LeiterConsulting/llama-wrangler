# Support Bundle Schema

`POST /wrangler/support-bundle/export` returns a sanitized diagnostic artifact for local troubleshooting and downstream support tooling. It is not a backup, restore, import, or state replication format.

## Schema Identity

Every support bundle includes `bundle_schema`:

```json
{
  "name": "llama-wrangler.support-bundle",
  "version": 1,
  "json_schema": "schemas/support_bundle.schema.json",
  "documentation": "docs/13_support_bundle_schema.md",
  "compatibility": "additive_backward_compatible"
}
```

`bundle_schema.version` is the version of the support-bundle export contract. It is separate from:

- top-level `version`, which is the Llama Wrangler service version that generated the bundle
- `service.schema_version`, which is the persisted app-state schema version
- `service.config_version`, which is the persisted runtime config version

## Compatibility Rules

Version 1 is additive-backward-compatible:

- Required top-level fields stay present for all version 1 bundles.
- Existing field meanings should not change within version 1.
- New fields may be added to any object.
- Support tooling should ignore unknown fields.
- Removing required fields, changing privacy guarantees, or changing existing field meanings requires a new `bundle_schema.version`.

The machine-readable schema lives at `schemas/support_bundle.schema.json`.

## Required Top-Level Fields

Version 1 bundles include:

- `generated_at`: bundle generation timestamp
- `version`: service version
- `bundle_schema`: support-bundle schema metadata
- `service`: role, node ID, setup status, app-state schema/config versions, migration history, timestamps, client-key counts, and sanitized enrollment queue
- `config`: sanitized runtime config
- `nodes`: sanitized node metadata, including additive Managed Node versus Passive Endpoint control/trust metadata when present
- `sessions`: sanitized session metadata
- `queue`: metadata-only queue snapshot, including scheduling policy and weights
- `audit`: metadata-only audit events
- `secret_storage`: secret-storage status metadata only
- `privacy`: flags and redaction field names for excluded data classes

## Privacy Contract

Version 1 support bundles must report:

```json
{
  "privacy": {
    "prompt_bodies_included": false,
    "response_bodies_included": false,
    "secrets_included": false
  }
}
```

Support bundles must not include admin tokens, client API keys, Splunk HEC tokens, provider keys, raw enrollment tokens, enrollment token hashes, raw heartbeat credentials, rotated heartbeat credentials, raw headers, prompt bodies, response bodies, raw request bodies, payload fields, or other secrets. Enrollment queue entries may include safe token hints, expiry, registration timestamps, node ID, subscriber URL, control level, trust level, capability source, and approval state.

Node metadata may include `control_level`, `trust_level`, `capability_source`, `approval_state`, source fields, heartbeat-required/freshness state, safe heartbeat auth method/hint/provisioning metadata, benchmark job status, benchmark result summaries, benchmark workload suite IDs/task IDs/result metric names, local fixture manifest IDs or basename hints, model lifecycle action summaries, model lifecycle action policy reason codes, and freshness timestamps. Benchmark job/result summaries may include benchmark IDs, job status, model names, timing, token-count, rate, source, suite ID, task count, runner mode, and error-code metadata only. Model lifecycle action summaries may include action ID, action type, model name, desired keep-warm state, status, queue/claim/update/completion/failure timestamps, policy, endpoint names, and normalized safe error code only. Model lifecycle action policy metadata may include eligible/blocked counts, supported action IDs, control level, trust level, approval state, source, mode, model count, action count, pending action count, last action status, and reason codes only. Config metadata may include benchmark runner enablement, dry-run mode, metrics-only result policy, poll interval, and max jobs per tick. These fields are diagnostic metadata only. They must not include heartbeat credentials, enrollment tokens, API keys, raw headers, prompt bodies, response bodies, fixture contents, full local fixture paths, request bodies, or payloads.

Sanitized audit events may include consensus participant IDs/counts, required/maximum participant bounds, successful/failed counts, agreement count/score, comparison strategy ID, winner node, disagreement/reached/timeout/cancellation flags, duration, disabled-frontier escalation recommendation, and fixed participant failure summaries. A participant failure summary may include only node ID, one of `missing_proxy_url`, `connection_error`, `upstream_4xx`, `upstream_5xx`, `body_read_failure`, `response_size_limit`, `timeout`, or `cancellation`, optional numeric upstream status, and duration. Consensus prompt bodies, response bodies, extracted answer content, arbitrary error text, upstream URLs, comparison signatures, validator/evaluator input, raw headers, and credentials are forbidden.

Downstream tools should treat support bundles as diagnostic metadata, but they should still avoid redistributing bundles without user approval because hostnames, model names, node metadata, timestamps, and operational state can be sensitive in some environments.

Support bundles are not backup or restore artifacts. The `secret_storage` object may include backup guidance such as `backup_required_files`, `backup_description`, `restore_description`, and `backup_warnings`, including required file names such as `secrets.enc.json` and `secrets.key`. It must not include the contents of `secrets.enc.json`, the contents of `secrets.key`, the `LLAMA_WRANGLER_SECRETS_KEY` value, or any other secret material. For actual encrypted fallback backup and restore behavior, use `docs/11_configuration_storage.md`.

## Consumer Guidance

Downstream support tooling should:

- read `bundle_schema.name` and `bundle_schema.version` before parsing
- accept version `1` when required top-level fields are present
- tolerate extra fields in any object
- use `service.schema_version` and `service.config_version` only as metadata about the app-state/config that produced the export
- treat `secret_storage` as status metadata, not as a source of secret values
- treat any backup/restore fields in `secret_storage` as operator guidance only
- reject or quarantine bundles that claim `privacy.secrets_included`, `privacy.prompt_bodies_included`, or `privacy.response_bodies_included` is true

When a future bundle has a higher `bundle_schema.version`, tooling should either use a compatible parser for that version or fall back to a conservative summary using only common metadata.
