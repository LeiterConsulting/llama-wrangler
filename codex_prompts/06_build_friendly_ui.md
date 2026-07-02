# Codex Prompt: Build Friendly UI

Continue Llama Wrangler as a UI-first installable local app.

## Required MVP

1. Serve an embedded local UI from the service at `/ui`.
2. Provide first-run setup without manual YAML or `.env` editing.
3. Persist app state in the platform app-data directory.
4. Detect local system and Ollama model inventory.
5. Show nodes, models, aliases, telemetry status, IDE setup, and audit events.
6. Keep safe defaults:
   - localhost binding
   - Frontier disabled
   - local-only policy
   - metadata-only telemetry
   - prompt/response body logging disabled
7. Keep APIs ready for future React/Vite replacement and OS keychain secret storage.

## Non-goals

- Do not block MVP on polished native desktop packaging.
- Do not require users to manually author config files.
- Do not expose LAN endpoints without explicit approval.
