# V1 Acceptance and Security Matrix

## Purpose

This matrix is the V1 release-gate index for the Ollama-first Llama Wrangler control plane. It consolidates automated evidence without replacing the focused tests that own each behavior.

Run the local automated harness from the repository root:

```bash
./scripts/v1_acceptance.sh
```

The harness builds into a temporary directory, uses disposable `HOME` and `XDG_CONFIG_HOME` values, listens only on a temporary loopback port, performs a setup/auth/enrollment/restart/privacy flow, and removes its temporary state on exit. It does not install, start, stop, or modify a real OS service. Set `LLAMA_WRANGLER_ACCEPTANCE_PORT` only when a fixed disposable port is required. `LLAMA_WRANGLER_ACCEPTANCE_SKIP_TESTS=1` skips the grouped/full Go suites and runs only static plus disposable lifecycle checks; this is a development shortcut, not complete release evidence.

On macOS, review the disposable user-level launchd package plan without registering a job:

```bash
./scripts/macos_user_launchd_acceptance.sh
```

The default path builds a temporary initial package, renders an absolute-path plist with encrypted fallback, runs `plutil -lint`, and deletes the package. It does not call `launchctl`. The current-user lifecycle check is deliberately opt-in:

```bash
LLAMA_WRANGLER_MACOS_LAUNCHD_ACCEPTANCE=1 ./scripts/macos_user_launchd_acceptance.sh
```

The opt-in path uses a unique label in `gui/<uid>`, a temporary home/package/log/config root, a loopback-only port, and unconditional bootout/removal cleanup. It validates install/start, schema/auth/secret persistence across restart, atomic binary replacement plus restart, service-log privacy, and uninstall. It never writes the operator's real `~/Library/LaunchAgents`, never uses a system LaunchDaemon, and never enables the OS keychain.

## Automated Matrix

| ID | Domain | Acceptance requirement | Evidence | Status |
| --- | --- | --- | --- | --- |
| V1-LIFE-001 | Service lifecycle | Build and start standalone on loopback with isolated app data; `/healthz` and `/ui/` respond. | Harness A13-A16 | Automated |
| V1-LIFE-002 | Service lifecycle | Stop and restart against the same disposable state without losing setup/auth/node metadata. | Harness A21-A22 | Automated |
| V1-SAFE-001 | Safe defaults | Local-only, Frontier disabled, LAN disabled, metadata-only telemetry, body logging disabled, HEC disabled/TLS verification enabled. | `TestDefaultSafePosture`; Harness A15 | Automated |
| V1-STATE-001 | State schema | New/legacy state reaches schema version 2, migration history is preserved, future versions are rejected, config versions increment. | `internal/appstate` migration tests; Harness A15/A22 | Automated |
| V1-AUTH-001 | Setup/admin auth | Setup returns one-time admin material; unauthenticated management fails; valid admin auth survives restart; rotation/rate limits work. | `internal/httpapi/auth_test.go`; Harness A17/A22 | Automated |
| V1-AUTH-002 | Client auth | Missing/invalid client credentials use compatibility errors; valid keys work; revocation/rotation/rate limits work and persistence survives restart. | Auth/error compatibility tests; Harness A17/A22 | Automated |
| V1-SEC-001 | Secret storage | Encrypted fallback is the default service path; files are `0600`; plaintext values and legacy plaintext storage are absent; rekey preserves credentials. | `internal/secrets`; Harness A19 | Automated |
| V1-NODE-001 | Managed Node enrollment | One-time token registration creates a pending Managed Node, heartbeat shared-secret metadata is safe, approval gates routing, credentials are not echoed. | Node metadata tests; Harness A18 | Automated |
| V1-NODE-002 | Managed Node identity/freshness | Stored heartbeat credential is required where provisioned; rotation invalidates old credentials; missing/stale required heartbeats exclude routing. | Node metadata and routing tests | Automated |
| V1-PASS-001 | Passive Endpoint boundary | URL and `/api/tags` validation are safe; endpoint remains marshal-observed, approval-gated, probe-only, and without local-control claims. | Passive endpoint, benchmark, lifecycle, routing tests | Automated |
| V1-ROUTE-001 | Routing/session policy | Approval, enabled/health, control, trust, model, heartbeat freshness, benchmark placement, queue policy, and affinity exclusions are enforced. | `internal/routing`, `internal/session`, queue/routing policy tests | Automated |
| V1-CONS-001 | Non-streaming consensus | Bounded approved/fresh/trusted Managed Node fan-out, deterministic aggregation, fixed failure reasons, compatibility errors, timeout/cancellation, and content exclusion. | Consensus engine/HTTP/real-client tests | Automated |
| V1-CONS-002 | Streaming posture | Consensus streaming is rejected before fan-out; single-route SSE/JSONL and no-retry-after-partial behavior remain intact. | Consensus and streaming compatibility tests | Automated |
| V1-BENCH-001 | Benchmark control | Managed-only job orchestration, bounded scheduler retry/timeout/history, opt-in local runner, metrics-only reporting, Passive probe boundary, fixture-content exclusion. | Benchmark policy/result/scheduler/suite/runner tests | Automated |
| V1-MODEL-001 | Model lifecycle | Managed warm-state metadata and keep-warm action queue/claim/status/history are policy-gated; Passive Endpoints remain inventory-only. | Model lifecycle tests | Automated |
| V1-SPLUNK-001 | Splunk package | Overview/Operations XML, navigation, conf stanzas, event catalog, fixed consensus reasons, disabled reports, TLS behavior, and metadata-only field policy validate. | `internal/hec`; Harness A06/A10-A12 | Automated asset validation |
| V1-SUPPORT-001 | Support bundle | Bundle schema/version metadata and diagnostics are present while secrets, credentials, raw HTTP material, fixture data, and inference content are excluded. | Support-bundle tests; Harness A20 | Automated |
| V1-UI-001 | Embedded UI | UI assets parse and the disposable `/ui/` route renders; browser regression covers Dashboard/Nodes/Splunk/Settings surfaces. | Harness A09/A16 plus ledger browser evidence | Automated plus browser evidence |
| V1-V2-001 | Capability boundary | V1 remains Ollama-first; generic Capability Endpoint work stays documentation-only and current schemas/APIs remain additively extensible. | `docs/19_capability_endpoint_future_plan.md`; plan validation | Documented gate |
| V1-PKG-MAC-DRY-001 | macOS package plan | Default harness builds, renders, and lints a disposable absolute-path user LaunchAgent plan with encrypted fallback and no `launchctl` mutation. | macOS harness M01-M02; acceptance safety tests | Automated on macOS |

## Environment-Dependent Acceptance

These rows remain open and must not be inferred from the automated harness.

| ID | Domain | Required evidence | Status |
| --- | --- | --- | --- |
| V1-PKG-MAC-001 | macOS user launchd mechanics | Explicit opt-in disposable current-user install, start, restart, atomic binary upgrade, uninstall, log privacy, schema/auth persistence, and encrypted fallback behavior. | Validated 2026-07-10 on macOS 27.0 arm64; harness M01-M08 |
| V1-PKG-MAC-002 | macOS keychain | Interactive opt-in prompts and migration behavior in a packaged/notarized build; no service-keychain guarantee required for V1. | Pending packaging evidence |
| V1-PKG-MAC-003 | macOS release candidate | Signed/package-candidate installation into normal user paths, upgrade compatibility, uninstall preservation/removal choices, permissions, and operator-facing recovery. | Pending release candidate |
| V1-PKG-LINUX-001 | Linux service packaging | systemd install/start/restart/upgrade/uninstall and encrypted fallback permissions under the service account. | Pending |
| V1-PKG-WIN-001 | Windows service packaging | Service install/start/restart/upgrade/uninstall and encrypted fallback permissions under the service identity. | Pending |
| V1-SPLUNK-RUNTIME-001 | Splunk runtime | Install app `0.2.0` into a supported Splunk search head, ingest sanitized sample events, run every dashboard/search, and verify permissions/performance. | Installed per operator report 2026-07-13; Pending ingestion/dashboard/search/permissions/performance evidence |
| V1-SIGN-001 | Distribution | macOS signing/notarization and platform package integrity/update verification. | Pending |
| V1-CLIENT-001 | External clients | Supported Cline, Continue, Open WebUI, generic OpenAI SDK, and Ollama CLI smoke tests against a package candidate. | Pending release candidate |

## Release Decision Rule

Automated rows must pass from a clean checkout. Environment-dependent rows required for the intended release platform must have attached evidence before that platform is declared supported. Disposable launchd mechanics do not satisfy signed release-candidate, notarization, normal-user-path, upgrade-migration, or packaged-keychain rows. A failure, skipped gate, unavailable dependency, or untested environment remains open; it is not converted into a pass through documentation.

Acceptance artifacts must remain metadata-only. Do not attach app-data directories, secret files, credentials, raw HTTP captures, inference content, local fixture data, or unredacted service logs to release records.
