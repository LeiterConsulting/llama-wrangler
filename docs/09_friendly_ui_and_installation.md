# Friendly UI and Installation

Llama Wrangler is implemented as a local background service with an embedded web UI. The default user path is:

1. Run `llama-wrangler start`.
2. Open `http://localhost:11435/ui`.
3. Complete first-run setup.
4. Copy the generated IDE endpoint.

The service uses safe defaults:

- localhost-only binding
- local-only Frontier policy
- metadata-only telemetry
- disabled prompt/response body logging
- soft session affinity

Advanced YAML configuration remains available through `marshal`, `subscriber`, and `standalone` CLI modes.

## Installation Commands

```bash
llama-wrangler install
llama-wrangler start
llama-wrangler open
llama-wrangler service-dry-run --target launchd --keychain
llama-wrangler uninstall
```

The `install` and `uninstall` commands still print guidance. The macOS acceptance harness validates disposable current-user launchd lifecycle mechanics without turning those command stubs into a production installer.

## Service Wrapper Dry Run

`llama-wrangler service-dry-run` prints review-only service-wrapper artifacts. It does not write files, load launchd agents, start background services, or modify keychain items.

The first target is macOS launchd:

```bash
llama-wrangler service-dry-run --target launchd --binary ./llama-wrangler --keychain
```

The output includes an absolute-path launchd plist, `LLAMA_WRANGLER_SERVICE_MODE=1`, explicit `LLAMA_WRANGLER_SECRET_BACKEND=encrypted_file` by default, optional OS keychain opt-in, validation commands, keychain check commands, and warnings that encrypted fallback remains required. Service-mode startup suppresses the recovery token from stdout/stderr logs.

## Disposable macOS Packaging Acceptance

Run the safe default from the repository root:

```bash
./scripts/macos_user_launchd_acceptance.sh
```

This builds a temporary package, renders its plist into a temporary `Library/LaunchAgents`, and runs `plutil -lint`. It does not call `launchctl`.

The real current-user lifecycle check requires exact opt-in:

```bash
LLAMA_WRANGLER_MACOS_LAUNCHD_ACCEPTANCE=1 ./scripts/macos_user_launchd_acceptance.sh
```

The opt-in flow registers a unique temporary label only in `gui/<uid>`, verifies install/start, setup and encrypted-secret posture, restart persistence, atomic binary replacement, and uninstall, then removes the job and all temporary files. It does not write the real user LaunchAgents directory, use system LaunchDaemons, enable OS keychain storage, or establish signed release-package support. See `docs/15_service_wrapper_dry_run.md` and `docs/21_v1_acceptance_security_matrix.md`.
