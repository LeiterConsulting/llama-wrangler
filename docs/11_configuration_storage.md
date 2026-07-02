# Configuration Storage

Llama Wrangler stores app state in the platform app-data directory:

- macOS: `~/Library/Application Support/Llama Wrangler/`
- Windows: `%APPDATA%\Llama Wrangler\`
- Linux: `~/.config/llama-wrangler/`

The MVP writes `state.json` with:

- setup completion state
- node identity
- persisted service config
- discovered nodes
- session affinity records
- recent metadata-only audit events

Secrets are not stored in `state.json`. Local fallback secret storage uses:

- `secrets.enc.json`: AES-GCM encrypted secret payloads
- `secrets.key`: generated local fallback key, permissioned `0600`

`LLAMA_WRANGLER_SECRETS_KEY` can provide a base64-encoded 32-byte key instead of using `secrets.key`.

Legacy plaintext `secrets.json` files are migrated into `secrets.enc.json` and removed after successful encryption.

When the key source is the local `secrets.key` file, Settings and `POST /wrangler/secrets/rekey` can rotate that fallback key and rewrite `secrets.enc.json` without returning secret values. When `LLAMA_WRANGLER_SECRETS_KEY` supplies the key, local rekey is unavailable and the external key owner remains responsible for rotation.

Secrets are masked or omitted in API responses. Bootstrap and auth status responses expose only secret-storage metadata such as backend, encrypted status, migration status, fallback availability, and key source.

## OS Keychain Backend Spike

The OS keychain feasibility and integration plan lives in `docs/14_os_keychain_plan.md`. A minimal additive backend spike is available behind the existing secret-store API:

```bash
LLAMA_WRANGLER_SECRET_BACKEND=os_keychain ./llama-wrangler start
```

When enabled, Llama Wrangler attempts to use the OS keychain for admin tokens, generated client API keys, Splunk HEC tokens, and future provider keys. Existing encrypted fallback values are copied into the keychain only when the keychain item is missing. The encrypted fallback is retained, remains the default backend when the env var is unset, and is used automatically if the keychain backend is unavailable.

This is not a keychain-only mode. Rekey continues to apply only to the encrypted fallback files, and support bundles continue to include metadata only:

- `backend`: active backend, such as `encrypted_file` or `os_keychain`
- `fallback_backend`: currently `encrypted_file`
- `fallback_available`: whether encrypted fallback remains available
- `os_keychain_status`: `disabled`, `active`, or `unavailable`
- `os_keychain_migrated`: count of fallback values copied into the keychain, never the values
- `os_keychain_platform`: local runtime platform, such as `darwin`, `windows`, or `linux`
- `os_keychain_runtime`: `interactive_user` or `service_like`
- `os_keychain_service_mode`: whether the current process appears to be service-like
- `os_keychain_warning`: service-mode/keychain-access guidance when relevant
- `os_keychain_next_step`: operator guidance

Platform-specific keychain behavior, including macOS prompts, Windows service behavior, and Linux desktop/session availability, still requires opt-in verification before keychain-only or service-keychain behavior can be supported. Phase A closes with encrypted fallback as the supported service/default credential path. `LLAMA_WRANGLER_SERVICE_MODE=1` can be used by install/service wrappers to mark a process as service-like until OS-native service integration is implemented.

## Backup and Restore

Support bundles are diagnostic exports. They are not backups and cannot restore secrets.

For the local encrypted fallback with `key_source: file`, back up these files together from the Llama Wrangler app-data directory:

- `secrets.enc.json`
- `secrets.key`

`secrets.enc.json` cannot be decrypted without the matching `secrets.key`. Restoring only one of those files is not enough. To restore, stop Llama Wrangler, place both files back in the app-data directory, preserve restrictive file permissions where possible, then start Llama Wrangler.

For deployments using `LLAMA_WRANGLER_SECRETS_KEY`, back up:

- `secrets.enc.json`
- the external `LLAMA_WRANGLER_SECRETS_KEY` value in the operator's existing secret manager

Do not store `LLAMA_WRANGLER_SECRETS_KEY` in support bundles, shell history, ordinary docs, or source control. To restore, provide the same environment key before starting Llama Wrangler with the restored `secrets.enc.json`.

Rekey changes the local key and encrypted file together. After a successful local rekey, any previous backup of `secrets.key` no longer decrypts the current `secrets.enc.json`; create a fresh backup pair.

Sanitized export endpoints must not include secrets, prompt bodies, response bodies, or API keys.
