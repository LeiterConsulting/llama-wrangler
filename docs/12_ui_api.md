# UI API

The local UI is served at `/ui` and uses JSON management endpoints.

## Setup

- `GET /wrangler/ui/bootstrap`
- `GET /wrangler/ui/status`
- `POST /wrangler/setup/start`
- `POST /wrangler/setup/scan-local`
- `POST /wrangler/setup/detect-ollama`
- `POST /wrangler/setup/discover-peers`
- `POST /wrangler/setup/apply-recommended`
- `POST /wrangler/setup/test-ollama`
- `POST /wrangler/setup/test-hec`
- `POST /wrangler/setup/complete`

## Configuration

- `GET /wrangler/config`
- `PUT /wrangler/config`
- `POST /wrangler/config/export`
- `POST /wrangler/support-bundle/export`

## Fleet

- `GET /wrangler/nodes`
- `POST /wrangler/nodes/manual-add`
- `POST /wrangler/nodes/:id/benchmark`
- `POST /wrangler/nodes/:id/enable`
- `POST /wrangler/nodes/:id/disable`
- `POST /wrangler/nodes/:id/overrides`
- `GET /wrangler/models`
- `GET /wrangler/aliases`
- `PUT /wrangler/aliases`

## Operations

- `GET /wrangler/routing/policies`
- `PUT /wrangler/routing/policies`
- `GET /wrangler/client-presets`
- `GET /wrangler/auth/status`
- `POST /wrangler/auth/admin-token/rotate`
- `POST /wrangler/auth/api-keys`
- `POST /wrangler/auth/api-keys/:id/rotate`
- `POST /wrangler/auth/api-keys/:id/revoke`
- `POST /wrangler/secrets/rekey`
- `GET /wrangler/telemetry/status`
- `PUT /wrangler/telemetry/splunk-hec`
- `POST /wrangler/telemetry/test-hec`
- `GET /wrangler/audit/recent`
- `GET /wrangler/metrics`

## Authentication

Before setup completion, local management endpoints remain open to keep first-run setup smooth on localhost.

After setup completion:

- management endpoints require `Authorization: Bearer <admin-token>`
- inference endpoints require `Authorization: Bearer <client-api-key>` when client auth is enabled
- generated tokens are shown once and only hints are stored in ordinary app state
- admin tokens can be rotated from Settings
- client API keys can be generated, regenerated, or revoked from IDE setup
- repeated invalid admin or client API-key attempts are rate limited per remote address
- rate-limited responses return `429` with `Retry-After`

Management rate-limit responses use UI-friendly JSON such as `{"error":"admin_auth_rate_limited","message":"..."}`. Inference rate-limit responses preserve API-family compatibility: `/v1/*` returns OpenAI-style `rate_limit_error` objects and `/api/*` returns Ollama-style string `error` plus safe `type` and `code` metadata.

## Network Exposure

`GET /wrangler/ui/bootstrap` includes `safe_defaults` fields for LAN posture:

- `marshal_listen`
- `lan_access_by_default`
- `lan_access_enabled`
- `lan_requires_explicit_enablement`
- `lan_access_warning`

Localhost and loopback listen addresses are treated as the safe default. Any all-interface, non-loopback IP, or non-local hostname listen address is surfaced as LAN exposure so Settings can show an explicit warning.

## Splunk TLS Warning

`GET /wrangler/ui/bootstrap` and `GET /wrangler/telemetry/status` include Splunk HEC TLS posture metadata under `telemetry.splunk_hec` / `splunk_hec`:

- `verify_ssl`
- `tls_verification_disabled`
- `tls_warning`
- `has_token`

When `verify_ssl` is `false`, the Splunk UI must show an explicit warning that certificate verification is disabled and should only be used for trusted self-signed Splunk lab certificates. The warning metadata must not include HEC token values, admin tokens, client API keys, raw headers, prompts, responses, request bodies, or payloads.

## Client Presets

`GET /wrangler/ui/bootstrap` includes `client_presets`, and `GET /wrangler/client-presets` returns the same preset list for the current request host.

Preset cards cover:

- Cline
- Continue
- Open WebUI
- generic OpenAI SDK clients

Preset metadata includes the derived OpenAI-compatible base URL, selected model alias, display fields, and copyable snippet bodies. API keys are represented by `<client-api-key>` placeholders in API responses. The UI may substitute the browser-local one-time generated key for display/copy, but stored client API keys, admin tokens, HEC tokens, prompts, responses, request bodies, and headers must not be returned by preset APIs.

## Node Control Metadata

`GET /wrangler/ui/bootstrap`, `GET /wrangler/nodes`, and support bundles include node control metadata for Phase B planning and UI badges.

Node records can include:

- `control_level`: `managed` or `passive`
- `trust_level`: `local`, `lan_trusted`, `lan_unverified`, or `external`
- `capability_source`: `subscriber_reported`, `marshal_observed`, or `manual`
- `approval_state`: `pending`, `approved`, `rejected`, or `revoked`
- `health_source`
- `model_inventory_source`
- `benchmark_source`
- `warm_state_supported`
- `management_supported`
- `telemetry_level`
- `last_observed_at`
- `last_reported_at`

These fields are metadata only. They must not contain enrollment tokens, API keys, HEC tokens, future provider keys, raw headers, prompt bodies, response bodies, request bodies, or payloads.

Current behavior is intentionally conservative: existing local and manually added subscriber records migrate as Managed Node records. Passive Endpoint records are schema-supported and surfaced with limited-control metadata, but the dedicated Add existing Ollama endpoint flow is a later Phase B slice.

## Inference Proxy Semantics

For OpenAI-compatible and Ollama-compatible inference endpoints, marshal retries fallback nodes only before client-visible output begins.

- streaming requests may retry after upstream connection failures, 5xx responses, or body-read failures before the first token
- streaming requests must not retry after any partial output has been written to the client
- non-streaming responses are buffered before commit so upstream read failures can still retry safely
- cancellation, retry, and partial-output events are emitted as metadata-only telemetry
- OpenAI-compatible streaming preserves upstream `text/event-stream` SSE chunks so SDK-style clients can read `data: ...` lines before upstream completion
- Ollama-compatible streaming preserves upstream newline-delimited JSON chunks so Ollama-style clients can read one JSON object per line before upstream completion

## Queue Metadata

`GET /wrangler/ui/bootstrap` and `GET /wrangler/metrics` include queue metadata for UI visibility:

- `max_depth`
- `active`
- `waiting`
- `available`
- `current`
- `recent`
- `priorities`
- `scheduling`

Queue entries may include request ID, priority, status, API surface, requested model, session ID, stream flag, timestamps, and capacity/depth metadata. Queue entries must not include prompt text, response text, request bodies, headers, API keys, or other secrets.

Inference clients can set `X-Llama-Wrangler-Priority: high|normal|low`. If no header is present, JSON request fields `priority` or `queue_priority` may provide the same metadata.

Queue dispatch can use:

- `weighted_priority`, the default, which dispatches waiting requests by configured high/normal/low weights
- `fifo`, which dispatches waiting requests in arrival order

Dashboard queue scheduling controls write to `PUT /wrangler/routing/policies` using `queue_scheduling_policy` and `queue_priority_weights`. Priority, scheduling policy, and weights are metadata-only; they must not include payloads, prompts, responses, headers, or secrets.

## Operation Stats

`GET /wrangler/ui/bootstrap` and `GET /wrangler/metrics` include `operation_stats` derived from recent metadata-only audit events:

- `window`
- `audit_events`
- `retries.total`
- `retries.before_first_token`
- `retries.last_at`
- `partials.total`
- `partials.after_partial`
- `partials.last_at`
- `cancellations.total`
- `cancellations.before_first_token`
- `cancellations.after_partial_output`
- `cancellations.before_queue`
- `cancellations.last_at`

These counters summarize `upstream_retry`, `response_partial`, and `request_cancelled` events for the operations UI. They must not include prompt text, response text, request bodies, raw headers, API keys, HEC tokens, provider keys, or other secrets.

## Error Compatibility

Inference endpoint errors are normalized by API family:

- `/v1/*` returns OpenAI-style errors: `{"error":{"message","type","param","code"}}`
- `/api/*` returns Ollama-style string errors with safe metadata: `{"error":"...","type":"...","code":"..."}`
- management/UI endpoints keep the existing UI-friendly JSON error maps

Error responses must not echo prompt bodies, response bodies, raw headers, API keys, HEC tokens, upstream URLs, or other secrets. Upstream transport failures are reported with generic client-facing messages and metadata-safe error codes.

## Support Bundle

`POST /wrangler/support-bundle/export` returns a sanitized JSON troubleshooting bundle for local support workflows.

Each bundle includes versioned support-bundle schema metadata:

- `bundle_schema.name`: `llama-wrangler.support-bundle`
- `bundle_schema.version`: support-bundle contract version
- `bundle_schema.json_schema`: `schemas/support_bundle.schema.json`
- `bundle_schema.documentation`: `docs/13_support_bundle_schema.md`
- `bundle_schema.compatibility`: `additive_backward_compatible`

The bundle can include:

- generated timestamp and service version
- support-bundle schema metadata
- role, node ID, setup status, schema version, config version, and migration history
- sanitized runtime config
- node and session metadata
- queue snapshot
- metadata-only audit events
- secret-storage status metadata
- privacy flags describing excluded data classes

The bundle must not include admin tokens, client API keys, HEC tokens, provider keys, enrollment token hashes, prompt bodies, response bodies, raw request bodies, raw headers, payload fields, or other secrets.

Downstream tooling should parse `bundle_schema` first. `bundle_schema.version` is distinct from the top-level service `version`, `service.schema_version`, and `service.config_version`.

## State Metadata

`GET /wrangler/ui/bootstrap` returns:

- `schema_version`
- `config_version`
- `migration_history`
- `secret_storage`

Settings displays the active schema and config versions so persisted app-data upgrades are visible during support and troubleshooting.

`secret_storage` is status metadata only. It can include the storage backend, encrypted status, legacy migration status, and key source, but it must not include admin tokens, client API keys, HEC tokens, provider keys, or secret payloads.

Secret-storage status can include backup/restore guidance metadata:

- `fallback_backend`
- `fallback_available`
- `backup_required_files`
- `backup_description`
- `restore_description`
- `backup_warnings`
- `os_keychain_status`
- `os_keychain_plan`
- `os_keychain_next_step`
- `os_keychain_migrated`
- `os_keychain_platform`
- `os_keychain_runtime`
- `os_keychain_service_mode`
- `os_keychain_warning`

For `key_source: file`, `backup_required_files` names `secrets.enc.json` and `secrets.key`. For `key_source: env`, it names `secrets.enc.json` and `LLAMA_WRANGLER_SECRETS_KEY`. These are instructional names only; responses must not include plaintext secret values, local absolute paths, admin tokens, client API keys, HEC tokens, provider keys, raw headers, prompts, responses, request bodies, or payloads.

The OS keychain fields are status metadata only. When `LLAMA_WRANGLER_SECRET_BACKEND=os_keychain` is set and the backend is usable, `secret_storage.backend` can report `os_keychain` while `fallback_available` remains true. If the keychain backend is unavailable, `secret_storage.backend` reports `encrypted_file` and `os_keychain_status` reports `unavailable`. Service-like runtime warnings are advisory metadata so operators know keychain access may differ under launchd, systemd, Windows service users, or future install helpers. These fields must never include keychain item contents or encrypted fallback file contents.

`POST /wrangler/secrets/rekey` rotates the local encrypted fallback key when `secret_storage.rekey_supported` is true. The endpoint returns metadata only:

- `rekeyed`
- `rotated_at`
- `status.backend`
- `status.encrypted`
- `status.key_source`
- `status.rekey_supported`

If the key source is external, such as `LLAMA_WRANGLER_SECRETS_KEY`, the endpoint returns `409 secret_rekey_unsupported` and does not modify local secret files. Rekey responses must not include admin tokens, client API keys, HEC tokens, provider keys, or secret payloads.
