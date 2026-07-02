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
