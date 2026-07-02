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

The MVP helpers print guidance; platform service registration can be added without changing the service API.

## Service Wrapper Dry Run

`llama-wrangler service-dry-run` prints review-only service-wrapper artifacts. It does not write files, load launchd agents, start background services, or modify keychain items.

The first target is macOS launchd:

```bash
llama-wrangler service-dry-run --target launchd --binary ./llama-wrangler --keychain
```

The output includes a launchd plist, `LLAMA_WRANGLER_SERVICE_MODE=1`, optional `LLAMA_WRANGLER_SECRET_BACKEND=os_keychain`, validation commands, keychain check commands, and warnings that encrypted fallback remains required. See `docs/15_service_wrapper_dry_run.md`.
