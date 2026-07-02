# Codex Prompt: Build Llama Wrangler MVP Service

You are building the MVP of Llama Wrangler, a local-first control plane for Ollama fleets.

## Goal

Create a service that can run in marshal or subscriber mode. The marshal exposes OpenAI-compatible and Ollama-compatible endpoints. Subscribers register with the marshal, report local capabilities, and proxy requests to their local Ollama instance.

## Use language/runtime

Prefer Go for a single distributable binary. If Go is impractical, use Python/FastAPI with clear module boundaries.

## MVP requirements

1. CLI command with modes:
   - `marshal --config <file>`
   - `subscriber --config <file>`
   - `standalone --config <file>`

2. Config loading from YAML.

3. Marshal endpoints:
   - `GET /healthz`
   - `GET /wrangler/nodes`
   - `GET /wrangler/models`
   - `POST /v1/chat/completions`
   - `POST /api/chat`
   - `POST /api/generate`

4. Subscriber endpoints:
   - `GET /healthz`
   - `GET /subscriber/capabilities`
   - `POST /subscriber/proxy/api/chat`
   - `POST /subscriber/proxy/api/generate`

5. Subscriber capability detection:
   - hostname
   - OS/platform
   - architecture
   - memory total/available where feasible
   - Ollama availability
   - Ollama `/api/tags` model list

6. Routing:
   - static subscriber list from marshal config
   - model alias resolution
   - best available node by model availability and health
   - fallback to next eligible node on failure

7. Streaming:
   - preserve Ollama streaming where possible
   - preserve OpenAI-compatible streaming where possible

8. Telemetry:
   - structured JSON logs
   - implement HEC client but allow disabling
   - emit request, routing_decision, response, node_health, and error events

## Constraints

- Do not implement model-parallel inference.
- Do not store prompt/response bodies in telemetry by default.
- Do not send data to frontier providers in MVP.
- Keep interfaces clean for future consensus/frontier features.

## Deliverables

- Compilable/runnable service
- Example configs
- README with run instructions
- Basic unit tests for config, routing, and HEC event generation
