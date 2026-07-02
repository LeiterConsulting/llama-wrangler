# Phase A Closure Decision

Phase A is complete as of 2026-07-02.

This decision closes Phase A Foundation Hardening on a deliberately conservative credential boundary:

- encrypted fallback is the supported default and service credential path
- OS keychain remains interactive opt-in
- OS keychain service-user behavior moves to packaging and install hardening

## Evidence

Phase A closure is based on implementation plus validation evidence recorded in `docs/00_project_ledger.md`.

Completed hardening includes:

- versioned app state and lightweight migrations
- encrypted fallback secret storage separate from ordinary app state
- legacy plaintext secret migration and removal
- fallback rekey support for local file-key mode
- backup/restore guidance for encrypted fallback secrets
- metadata-only OS keychain status and opt-in backend spike
- launchd service-wrapper dry-run harness
- streaming retry, partial-output, and cancellation semantics
- queue priority metadata, weighted scheduling, and UI visibility
- normalized OpenAI and Ollama error shapes
- sanitized, versioned support-bundle export
- LAN exposure warnings and auth failure rate limiting
- client preset cards
- Splunk HEC TLS verification warning state

Credential validation evidence:

- macOS interactive keychain opt-in test passed with `LLAMA_WRANGLER_RUN_KEYCHAIN_TESTS=1`.
- Service-wrapper dry-run emits review-only launchd metadata, validation commands, and keychain check commands without secrets.
- Disposable launchd validation ran on `127.0.0.1:11437` with a temporary HOME and disposable keychain service namespace.
- `plutil -lint` passed for the generated plist.
- `launchctl bootstrap` loaded the generated LaunchAgent and `launchctl print` showed it running.
- Disposable interactive setup reported `backend: encrypted_file`, `fallback_available: true`, and `os_keychain_status: unavailable`.
- Disposable launchd runtime reported `backend: encrypted_file`, `fallback_available: true`, `os_keychain_status: unavailable`, `os_keychain_runtime: service_like`, and `os_keychain_service_mode: true`.
- `launchctl bootout` completed.

## Decision

The disposable launchd run did not prove service-mode keychain availability, but it did prove that the encrypted fallback path remains available in service-like runtime with warning metadata. That evidence supports completing Phase A by treating encrypted fallback as the service/default credential path.

OS keychain is still useful, but it is not a Phase A dependency for service installs:

- interactive users can opt in with `LLAMA_WRANGLER_SECRET_BACKEND=os_keychain`
- fallback remains available even when OS keychain is active or unavailable
- support bundles, bootstrap, auth status, and Settings return metadata only
- service-user keychain behavior is packaging hardening, not MVP foundation hardening

## Preserved Requirements

Phase A closure does not relax any security or observability requirements.

Llama Wrangler must continue to keep these values out of ordinary app state, telemetry, UI responses, and support bundles:

- admin tokens
- client API keys
- Splunk HEC tokens
- future provider keys
- plaintext fallback keys
- raw headers
- prompt bodies
- response bodies
- request bodies
- payload fields

## Residual Work

Residual service-credential work moves to Phase H packaging and install hardening:

- real macOS install/uninstall helper validation
- signed/notarized packaging impact on keychain prompts
- systemd service credential behavior
- Windows service credential behavior
- optional keychain-only mode only after service-user behavior is proven

## Phase B Entry

Phase B can now proceed without reopening Phase A. The first Phase B planning target is the Managed Node versus Passive Endpoint model, including persistent control/trust metadata and separate UI flows for full-control subscriber enrollment and limited-control endpoint addition.
