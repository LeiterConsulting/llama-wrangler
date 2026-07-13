# Architecture

## Components

### Marshal

The marshal is the control plane and API gateway. It receives all client requests, applies policy, selects execution mode, chooses participants, proxies requests, evaluates responses, and emits telemetry.

Responsibilities:

- OpenAI-compatible API shim
- Ollama-compatible API shim
- Authentication and client policy
- Node registry
- Capability inventory
- Routing decisions
- Parallel execution orchestration
- Consensus scoring
- Frontier Delta escalation
- HEC/log output

### Subscriber

A subscriber runs on each worker endpoint. It reports local capabilities and proxies requests to the local Ollama service.

Responsibilities:

- Register with marshal
- Report host details
- Report Ollama version and model inventory
- Report health, load, memory, and optional GPU stats
- Execute local Ollama requests
- Stream responses back to marshal
- Emit local metrics

### Managed Nodes and Passive Endpoints

Llama Wrangler distinguishes between full-control Managed Nodes and limited-control Passive Endpoints.

- Managed Nodes run the Llama Wrangler subscriber beside Ollama. They can report hardware, model state, warm-model state, load, health, benchmarks, and richer metadata-only telemetry.
- Passive Endpoints are existing Ollama-compatible URLs known only to the marshal. The marshal can route requests and collect marshal-observed telemetry, but it cannot fully inspect or control the asset.

Routing, consensus, benchmarks, safety policy, and UI badges must account for both control level and trust level. See `docs/16_node_control_modes.md`.

### Future Capability Endpoints

Future versions may generalize Managed Nodes and Passive Endpoints into broader Capability Endpoints for additional runtimes, providers, agent surfaces, build runners, or automation systems. This is a V2 direction, not current MVP scope.

The V1 architecture should keep policy, routing, telemetry, and support-bundle metadata adaptable by using generic control level, trust level, capability source, capability, and freshness fields where they naturally fit. It should not add placeholder UI or unfinished non-Ollama integrations before the Ollama fleet control plane is functional. See `docs/19_capability_endpoint_future_plan.md`.

### Local Ollama runtime

Each subscriber expects a local Ollama service, usually at `http://localhost:11434`.

### Splunk App

The Splunk app provides dashboards, sourcetypes, field extractions, eventtypes, macros, and operational views for Llama Wrangler telemetry.

## Logical topology

```text
Client / IDE / Agent
  -> Llama Wrangler Marshal
      -> Subscriber: M4 Mac mini / optional local Ollama
      -> Subscriber: M3 Pro / MLX-friendly models
      -> Subscriber: M1 Max / memory-rich Apple Silicon worker
      -> Subscriber: RTX 4090 PC / primary heavy coding worker
      -> Optional frontier provider for delta operations
```

## API compatibility

Expose at minimum:

- `/v1/chat/completions`
- `/v1/completions`
- `/v1/embeddings`
- `/api/chat`
- `/api/generate`
- `/api/tags`
- `/healthz`
- `/wrangler/nodes`
- `/wrangler/models`
- `/wrangler/metrics`

## Binary modes

```bash
llama-wrangler marshal --config ./marshal.yaml
llama-wrangler subscriber --config ./subscriber.yaml
llama-wrangler standalone --config ./standalone.yaml
```

## Execution flow

1. Client sends request to marshal.
2. Marshal authenticates request.
3. Marshal resolves model alias.
4. Marshal selects execution mode.
5. Marshal identifies eligible nodes.
6. Marshal executes single route, race, consensus, or consensus-delta workflow.
7. Marshal validates and scores outputs.
8. Optional frontier delta escalation occurs if policy permits.
9. Final response is returned.
10. Structured events are emitted to JSON logs and Splunk HEC.

### V1 non-streaming consensus

- consensus aliases default to two required and four maximum participants when bounds are omitted; maximum fan-out is capped at eight
- routing applies approval, enabled/health, model availability, Managed Node control level, `local`/`lan_trusted` trust, and heartbeat freshness before fan-out
- Passive Endpoints, `lan_unverified`, `external`, pending/revoked, disabled/unhealthy, stale/missing-heartbeat, and model-ineligible nodes do not participate
- marshal concurrently collects bounded non-streaming responses under the request cancellation context and routing timeout
- each participant response is capped at 8 MiB in request memory and is never persisted
- normalized text and JSON structural comparison are implemented through a pluggable local consensus engine; regex validator and local evaluator hooks are available
- strict majority agreement wins; no-majority ties return the earliest routing-ranked successful participant and emit disagreement metadata
- the winning OpenAI/Ollama response body and content type are returned unchanged; consensus metadata appears in response headers only when safe debug mode is explicitly requested
- participant failures use a fixed metadata-only vocabulary: `missing_proxy_url`, `connection_error`, `upstream_4xx`, `upstream_5xx`, `body_read_failure`, `response_size_limit`, `timeout`, and `cancellation`
- participant failure projections contain only node ID, reason code, optional numeric upstream status, and duration; raw errors, URLs, headers, and response content are discarded from telemetry and management surfaces
- partial participant failure does not prevent deterministic aggregation when the configured required-success bound is met
- unmet quorum returns the requesting API family's compatibility error shape; all-participant upstream 4xx failures retain normalized upstream status semantics, timeout-driven failures return HTTP 504, and other unmet-quorum failures return HTTP 502
- V1 streaming consensus is deliberately unsupported and rejected before fan-out; single-route SSE/JSONL behavior remains unchanged
- enabling streaming consensus later requires a separate protocol for bounded per-participant stream collection, quorum timing, deterministic aggregation, backpressure, cancellation, response commitment, and no-retry-after-partial-output guarantees; the non-streaming engine is not evidence that those semantics are safe
- `consensus_delta` may recommend escalation in metadata, but Frontier execution remains disabled

## Streaming and Retry Semantics

Marshal retries upstream proxy attempts only before any response bytes are written to the client.

- upstream connection errors, 5xx responses, and body-read failures before the first streamed token may use the next eligible fallback node
- after partial output has been written to the client, retries are disabled for that request
- non-streaming upstream responses are buffered before the client response is committed, so read failures can still fall back before client-visible output
- client cancellation emits cancellation telemetry and does not trigger fallback
- retry, cancellation, and partial-output telemetry must remain metadata-only
- non-streaming consensus cancellation cancels all in-flight participants; insufficient successful participants return a compatibility-shaped error without committing a candidate response

Client compatibility checks exercise the public HTTP routes with real HTTP readers:

- OpenAI-compatible `/v1/chat/completions` streaming preserves `text/event-stream` SSE framing and flushes `data: ...` lines before upstream completion
- Ollama-compatible `/api/chat` streaming preserves newline-delimited JSON framing and flushes each JSON object line before upstream completion
- OpenAI-compatible and Ollama-compatible non-streaming consensus checks use real marshal/participant HTTP listeners and `net/http` clients to verify partial-success winners, deterministic response shape, normalized upstream 4xx errors, HTTP 502/504 quorum errors, streaming rejection before fan-out, and payload exclusion from metadata surfaces

## Queue Visibility

Marshal keeps a lightweight runtime queue snapshot for operational visibility:

- active and waiting counts
- available capacity
- current entries
- recent completed, failed, cancelled, rejected, or partial entries
- request priority metadata
- scheduling policy and high/normal/low dispatch weights

Queue visibility is intentionally metadata-only. It can include request ID, model name, API surface, priority, status, stream flag, session ID, timestamps, and depth/capacity values. It must not include prompt bodies, response bodies, raw headers, tokens, or secrets.

Priority metadata is accepted from `X-Llama-Wrangler-Priority`, `priority`, or `queue_priority`. The default scheduling policy is `weighted_priority`, which uses configurable high/normal/low weights to choose among waiting requests when active capacity frees up. A `fifo` policy is also available for arrival-order dispatch.

The operations UI also summarizes recent metadata-only audit events into `operation_stats` for retry, partial-response, and cancellation counts. These counters are derived from `upstream_retry`, `response_partial`, and `request_cancelled` events and intentionally exclude prompt bodies, response bodies, raw headers, API keys, tokens, and other payload-like fields.

## Error Compatibility

Marshal normalizes inference error responses by endpoint family:

- OpenAI-compatible `/v1/*` endpoints return an `error` object with `message`, `type`, `param`, and `code`
- Ollama-compatible `/api/*` endpoints keep `error` as a string and add safe `type` and `code` metadata
- UI and management endpoints keep simple JSON errors that are easier for the embedded UI to consume

Client-facing inference errors must be sanitized. They should use stable codes and friendly messages, not raw upstream transport errors, prompt or response content, authorization headers, API keys, HEC tokens, upstream URLs, or other secrets.

## Auth and Network Exposure Hardening

Marshal binds to localhost by default. Runtime UI bootstrap metadata derives a LAN exposure posture from the configured listen address. Loopback addresses are treated as safe defaults; all-interface, non-loopback IP, and non-local hostname listens produce an explicit warning for Settings and support workflows.

Management admin-token checks and inference client API-key checks include lightweight in-memory failure rate limiting keyed by remote address and auth scope. Valid credentials can still pass through after failed attempts, but repeated invalid presented tokens return `429` with `Retry-After`. Client-facing inference rate-limit responses continue to use OpenAI-compatible or Ollama-compatible error shapes.

Splunk HEC TLS posture is also surfaced as metadata. If certificate verification is disabled for trusted self-signed lab compatibility, bootstrap and telemetry status expose `tls_verification_disabled` plus a warning string so the Splunk UI can show an explicit risk state without exposing HEC tokens or payloads.

## Secret Storage Rekey

The encrypted local fallback secret store supports an explicit rekey workflow when the key source is the local `secrets.key` file. Rekey generates new local key material and rewrites `secrets.enc.json` while keeping secret values out of ordinary app state, telemetry, support bundles, and API responses. If the key source is external, such as `LLAMA_WRANGLER_SECRETS_KEY`, local rekey is unavailable and the external key owner remains responsible for rotation.

Encrypted fallback backup and restore is a file-pair contract, not a support-bundle workflow. With a local key source, `secrets.enc.json` and `secrets.key` must be backed up and restored together. With an external key source, `secrets.enc.json` must be restored with the same `LLAMA_WRANGLER_SECRETS_KEY` value supplied by the operator's secret manager. Settings and support metadata can describe these requirements, but they must never include plaintext secrets, key values, or secret file contents.

OS keychain integration is implemented as a minimal additive, opt-in backend spike behind the existing secret-store API. `LLAMA_WRANGLER_SECRET_BACKEND=os_keychain` enables the OS backend where practical, keeps encrypted fallback available, migrates values non-destructively when the keychain backend is usable, and exposes only metadata such as active backend, fallback availability, migrated count, platform, runtime context, and service-mode warning state. Encrypted fallback remains the default until service-user keychain behavior is verified across target install modes.

## Support Bundle

Marshal exposes a sanitized support-bundle export for local troubleshooting. It is a diagnostic artifact, not a backup format.

The bundle can include schema and config version metadata, migration history, sanitized config, node metadata, session metadata, queue metadata, metadata-only audit events, and secret-storage status. It must exclude admin tokens, client API keys, HEC/provider tokens, enrollment token hashes, raw headers, prompt bodies, response bodies, request bodies, payload fields, and other secrets.

Support bundles are self-describing through `bundle_schema` metadata. Version 1 uses `llama-wrangler.support-bundle`, points downstream tooling at `schemas/support_bundle.schema.json` and `docs/13_support_bundle_schema.md`, and follows an additive-backward-compatible policy. The support-bundle schema version is separate from the service version, app-state schema version, and config version.
