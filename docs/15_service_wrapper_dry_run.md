# Service Wrapper and macOS Packaging Acceptance

This document defines the service-wrapper dry-run workflow and the separate disposable macOS user-level launchd acceptance flow. Dry-run remains the default and is non-mutating. The lifecycle path requires exact opt-in, uses only temporary artifacts and a unique current-user label, and always boots out the job during cleanup.

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
- default `environment.LLAMA_WRANGLER_SECRET_BACKEND: "encrypted_file"`
- optional `environment.LLAMA_WRANGLER_SECRET_BACKEND: "os_keychain"`
- optional `environment.LLAMA_WRANGLER_KEYCHAIN_SERVICE` for disposable keychain namespaces
- validation commands for `plutil` and `launchctl`
- keychain verification commands
- warnings that encrypted fallback remains required and available

For config-file service modes, use `marshal`, `subscriber`, or `standalone`:

```bash
llama-wrangler service-dry-run --target launchd --binary ./llama-wrangler --mode marshal --config ./configs/marshal.example.yaml --keychain
```

For subscriber heartbeat credential rotation, the marshal rotation response includes a `subscriber_install` object with placeholder commands and service-wrapper artifacts. The subscriber config should reference the credential through an environment variable:

```yaml
registration:
  marshal_url: "http://<marshal-host>:11435"
  heartbeat_credential_env: LLAMA_WRANGLER_SUBSCRIBER_HEARTBEAT_CREDENTIAL
```

The launchd dry-run command can include the environment variable placeholder for review:

```bash
llama-wrangler service-dry-run \
  --target launchd \
  --mode subscriber \
  --config ./subscriber.yaml \
  --env 'LLAMA_WRANGLER_SUBSCRIBER_HEARTBEAT_CREDENTIAL=<credential-from-rotation-response>'
```

The rotation response also includes:

- `env_file_template` for shell/systemd-style wrapper workflows
- `service_wrapper.launchd_plist_template` with the credential placeholder in `EnvironmentVariables`
- `service_wrapper.install_commands` for creating local launchd/log directories, linting the plist, bootstrapping, and kickstarting launchd
- `service_wrapper.validation_commands` for launchctl inspection and an authenticated heartbeat probe
- `service_wrapper.uninstall_commands` for booting out the launchd job and removing the plist after review

For subscriber benchmark runner packaging, `GET /wrangler/benchmarks/runner/guidance` exposes placeholder-only env vars, endpoint flow, bounded runner settings, and packaging-hook notes. The built-in runner is an opt-in subscriber-local loop. `dry_run_v1` verifies claim/status/result plumbing and reports deterministic metric summaries only. `synthetic_builtin_v1` executes built-in synthetic suite/task IDs against the subscriber-local Ollama endpoint, discards responses locally, and reports metric summaries only. Operator local fixture manifests remain ID-only until a separate safe manifest execution boundary is implemented. Prompt text, response text, fixture contents, full fixture paths, and raw credentials must not be written into service-wrapper artifacts, support bundles, telemetry, or marshal state.

These are manual operator artifacts for the subscriber host. The marshal does not remotely write subscriber configs, install launchd plists, mutate service wrappers, or start subscriber services.

Do not paste real heartbeat credentials into support bundles, shared logs, tickets, or committed plist/config examples. The raw rotated credential is returned only in the immediate rotation response; ordinary app state, bootstrap, telemetry, and support bundles must contain only safe hints, placeholders, service-wrapper metadata, and env-var names.

`start` intentionally rejects `--config` because the current `start` command is UI-first and does not accept a config path.

## macOS User-Level Package Lifecycle

Generate and lint a disposable package plan without loading launchd:

```bash
./scripts/macos_user_launchd_acceptance.sh
```

The safe default performs only these operations:

1. Creates a temporary home, package root, config, log directory, and LaunchAgents directory.
2. Builds a temporary binary identified as `acceptance-initial`.
3. Generates the launchd plan through `service-dry-run` with absolute paths and encrypted fallback selected explicitly.
4. Writes the plist only inside the temporary home, sets mode `0600`, and validates it with `plutil -lint`.
5. Deletes all temporary artifacts without invoking `launchctl`.

Run the disposable current-user lifecycle only with exact opt-in:

```bash
LLAMA_WRANGLER_MACOS_LAUNCHD_ACCEPTANCE=1 ./scripts/macos_user_launchd_acceptance.sh
```

The opt-in path additionally:

- bootstraps a unique `com.llama-wrangler.acceptance.<uid>.<pid>` label into `gui/<uid>`
- verifies loopback health, setup completion, schema version 2, safe defaults, and encrypted fallback files with `0600` permissions
- verifies admin/client authentication survives `launchctl kickstart -k`
- atomically replaces the disposable binary with an `acceptance-upgrade` build, restarts it, and verifies state/auth/secret posture survives
- scans temporary service logs for credential and payload marker patterns without exporting the logs
- boots out the job, verifies the endpoint and label are gone, removes package artifacts, and deletes the temporary root

Service-mode runtime suppresses recovery-token output. The harness keeps setup credentials in shell variables only and never prints them. It does not write the operator's real `~/Library/LaunchAgents`, mutate `/Library/LaunchDaemons`, use `sudo`, enable OS keychain storage, retain logs/app data, or claim signed release-candidate acceptance.

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
