# MVP Roadmap

## Phase 0: Skeleton

- Go module or Python/FastAPI project created
- Single binary command structure
- Config loading
- Structured JSON logging
- Health endpoint

## Phase 1: Basic marshal/subscriber

- Marshal mode
- Subscriber mode
- Subscriber registration
- Managed Node versus Passive Endpoint control-level model
- Node health checks
- Model inventory via Ollama `/api/tags`
- Basic request proxy to one subscriber

## Phase 2: API compatibility

- OpenAI `/v1/chat/completions`
- Ollama `/api/chat`
- Streaming pass-through
- Non-streaming response normalization
- Model aliases

## Phase 3: Routing

- Static routing
- Weighted best available routing
- Fallback on failure
- Per-client policies
- Control/trust-aware routing for Managed Nodes and Passive Endpoints
- Basic task type classifier

## Phase 4: Telemetry and Splunk

- HEC client
- Event schema implementation
- Request/response events
- Node health events
- Routing decision events
- Splunk app dashboards/searches

## Phase 5: Consensus

- Multi-node fan-out
- Response collection
- Basic agreement scoring
- Deterministic validators for JSON/regex/code snippets where possible
- Consensus event telemetry

## Phase 6: Frontier Delta

- Provider abstraction
- Delta payload builder
- Policy engine
- Redaction hooks
- Cost estimates
- Approval option
- Frontier telemetry

## Phase 7: UI and polish

- Local dashboard
- Node management
- Separate UI flows for installing/enrolling Managed Nodes and adding Passive Endpoints
- Control-level and trust-level badges
- Policy editor
- Replay/debug traces
- Installer scripts

## V1 Release Gate

- Run `scripts/v1_acceptance.sh` from a clean checkout.
- On macOS, run the default `scripts/macos_user_launchd_acceptance.sh` plan/lint gate and attach explicit opt-in disposable lifecycle evidence where required.
- Require every automated row in `docs/21_v1_acceptance_security_matrix.md` to pass.
- Attach real environment evidence for each intended platform's service packaging, signing/integrity, supported external clients, and Splunk runtime rows.
- Do not convert unavailable, skipped, or environment-dependent checks into passes through documentation.
- Keep acceptance artifacts metadata-only and exclude app data, credentials, raw HTTP captures, inference content, local fixtures, and unredacted service logs.

## Future V2: Capability Endpoints

Only after the V1 Ollama fleet control plane is functional, consider broader Capability Endpoint work:

- generic endpoint type registry
- non-Ollama local runtimes such as LM Studio, MLX, or vLLM
- frontier provider endpoint adapters behind explicit policy and approval
- Codex-style agent, IDE-agent, GitHub, Xcode, Docker, CI/CD, Splunk, or build-runner integrations
- generic capability-based routing and policy beyond inference

Do not add these integrations, UI pages, or tool-execution surfaces during the MVP unless a later ledger decision explicitly starts V2 work.
