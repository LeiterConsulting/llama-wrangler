# OS Keychain Integration Plan

This plan covers moving Llama Wrangler secrets toward OS-managed credential storage while preserving the encrypted local fallback already implemented.

## Scope

Secrets in scope:

- admin token
- generated client API keys
- Splunk HEC token
- future frontier provider keys
- future subscriber enrollment or trust tokens

Secrets out of scope:

- prompt bodies
- response bodies
- request payloads
- raw headers
- support-bundle contents
- ordinary app state in `state.json`

## Current State

The current fallback is intentionally separate from ordinary app state:

- `state.json` stores config, topology, sessions, and metadata.
- `secrets.enc.json` stores encrypted secret values.
- `secrets.key` or `LLAMA_WRANGLER_SECRETS_KEY` provides the fallback encryption key.
- bootstrap, auth status, support bundles, and Settings expose status metadata only.
- support bundles are diagnostic artifacts, not backups or restore/import formats.

This fallback remains required even after OS keychain support because Linux headless environments, locked keychains, service accounts, and CI-like runs may not have an interactive credential store.

A minimal additive backend spike now exists behind the existing secret-store API. It is disabled by default and can be enabled with:

```bash
LLAMA_WRANGLER_SECRET_BACKEND=os_keychain ./llama-wrangler start
```

When enabled, Llama Wrangler attempts to store and read secrets from the OS keychain while retaining the encrypted fallback. If the keychain backend is unavailable, the service marks keychain status as `unavailable` and continues with encrypted fallback.

The status surface also reports platform/runtime context:

- `os_keychain_platform`
- `os_keychain_runtime`
- `os_keychain_service_mode`
- `os_keychain_warning`

Service-like runtimes can be detected from common service environment markers or explicitly marked with `LLAMA_WRANGLER_SERVICE_MODE=1`. This is metadata-only guidance; it does not disable fallback storage.

The service-wrapper dry-run harness in `docs/15_service_wrapper_dry_run.md` starts with macOS launchd. It emits review-only plist JSON and validation commands without installing services or modifying keychain items.

## Candidate Libraries

Primary candidates as of 2026-07-02:

- `github.com/zalando/go-keyring`
  - Provides an OS-agnostic interface for set/get/delete.
  - Supports macOS, Linux/BSD through dbus, and Windows.
  - Avoids C bindings by using platform commands/APIs where practical.
  - Good fit for a small first implementation with a narrow interface.
- `github.com/99designs/keyring`
  - Provides a broader backend matrix, including macOS Keychain, Windows Credential Manager, Secret Service, KWallet, Pass, encrypted file, and KeyCtl.
  - Useful if Linux desktop diversity or explicit backend selection becomes more important than a minimal first pass.
  - Larger surface area than the first implementation needs.

The first spike uses a narrow Llama Wrangler backend boundary with `github.com/zalando/go-keyring` as the OS backend and keeps the existing encrypted file store as the fallback backend.

## Backend Contract

The backend boundary inside `internal/secrets` is intentionally narrow:

```go
type SecretBackend interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
}
```

The public `Store` behavior should remain stable:

- `Get`, `Set`, `Delete`, and `Match` keep their current semantics.
- `Rekey` applies only to encrypted fallback storage.
- API responses keep returning metadata only.
- support bundles keep returning status and guidance only.
- failure to open an OS backend falls back to encrypted file storage unless the operator explicitly requires keychain-only mode later.

## Secret Naming

Use deterministic service/item names so migration is predictable:

- service: `llama-wrangler`
- account/item keys:
  - `admin_token`
  - `splunk_hec_token`
  - `api_key:<id>`
  - `frontier_provider:<provider>:<name>` later
  - `subscriber_token:<node_id>` later

Do not store model prompts, responses, request bodies, raw headers, or support bundles in the keychain.

## Migration Strategy

The first OS keychain implementation is non-destructive:

1. Open encrypted fallback store.
2. Attempt to open OS keychain backend.
3. If OS keychain opens, copy known fallback secrets into OS keychain only when the key does not already exist there.
4. Keep fallback encrypted secrets intact during the first implementation slice.
5. Mark status metadata with:
   - active backend
   - fallback availability
   - migrated count only, never migrated values
6. Add a later explicit cleanup action after successful multi-release confidence, if desired.

This avoids losing access when an OS keychain is locked, unavailable in a service context, or different between interactive and background launches.

## Platform Notes

macOS:

- Target macOS Keychain.
- Expect first-access prompts depending on signing, binary path, and keychain ACL behavior.
- Unsigned development binaries may create repeated access prompts; package signing can improve this later.

Windows:

- Target Windows Credential Manager.
- Confirm behavior for services versus interactive users before enabling keychain-only mode.

Linux:

- Target Secret Service/dbus where available.
- Headless servers, SSH sessions, and minimal desktops may not have a usable keyring.
- Encrypted fallback remains the expected safe path for those environments.

## UI and API Status

Settings shows:

- active secret backend: `os_keychain` or `encrypted_file`
- OS backend status: `disabled`, `active`, or `unavailable`
- fallback available: yes/no
- migrated count only, when non-zero
- rekey availability for encrypted fallback only
- backup guidance for encrypted fallback only

The UI must not show secret values. API responses must not include keychain item contents, HEC tokens, admin tokens, client API keys, provider keys, prompts, responses, request bodies, raw headers, or payloads.

Planning and status metadata must not include prompt bodies, response bodies, request payloads, raw headers, or plaintext secret values.

## Tests and Verification

Unit tests:

- backend selection falls back to encrypted file when OS keychain is unavailable
- `Store` API semantics remain unchanged
- status metadata never includes secret values
- service-like runtime status reports warning metadata while retaining fallback
- rekey remains unsupported for OS keychain and env-key modes
- support bundles remain secret-free

Integration tests:

- macOS keychain set/get/delete behind an opt-in test flag
- Windows Credential Manager set/get/delete behind an opt-in test flag
- Linux Secret Service set/get/delete behind an opt-in test flag
- service/install wrapper checks should run with `LLAMA_WRANGLER_SERVICE_MODE=1` and confirm the same service user/session can still read expected keychain items
- launchd dry-run output should be reviewed and manually validated before any real service install is attempted

Live verification:

- bootstrap and auth status show backend metadata only
- Settings renders backend status without token values
- generated admin and client credentials still work after restart
- Splunk HEC token presence remains `has_token`, not the token value

## Risks

- OS keychain may be unavailable in headless or service-mode runs.
- macOS access prompts may be noisy until packaging/signing is mature.
- Linux keyring availability varies widely across distributions and desktop/session types.
- Background services may run as a different OS user than the first-run UI.
- A destructive migration could lock users out; the first slice must be additive.

## Decision

Phase A should not be marked complete on planning alone. The minimal additive OS keychain backend spike now:

- adds the internal backend boundary
- adds optional OS keychain backend activation through `LLAMA_WRANGLER_SECRET_BACKEND=os_keychain`
- keeps encrypted fallback available and non-destructively migrates values into OS keychain when available
- exposes metadata-only backend status in Settings, bootstrap, auth status, and support bundles
- adds unit tests plus an opt-in platform integration check with `LLAMA_WRANGLER_RUN_KEYCHAIN_TESTS=1`

The launchd dry-run and disposable validation evidence is now recorded in `docs/17_phase_a_closure_decision.md`. Phase A closes with encrypted fallback as the supported service/default credential path, OS keychain as interactive opt-in, and service-keychain behavior as packaging hardening.

Future packaging work can promote stronger OS keychain guarantees only after service-user behavior is proven for the target install mode.

## References

- `github.com/zalando/go-keyring`: https://github.com/zalando/go-keyring
- `github.com/99designs/keyring`: https://github.com/99designs/keyring
- Git credential helper platform storage reference: https://git-scm.com/doc/credential-helpers
