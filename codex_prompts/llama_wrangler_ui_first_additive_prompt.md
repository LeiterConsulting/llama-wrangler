# Additive Codex Prompt: Make Llama Wrangler a Friendly Installable App

You are continuing the existing **Llama Wrangler** project. The current project already scopes the marshal/subscriber Ollama control plane, capability-aware routing, consensus execution, Frontier Delta escalation, Splunk HEC telemetry, and an accompanying Splunk app.

This additive prompt changes the product requirement from “developer-operated service with config files” to **friendly installable local app with guided setup, auto-detection, and no manual `.env`, YAML, or config editing required for normal users**.

The goal is to make Llama Wrangler usable by a technical-but-not-devops user who wants to install it, launch a UI, discover local machines, connect Ollama endpoints, configure Splunk HEC, and point an IDE/agentic tool at the marshal endpoint without manually editing files.

---

## Product Reframing

Llama Wrangler should be delivered as both:

1. A background service / daemon that performs marshal and subscriber functions.
2. A friendly local web UI / desktop-like app used for setup, configuration, monitoring, and daily operation.

The user experience should feel closer to a network appliance, NAS admin UI, or Home Assistant-style local app than a CLI-only developer tool.

Normal users should not need to:

- edit `.env` files
- hand-author YAML
- manually discover IP addresses
- manually create service definitions
- manually create HEC payloads
- manually configure model aliases before first use
- manually determine which machine should be marshal vs subscriber

Advanced users may still export/import/edit YAML, but the UI should be the primary configuration surface.

---

## Core UX Principles

1. **First-run wizard over manual setup**
   - On first launch, guide the user through setup.
   - Detect local Ollama.
   - Ask whether this node should be the marshal, subscriber, or auto-selected.
   - Offer simple recommended defaults.

2. **Auto-detect before asking**
   - Detect OS, CPU, GPU, RAM, Ollama availability, installed models, network hostname, reachable peers, and likely role.
   - Only ask the user when detection cannot safely infer the answer.

3. **One-click recommended configuration**
   - Provide a “Recommended Setup” button that assigns roles and routing policies automatically.
   - The user can refine later.

4. **Safe defaults**
   - Local-only by default.
   - Frontier models disabled by default until explicitly configured.
   - Prompt/response body logging disabled by default.
   - Splunk HEC optional but strongly supported.

5. **Explain decisions clearly**
   - If the platform chooses the RTX 4090 as the primary coding node, explain why.
   - If the M4 Mac mini is selected as marshal, explain why.

6. **No secrets in plaintext UI**
   - API keys and HEC tokens must be masked.
   - Store secrets using the OS keychain/credential store when possible.
   - Provide fallback encrypted local storage if keychain support is unavailable.

---

## Required Application Surfaces

Build a local web UI served by the Llama Wrangler service.

Recommended default:

```text
http://localhost:11435/ui
```

The UI should be available on the marshal and optionally on subscribers. Subscriber UI can be read-only or limited to local node configuration.

### Required Pages

#### 1. First-Run Wizard

Steps:

1. Welcome
2. Local system scan
3. Ollama detection
4. Role selection/recommendation
5. Peer discovery
6. Model inventory
7. Routing profile generation
8. Splunk HEC setup, optional
9. Frontier provider setup, optional
10. IDE/agent endpoint instructions
11. Final review and launch

The wizard should allow skipping optional sections and returning later.

#### 2. Dashboard

Show:

- Current marshal endpoint
- Active subscribers
- Node health
- Running requests
- Queue depth
- Model aliases
- Recent routing decisions
- Current execution mode
- Splunk HEC status
- Frontier Delta status

#### 3. Nodes Page

Show each node as a card:

- Display name
- Hostname/IP
- Role: marshal/subscriber/standalone
- OS/platform
- CPU/GPU/RAM summary
- Ollama status
- Installed models
- Current load
- Current jobs
- Observed tokens/sec by model
- Assigned capabilities
- Manual overrides

Allow actions:

- Rename node
- Enable/disable node
- Pin roles
- Set max concurrent jobs
- Mark as preferred for coding/general/long-context/embeddings
- Mark as battery-sensitive
- Run benchmark
- Pull model, optional if allowed
- Remove node

#### 4. Models & Aliases Page

Show:

- Installed models across fleet
- Which nodes have each model
- Model size/quant where known
- Observed performance
- Recommended use
- Alias mappings

Provide friendly default aliases:

```text
local-fast
local-code
local-long
local-json
local-consensus
local-best
```

Allow the user to edit aliases visually without YAML.

#### 5. Routing Policies Page

Expose routing modes:

- Single best node
- Race
- Consensus
- Consensus Delta
- Local-only
- Frontier Delta

Allow simple sliders/toggles:

- Prefer speed vs quality
- Allow parallel local attempts
- Max participating nodes
- Require consensus for risky tasks
- Allow frontier escalation
- Require approval before external calls
- Prefer low-power nodes when possible
- Avoid battery-powered nodes

Advanced section can expose rule editing.

#### 6. Splunk Page

Provide full guided setup for Splunk HEC.

Fields:

- Splunk HEC URL
- HEC token
- Index
- Source
- Sourcetype prefix
- SSL verification toggle
- Test connection button
- Send sample event button
- Open dashboard link/instructions

The UI should validate:

- URL format
- token presence
- HEC reachability
- response status
- SSL errors
- index configuration warnings if detectable

Also show recent HEC delivery status:

- last successful event
- failed event count
- retry queue
- average delivery latency

#### 7. Frontier Providers Page

Disabled by default.

Support provider configuration using UI-managed credentials:

- OpenAI-compatible provider base URL
- API key
- default model
- max daily spend
- max per-request spend
- require approval toggle
- summaries-only mode
- never-send-source-code mode
- secrets detection requirement

Do not implement provider-specific assumptions in the UI unless adapters exist. Start with generic OpenAI-compatible provider support.

#### 8. IDE / Agent Setup Page

Show copy/paste configuration snippets for common clients:

- OpenAI-compatible endpoint
- Ollama-compatible endpoint
- Cline / Continue / Open WebUI / generic tools

Show:

```text
Base URL: http://<marshal-host>:11435/v1
Model: local-code
API key: <generated local key, if auth enabled>
```

Provide a “Test prompt” button that sends a small request through the marshal and shows which node handled it.

#### 9. Logs & Audit Page

Show human-readable request trail:

- request id
- client
- model alias
- selected node
- execution mode
- latency
- success/failure
- fallback used
- frontier used/not used
- Splunk HEC status

Do not show prompt/response bodies unless explicit debug logging is enabled.

#### 10. Settings Page

Allow configuration of:

- service ports
- auth/API keys
- UI access control
- data retention
- telemetry settings
- backup/export/import
- advanced config editor
- reset wizard

---

## Installation Requirements

The app should be easy to install and run on macOS, Linux, and Windows.

### MVP Installation Targets

At minimum, support:

```text
macOS Apple Silicon
Windows 11 with NVIDIA GPU
Linux x86_64
```

### Preferred Install Experience

Provide:

1. Single binary download where possible.
2. `install` command that sets up the service interactively.
3. UI-driven first-run setup.
4. Optional launch-at-login / background service install.
5. Clear uninstall command.

Example commands:

```bash
llama-wrangler install
llama-wrangler start
llama-wrangler open
llama-wrangler uninstall
```

The CLI may exist, but should guide users to the UI.

If a user starts the binary directly, it should print:

```text
Llama Wrangler is running.
Open the setup UI: http://localhost:11435/ui
```

---

## Configuration Storage

Do not require manual `.env` or YAML editing.

Use an internal app data directory:

### macOS

```text
~/Library/Application Support/Llama Wrangler/
```

### Windows

```text
%APPDATA%\Llama Wrangler\
```

### Linux

```text
~/.config/llama-wrangler/
```

Store:

- app config
- node identity
- discovered peers
- routing policies
- benchmark results
- telemetry queue
- local audit logs

Secrets should be stored separately using OS credential storage where possible.

Provide export/import:

```text
Export sanitized support bundle
Export full backup
Import config backup
```

Sanitized support bundle must not include secrets, prompt bodies, response bodies, or API keys.

---

## Auto-Detection Requirements

Implement detection modules for:

### Local system

- hostname
- OS
- architecture
- CPU model if available
- total memory
- available memory
- GPU summary where practical
- battery/power status where practical

### Ollama

- whether Ollama is installed
- whether Ollama is running
- Ollama API URL
- Ollama version if available
- installed models via `/api/tags`
- ability to run a test prompt

### Network peers

Use safe LAN discovery.

Start with one or more:

- mDNS/Bonjour advertisement
- manual invite code
- manual host add
- subnet scan only with explicit user approval

Do not perform aggressive scanning by default.

### Capabilities

Infer capabilities such as:

- light-chat
- code
- long-context
- embeddings
- json-extractor
- consensus-participant
- frontier-reviewer
- marshal-candidate
- subscriber-candidate

Use hardware, models, benchmarks, and user overrides.

---

## First-Run Recommended Setup Logic

When a fleet is discovered, recommend roles automatically.

Example for the target environment:

```text
M4 Mac mini:
  Recommended role: Marshal
  Reason: always-on, low power, stable host for clients

M3 Pro 18GB:
  Recommended role: Subscriber / medium MLX worker
  Reason: Apple Silicon, moderate memory, good for local-fast/local-general

M1 Max 64GB:
  Recommended role: Subscriber / long-context and secondary coding worker
  Reason: large unified memory, good for heavier local jobs

i9-13900K + RTX 4090:
  Recommended role: Subscriber / primary coding and heavy inference worker
  Reason: strongest GPU throughput for models that fit in VRAM
```

The UI should show this as editable recommendations, not hard-coded behavior.

---

## UI Technology Guidance

Prefer a simple embedded web app.

Recommended stack:

- Backend: Go service
- UI: React + TypeScript + Vite
- Styling: simple component library or clean CSS
- API: JSON REST endpoints plus optional SSE/WebSocket for live updates

The backend should serve the compiled UI assets from the same binary if possible.

Avoid requiring a separate Node.js runtime after build.

---

## Backend API Additions

Add internal management endpoints for UI use.

Examples:

```text
GET  /wrangler/ui/bootstrap
GET  /wrangler/ui/status
POST /wrangler/setup/start
POST /wrangler/setup/scan-local
POST /wrangler/setup/detect-ollama
POST /wrangler/setup/discover-peers
POST /wrangler/setup/apply-recommended
POST /wrangler/setup/test-ollama
POST /wrangler/setup/test-hec
POST /wrangler/setup/complete

GET  /wrangler/config
PUT  /wrangler/config
POST /wrangler/config/export
POST /wrangler/config/import

GET  /wrangler/nodes
POST /wrangler/nodes/:id/benchmark
PUT  /wrangler/nodes/:id/overrides
POST /wrangler/nodes/:id/disable
POST /wrangler/nodes/:id/enable

GET  /wrangler/models
GET  /wrangler/aliases
PUT  /wrangler/aliases

GET  /wrangler/routing/policies
PUT  /wrangler/routing/policies

GET  /wrangler/telemetry/status
POST /wrangler/telemetry/test-hec

GET  /wrangler/audit/recent
```

Protect management endpoints with local auth once setup is complete.

---

## Security Requirements

1. First-run setup should generate a local admin token/password.
2. UI should require authentication when exposed beyond localhost.
3. API keys for IDE/agent clients should be generated and managed in UI.
4. Secrets should be masked and stored securely.
5. Frontier provider calls require explicit enablement.
6. External calls should support approval workflows.
7. Prompt/response logging disabled by default.
8. Provide an obvious “Local Only Mode” switch.

---

## Friendly Error Handling

Errors should be actionable.

Bad:

```text
connection refused
```

Good:

```text
Could not reach Ollama at http://localhost:11434.
Ollama may not be running on this machine. Start Ollama, then click Retry.
```

Bad:

```text
HEC 403
```

Good:

```text
Splunk rejected the HEC token. Check that the token is enabled and allowed to write to the selected index.
```

---

## Splunk App UI Integration

The local Llama Wrangler UI and Splunk App should complement each other.

Local UI handles:

- install/setup
- fleet configuration
- routing policy
- secrets
- frontier provider config
- live node operations

Splunk App handles:

- historical observability
- dashboards
- performance trends
- failure analysis
- consensus effectiveness
- cost/frontier tracking
- audit reporting

Add a Splunk setup helper in the local UI that can:

- show required Splunk HEC settings
- send sample events
- validate HEC connectivity
- show expected index/sourcetype names
- link to bundled Splunk app installation docs

---

## MVP UI Deliverables

Implement the following in the first UI-focused pass:

1. Embedded local web UI served by marshal.
2. First-run wizard.
3. Local system and Ollama detection.
4. Manual peer add plus mDNS discovery if feasible.
5. Node dashboard.
6. Model inventory page.
7. Visual model alias editor.
8. Splunk HEC setup/test page.
9. IDE setup page with copy/paste endpoint instructions.
10. Config persistence without manual files.
11. Masked secret storage.
12. JSON structured audit log view.
13. Basic service install/start/open commands.

Do not block the MVP on:

- perfect GPU detection across every platform
- polished desktop packaging
- advanced frontier-provider adapters
- full drag-and-drop workflow design
- multi-user RBAC
- complex subnet scanning

---

## Acceptance Criteria

A successful implementation allows a user to:

1. Download or build Llama Wrangler.
2. Run one command to start it.
3. Open a browser UI.
4. Complete first-run setup without editing a config file.
5. Detect local Ollama and installed models.
6. Add at least one subscriber node through the UI.
7. See node capabilities and models.
8. Create or accept recommended model aliases.
9. Configure Splunk HEC through the UI and send a test event.
10. Copy an OpenAI-compatible endpoint into an IDE/agent tool.
11. Send a test prompt and see which node handled it.
12. View the event in the local audit log and, if configured, in Splunk.

---

## Documentation Updates Required

Update existing docs to reflect the UI-first experience.

Add or revise:

```text
docs/09_friendly_ui_and_installation.md
docs/10_first_run_wizard.md
docs/11_configuration_storage.md
docs/12_ui_api.md
codex_prompts/06_build_friendly_ui.md
```

Update README.md with:

- Quick Start
- UI setup path
- no-manual-config positioning
- service modes explained in user-friendly language
- screenshots placeholder section
- troubleshooting section

---

## Suggested Codex Implementation Plan

Work in phases.

### Phase 1: Backend support for UI

- Add app data directory handling.
- Add persistent config store.
- Add setup state detection.
- Add management API endpoints.
- Add secret abstraction.
- Add local system detection.
- Add Ollama detection.

### Phase 2: Basic embedded UI

- Add React/Vite UI.
- Serve compiled UI from backend.
- Build first-run wizard.
- Build dashboard shell.
- Build nodes/models/config pages.

### Phase 3: Guided integrations

- Add Splunk HEC setup/test UI.
- Add IDE setup page.
- Add API key generation.
- Add routing policy editor.

### Phase 4: Polish

- Add friendly error messages.
- Add support bundle export.
- Add service install helpers.
- Add visual status indicators.
- Add documentation and screenshots.

---

## Final Instruction to Codex

Treat usability as a core feature, not a later enhancement. The existing Llama Wrangler technical architecture is still valid, but the default operator experience must be UI-driven and wizard-guided. Advanced config files may exist, but the normal path must be install, open UI, auto-detect, accept recommendations, test, and use.

Build toward the experience where a user can say:

> I installed Llama Wrangler, it found my Ollama nodes, recommended the best marshal/subscriber setup, configured Splunk HEC, generated my IDE endpoint, and started routing requests — without me editing a single config file.
