Additive Prompt: Deep Architecture Hardening for Llama Wrangler

You are continuing implementation of Llama Wrangler, a local-first control plane for Ollama fleets with marshal/subscriber roles, capability-aware routing, consensus execution, Frontier Delta escalation, Splunk HEC telemetry, and a friendly web UI.

The prior project scope already defines the core marshal/subscriber service, UI-first setup requirements, Splunk integration, and Codex implementation prompts. This additive prompt hardens the product architecture by adding the missing operational, stateful, security, and compatibility requirements needed for a reliable user-facing system.

Core Principle

Llama Wrangler is not only a request router. It must manage state, sessions, policies, node capability, model lifecycle, client compatibility, safe escalation, and observability so that multiple local Ollama endpoints behave like one coherent local inference platform.

Add These Required Capabilities

1. Session Affinity and Context Continuity

Implement session-aware routing.

The marshal must support session IDs and request lineage. Related requests from IDEs, agents, chat tools, or API clients should be able to remain associated with the same node/model when appropriate.

Support these affinity modes:

* none: every request is independently routed.
* soft: prefer the same node/model but allow rerouting.
* strict: keep a session on the same node/model unless it fails.
* task: keep related agent task chains together.

The default for generic API requests should be soft. The default for stateless extraction/classification can be none. The UI must allow users to view and adjust session behavior per client or model alias.

2. Model Warmness and Load State

Do not route based only on whether a model is installed. Track model state per node:

* installed
* loading
* warm
* busy
* unloaded
* evicted
* failed

Routing should prefer warm models when latency matters. Add a configurable warm model pool per node. The UI should expose a simple “keep warm” toggle for selected models.

3. Streaming, Cancellation, and Backpressure

The marshal must correctly proxy streaming responses for OpenAI-compatible and Ollama-compatible clients.

Add support for request cancellation, client disconnect detection, timeout handling, queue cancellation, and subscriber-side cancellation where possible.

For streaming requests, define retry behavior carefully:

* retry before first token if safe
* do not silently retry after partial output has already streamed to the client

Emit telemetry events for started, completed, failed, cancelled, retried, and partially completed requests.

4. Queueing and Priority

Implement a queue for requests instead of immediate route-and-pray behavior.

Requests should include:

* priority
* client identity
* task type
* timeout
* max retries
* streaming flag
* session ID
* estimated token count
* execution mode
* frontier policy

Interactive IDE/chat requests should generally outrank background batch jobs. Add UI visibility into active requests, queue depth, waiting jobs, and cancellable jobs.

5. Capability Evaluation Must Be Empirical

Capability scoring must be based on observed performance, not just hardware assumptions.

Track per node/model/task:

* tokens per second
* time to first token
* model load time
* context handling
* JSON validity rate
* code/test validation rate
* timeout rate
* error rate
* fallback rate
* user feedback
* frontier correction rate

Use these measurements to improve routing decisions over time.

6. Built-In Benchmark and Evaluation Harness

Add a benchmark harness with practical task evaluations:

* JSON extraction from messy text
* summarization
* regex generation with test cases
* Python syntax/unit-test generation
* simple code explanation
* SPL review
* task classification
* customer-facing rewrite

The first-run wizard should offer to benchmark each node. Results should populate recommended roles and model aliases.

7. Client Compatibility Matrix

Do not assume “OpenAI-compatible” is enough.

Add an explicit compatibility layer for:

* /v1/models
* /v1/chat/completions
* /v1/completions
* /v1/embeddings
* /api/tags
* /api/chat
* /api/generate

Track support for:

* streaming
* tool calls
* JSON mode
* system prompts
* stop tokens
* max tokens
* usage fields
* error formatting
* model aliases

The UI should include a Client Setup Wizard with presets for common local AI tools and a generic OpenAI-compatible configuration page.

8. Tool Calling Boundaries

For MVP, Llama Wrangler should route inference but should not execute arbitrary tools.

Support pass-through tool-call structures where clients expect them, but keep actual tool execution on the client side unless a future controlled tool execution subsystem is explicitly designed.

Document this as an MVP non-goal.

9. Frontier Delta Safety

Before any frontier provider call, run a redaction and policy check.

Detect and optionally redact:

* API keys
* bearer tokens
* private keys
* passwords
* .env values
* AWS/GitHub/Splunk tokens
* hostnames
* customer identifiers
* email addresses
* source code, if policy forbids external code sharing

The UI must show whether a request is local-only, local-first, or may use frontier escalation. Frontier calls must be disabled by default unless configured by the user.

Support payload preview and approval for interactive mode.

10. Auth and Node Enrollment

Do not expose an unauthenticated LAN service by default.

Defaults:

* marshal binds to localhost first
* LAN access requires explicit enablement
* API keys are generated during setup
* subscribers join through an enrollment flow
* marshal must approve new subscribers
* node enrollment should use a token or equivalent trust mechanism

Add mDNS discovery if feasible, with manual enrollment fallback.

11. Model Lifecycle Management

The UI must manage models across nodes:

* show installed models
* show missing recommended models
* pull model to node
* remove model
* warm model
* benchmark model
* tag model for roles
* detect stale or failed model state

The setup wizard should recommend model placement based on detected hardware.

12. Deterministic Validators for Consensus

Consensus mode must not rely only on model judgment.

Where possible, validate outputs deterministically:

* JSON parse
* YAML parse
* regex tests
* Python syntax check
* unit tests
* markdown/frontmatter validation
* shellcheck if available
* schema validation

Use local model judges only after deterministic checks. Use Frontier Delta only when local validation and local judging cannot resolve the disagreement.

13. Splunk Logging Controls

Do not log full prompts/responses by default.

Support logging levels:

* metadata only
* redacted payloads
* summaries only
* full payloads

Default to metadata only.

Emit structured events for:

* request received
* routing decision
* request completed
* request failed
* request cancelled
* node heartbeat
* node capability update
* model benchmark
* model lifecycle event
* consensus comparison
* frontier delta request
* redaction event
* policy denial
* queue state
* session event

The Splunk app should include dashboards and alerts for these events.

14. Failure Semantics

Define how failures behave.

For non-streaming requests:

* retry on eligible fallback nodes
* respect max retries
* preserve request lineage

For streaming requests:

* retry before first token if possible
* do not silently retry after partial output unless explicitly configured

For consensus tasks:

* failed participants should be recorded
* consensus may proceed if minimum participants complete
* frontier escalation may occur if policy permits

15. Non-Goals for MVP

Do not implement these in MVP:

* splitting a single model across multiple machines
* replacing Ollama
* requiring Kubernetes
* executing arbitrary agent tools server-side
* logging full prompts/responses by default
* exposing LAN endpoints without explicit user approval
* sending prompts to frontier models by default
* requiring manual .env or YAML edits for normal operation

Acceptance Criteria

A successful implementation should allow a user to:

1. Install Llama Wrangler on multiple machines.
2. Start one marshal and multiple subscribers.
3. Enroll subscribers through the UI.
4. Auto-detect Ollama, hardware, models, and capabilities.
5. Run capability benchmarks.
6. Accept or override recommended roles.
7. Configure model aliases.
8. Point an IDE or agent tool at one local endpoint.
9. Run normal single-node routing.
10. Run consensus execution.
11. Cancel a streaming request.
12. View queue and node health.
13. Configure Splunk HEC without editing files.
14. Send operational telemetry to Splunk.
15. Keep prompt/response logging disabled or redacted by default.
16. Enable Frontier Delta only with explicit policy and redaction controls.
17. View request history, routing decisions, failures, and model performance in the UI and Splunk.