# Service Wrapper Dry-Run Harness

This document defines the current service-wrapper dry-run workflow. It is intentionally non-mutating: it does not install launchd agents, write plist files, start background services, or modify OS keychain items.

## Scope

The first target is macOS launchd because Phase A keychain evidence currently covers only the interactive macOS user session.

The dry-run harness should help operators answer:

- what plist would be reviewed before install
- what environment marks a runtime as service-like
- whether OS keychain opt-in is present
- what validation commands should be run manually
- whether encrypted fallback remains available

The dry-run harness must not include admin tokens, client API keys, HEC tokens, provider keys, prompt bodies, response bodies, raw request bodies, raw headers, or payloads.

## Command

```bash
llama-wrangler service-dry-run --target launchd --binary ./llama-wrangler --keychain
```

Disposable validation can add non-secret environment variables:

```bash
llama-wrangler service-dry-run \
  --target launchd \
  --binary ./llama-wrangler \
  --mode marshal \
  --config /tmp/llama-wrangler-launchd/marshal.yaml \
  --keychain \
  --env HOME=/tmp/llama-wrangler-launchd/home \
  --env LLAMA_WRANGLER_KEYCHAIN_SERVICE=llama-wrangler-dryrun-test
```

`LLAMA_WRANGLER_KEYCHAIN_SERVICE` is intended for disposable validation namespaces so launchd tests do not overwrite normal `llama-wrangler` keychain items.

The command prints JSON with:

- `dry_run: true`
- `target: launchd`
- `launchd_plist`: review-only plist text
- `environment.LLAMA_WRANGLER_SERVICE_MODE: "1"`
- optional `environment.LLAMA_WRANGLER_SECRET_BACKEND: "os_keychain"`
- optional `environment.LLAMA_WRANGLER_KEYCHAIN_SERVICE` for disposable keychain namespaces
- validation commands for `plutil` and `launchctl`
- keychain verification commands
- warnings that encrypted fallback remains required and available

For config-file service modes, use `marshal`, `subscriber`, or `standalone`:

```bash
llama-wrangler service-dry-run --target launchd --binary ./llama-wrangler --mode marshal --config ./configs/marshal.example.yaml --keychain
```

`start` intentionally rejects `--config` because the current `start` command is UI-first and does not accept a config path.

## Manual Validation Sequence

Use a disposable test environment when practical.

1. Run the interactive keychain check:

   ```bash
   LLAMA_WRANGLER_RUN_KEYCHAIN_TESTS=1 go test -v ./internal/secrets -run TestOSKeychainBackendPlatformOptIn
   ```

2. Generate and review the dry-run output:

   ```bash
   ./llama-wrangler service-dry-run --target launchd --binary ./llama-wrangler --keychain
   ```

3. If an operator chooses to write the plist manually, validate it first:

   ```bash
   plutil -lint ~/Library/LaunchAgents/com.llama-wrangler.marshal.plist
   ```

4. After a manual launchd bootstrap, verify metadata only:

   ```bash
   curl -sS http://localhost:11435/wrangler/ui/bootstrap | jq '.secret_storage'
   ```

Expected service-like metadata:

- `backend` is `os_keychain` when keychain opt-in works, or `encrypted_file` if keychain is unavailable
- `fallback_available` is `true`
- `os_keychain_runtime` is `service_like`
- `os_keychain_service_mode` is `true`
- `os_keychain_warning` is present

## Phase A Decision Rule

Phase A should not close solely because the dry-run harness exists. Phase A can close after one of these is recorded in the ledger:

- launchd service-wrapper validation proves keychain access works for the intended service user/session while encrypted fallback remains available
- the project explicitly decides encrypted fallback is the supported default for service installs, with OS keychain remaining interactive opt-in until packaging hardening

The second path is now the recorded Phase A closure decision. See `docs/17_phase_a_closure_decision.md`.
