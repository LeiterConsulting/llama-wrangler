let state = {};
let view = "wizard";
let adminToken = localStorage.getItem("llama_wrangler_admin_token") || "";

const title = document.querySelector("#title");
const subtitle = document.querySelector("#subtitle");
const content = document.querySelector("#content");

document.querySelectorAll("aside button").forEach((button) => {
  button.addEventListener("click", () => {
    view = button.dataset.view;
    render();
  });
});
document.querySelector("#refresh").addEventListener("click", load);

async function api(path, options = {}) {
  const headers = { "Content-Type": "application/json", ...(options.headers || {}) };
  if (adminToken) headers.Authorization = `Bearer ${adminToken}`;
  const res = await fetch(path, {
    ...options,
    headers,
  });
  const data = await res.json().catch(() => ({}));
  if (res.status === 401 && data.error === "admin_auth_required") {
    renderAuth(data);
    throw new Error(data.message || "Admin authentication required");
  }
  if (!res.ok) throw new Error(data.message || data.error || res.statusText);
  return data;
}

async function load() {
  const headers = {};
  if (adminToken) headers.Authorization = `Bearer ${adminToken}`;
  const res = await fetch("/wrangler/ui/bootstrap", { headers });
  const data = await res.json().catch(() => ({}));
  if (res.status === 401) {
    renderAuth(data);
    return;
  }
  if (!res.ok) throw new Error(data.message || data.error || res.statusText);
  state = data;
  render();
}

function renderAuth(data) {
  title.textContent = "Admin Unlock";
  subtitle.textContent = "Management settings are protected after setup.";
  const recovery = data.recovery_admin_token ? `<p>Recovery token for this local session:</p><pre>${escapeHTML(data.recovery_admin_token)}</pre>` : "";
  content.innerHTML = `<article class="card">
    <h2>Enter Admin Token</h2>
    <p>Hint: <code>${escapeHTML(data.admin_token_hint || "not available")}</code></p>
    ${recovery}
    <label>Admin token<input id="admin-token" type="password" placeholder="lw_admin_..."></label>
    <button class="primary" data-action="unlock">Unlock</button>
  </article>`;
  action("unlock", () => {
    adminToken = value("admin-token");
    localStorage.setItem("llama_wrangler_admin_token", adminToken);
    return load();
  });
}

function render() {
  document.querySelectorAll("aside button").forEach((button) => button.classList.toggle("active", button.dataset.view === view));
  const views = {
    wizard: renderWizard,
    dashboard: renderDashboard,
    nodes: renderNodes,
    models: renderModels,
    splunk: renderSplunk,
    ide: renderIDE,
    audit: renderAudit,
    settings: renderSettings,
  };
  views[view]();
}

function renderWizard() {
  title.textContent = "First-Run Setup";
  subtitle.textContent = state.setup_complete ? "Setup is complete. You can rerun scans or adjust recommendations." : "Scan, accept safe defaults, test integrations, and launch.";
  content.innerHTML = `
    <div class="grid">
      ${step("1", "Local system scan", "Detect hostname, OS, architecture, Ollama, and installed models.", "Scan local node", "scan")}
      ${step("2", "Recommended setup", "Apply local-only, metadata-only telemetry, soft sessions, and default aliases.", "Apply recommendations", "recommend")}
      ${step("3", "Splunk HEC", "Optional observability setup with safe sample-event testing.", "Test HEC", "hec")}
      ${step("4", "Launch", "Mark setup complete and start using the endpoint.", "Complete setup", "complete")}
    </div>
  `;
  action("scan", () => api("/wrangler/setup/scan-local", { method: "POST" }).then(load));
  action("recommend", () => api("/wrangler/setup/apply-recommended", { method: "POST" }).then(load));
  action("hec", () => api("/wrangler/setup/test-hec", { method: "POST", body: JSON.stringify(state.config.telemetry.splunk_hec) }).then(() => alert("HEC test sent")));
  action("complete", () => api("/wrangler/setup/complete", { method: "POST" }).then((result) => {
    if (result.admin_token) {
      adminToken = result.admin_token;
      localStorage.setItem("llama_wrangler_admin_token", adminToken);
    }
    if (result.client_api_key) {
      localStorage.setItem("llama_wrangler_last_client_key", result.client_api_key);
      alert(`Setup complete. Save this IDE API key now: ${result.client_api_key}`);
    }
    return load();
  }));
}

function step(number, name, copy, label, id) {
  return `<article class="card"><span class="status">${number}</span><h2>${name}</h2><p>${copy}</p><button class="primary" data-action="${id}">${label}</button></article>`;
}

function renderDashboard() {
  title.textContent = "Dashboard";
  subtitle.textContent = "Fleet status, queue depth, sessions, and safe-default posture.";
  const nodes = Object.values(state.nodes || {});
  const queue = state.queue || {};
  const routing = state.config?.routing || {};
  const scheduling = queue.scheduling || {};
  const weights = scheduling.weights || routing.queue_priority_weights || {};
  const policy = scheduling.policy || routing.queue_scheduling_policy || "weighted_priority";
  const ops = state.operation_stats || {};
  content.innerHTML = `
    <div class="grid">
      <article class="card"><h2>Marshal Endpoint</h2><p><code>http://${location.host}/v1</code></p><p><span class="status">${state.role}</span></p></article>
      <article class="card"><h2>Nodes</h2><p>${nodes.length} active or discovered nodes</p></article>
      <article class="card">
        <h2>Queue</h2>
        <div class="queue-meter"><span style="width:${queuePercent(queue)}%"></span></div>
        <div class="row"><span>Active</span><strong>${queue.active || 0}</strong></div>
        <div class="row"><span>Waiting</span><strong>${queue.waiting || 0}</strong></div>
        <div class="row"><span>Available</span><strong>${queue.available ?? 0} / ${queue.max_depth || 0}</strong></div>
      </article>
      <article class="card">
        <h2>Queue Scheduling</h2>
        <label>Policy<select id="queue-policy">
          <option value="weighted_priority" ${policy === "weighted_priority" ? "selected" : ""}>Weighted priority</option>
          <option value="fifo" ${policy === "fifo" ? "selected" : ""}>FIFO</option>
        </select></label>
        <div class="grid compact-grid">
          <label>High<input id="queue-weight-high" type="number" min="1" max="20" value="${escapeHTML(weights.high || 3)}"></label>
          <label>Normal<input id="queue-weight-normal" type="number" min="1" max="20" value="${escapeHTML(weights.normal || 2)}"></label>
          <label>Low<input id="queue-weight-low" type="number" min="1" max="20" value="${escapeHTML(weights.low || 1)}"></label>
        </div>
        <div class="row"><span>Current policy</span><strong>${escapeHTML(policy)}</strong></div>
        <button data-action="save-queue-scheduling">Save scheduling</button>
      </article>
      <article class="card">
        <h2>Streaming Outcomes</h2>
        <div class="row"><span>Retries</span><strong>${numberText(ops.retries?.total)}</strong></div>
        <div class="row"><span>Partial responses</span><strong>${numberText(ops.partials?.total)}</strong></div>
        <div class="row"><span>Cancellations</span><strong>${numberText(ops.cancellations?.total)}</strong></div>
        <div class="row"><span>No retry after partial</span><strong>${numberText(ops.partials?.after_partial)}</strong></div>
        <div class="row"><span>Cancel after partial</span><strong>${numberText(ops.cancellations?.after_partial_output)}</strong></div>
        <p>${operationLastSeen(ops)}</p>
      </article>
      <article class="card"><h2>Frontier Delta</h2><p><span class="status ${state.safe_defaults.frontier_enabled ? "warn" : ""}">${state.safe_defaults.frontier_enabled ? "enabled" : "disabled"}</span></p></article>
    </div>
    <div class="grid queue-grid">
      <article class="card">
        <h2>Current Queue</h2>
        ${queueEntries(queue.current || [], "No active queue entries.")}
      </article>
      <article class="card">
        <h2>Recent Queue</h2>
        ${queueEntries(queue.recent || [], "No recent queue activity.")}
      </article>
    </div>
  `;
  action("save-queue-scheduling", () => api("/wrangler/routing/policies", { method: "PUT", body: JSON.stringify(queueSchedulingPayload(routing)) }).then(load));
}

function queuePercent(queue) {
  const max = Number(queue?.max_depth || 0);
  if (!max) return 0;
  return Math.min(100, Math.round(((Number(queue.active || 0) + Number(queue.waiting || 0)) / max) * 100));
}

function numberText(value) {
  return String(Number(value || 0));
}

function operationLastSeen(ops) {
  const latest = [ops.retries?.last_at, ops.partials?.last_at, ops.cancellations?.last_at]
    .filter(Boolean)
    .map((value) => new Date(value))
    .filter((date) => !Number.isNaN(date.getTime()))
    .sort((a, b) => b.getTime() - a.getTime())[0];
  return latest ? `Last event ${latest.toLocaleTimeString()}` : "No retry, partial, or cancellation events in the current audit window.";
}

function queueEntries(entries, emptyText) {
  if (!entries.length) return `<p>${emptyText}</p>`;
  return `<div class="queue-list">${entries.slice(0, 8).map((entry) => `
    <div class="queue-entry">
      <div>
        <strong>${escapeHTML(entry.model || entry.surface || "request")}</strong>
        <span>${escapeHTML(entry.surface || "")}</span>
      </div>
      <div class="queue-tags">
        <span class="status ${entry.priority === "high" ? "warn" : ""}">${escapeHTML(entry.priority || "normal")}</span>
        <span class="status ${queueStatusClass(entry.status)}">${escapeHTML(entry.status || "unknown")}</span>
      </div>
      <div class="queue-time">${escapeHTML(formatTime(entry.started_at || entry.enqueued_at || entry.completed_at))}</div>
    </div>`).join("")}</div>`;
}

function queueStatusClass(status) {
  return ["failed", "rejected", "partial", "cancelled"].includes(status) ? "warn" : "";
}

function renderNodes() {
  title.textContent = "Nodes";
  subtitle.textContent = "Capabilities, Ollama health, model inventory, and operational controls.";
  const nodes = Object.values(state.nodes || {});
  content.innerHTML = `
    <article class="card">
      <h2>Add Subscriber</h2>
      <div class="grid">
        <label>Node ID<input id="node-id" placeholder="rtx4090"></label>
        <label>Subscriber URL<input id="node-url" placeholder="http://worker.local:11436"></label>
      </div>
      <p>The marshal will fetch the subscriber capability report when the URL is reachable.</p>
      <button class="primary" data-action="add-node">Add node</button>
    </article>
    <div class="grid">${nodes.map(nodeCard).join("") || `<article class="card"><p>No nodes yet. Run the setup scan.</p></article>`}</div>`;
  action("add-node", () => api("/wrangler/nodes/manual-add", { method: "POST", body: JSON.stringify({ node_id: value("node-id"), url: value("node-url") }) }).then(load));
  document.querySelectorAll("[data-node-action]").forEach((button) => {
    button.addEventListener("click", () => api(`/wrangler/nodes/${button.dataset.node}/${button.dataset.nodeAction}`, { method: "POST" }).then(load));
  });
}

function nodeCard(node) {
  const models = (node.models || []).map((m) => m.name).join(", ") || "No models detected";
  const control = node.control_level || "managed";
  const trust = node.trust_level || "local";
  const approval = node.approval_state || (node.approved ? "approved" : "pending");
  const capability = node.capability_source || "subscriber_reported";
  return `<article class="card">
    <h2>${escapeHTML(node.display_name || node.node_id)}</h2>
    <p class="badge-row">
      <span class="status ${node.status === "healthy" ? "" : "warn"}">${escapeHTML(node.status || "unknown")}</span>
      <span class="status ${control === "passive" ? "warn" : ""}">${escapeHTML(formatNodeMeta(control))}</span>
      <span class="status ${trustBadgeClass(trust)}">${escapeHTML(formatNodeMeta(trust))}</span>
      <span class="status ${approval === "approved" ? "" : "warn"}">${escapeHTML(formatNodeMeta(approval))}</span>
    </p>
    <div class="row"><span>Role</span><strong>${node.role}</strong></div>
    <div class="row"><span>Capability source</span><strong>${escapeHTML(formatNodeMeta(capability))}</strong></div>
    <div class="row"><span>Host</span><strong>${node.hostname || node.url || "local"}</strong></div>
    <div class="row"><span>Platform</span><strong>${node.platform}/${node.arch}</strong></div>
    <div class="row"><span>Ollama</span><strong>${node.ollama_available ? "available" : "not detected"}</strong></div>
    <p>${escapeHTML(models)}</p>
    <button data-node="${node.node_id}" data-node-action="benchmark">Run benchmark</button>
    <button data-node="${node.node_id}" data-node-action="${node.enabled ? "disable" : "enable"}">${node.enabled ? "Disable" : "Enable"}</button>
  </article>`;
}

function formatNodeMeta(value) {
  return String(value || "unknown").replace(/_/g, " ").replace(/\b\w/g, (letter) => letter.toUpperCase());
}

function trustBadgeClass(trust) {
  return ["lan_unverified", "external"].includes(trust) ? "warn" : "";
}

function renderModels() {
  title.textContent = "Models & Aliases";
  subtitle.textContent = "Friendly aliases are the stable names clients use.";
  const aliases = state.config.model_aliases || {};
  content.innerHTML = `<div class="grid">${Object.entries(aliases).map(([name, alias]) => `
    <article class="card"><h2>${name}</h2><p>${alias.strategy || "single"}</p><p>${(alias.candidates || []).map(escapeHTML).join(", ")}</p><p><span class="status">${alias.execution_mode || state.config.routing.default_execution_mode}</span></p></article>
  `).join("")}</div>`;
}

function renderSplunk() {
  title.textContent = "Splunk HEC";
  subtitle.textContent = "Configure observability without hand-authoring payloads.";
  const hec = state.config.telemetry.splunk_hec || {};
  const splunkStatus = state.telemetry?.splunk_hec || {};
  const hasToken = splunkStatus.has_token || false;
  const tlsWarning = splunkTLSWarning(hec, splunkStatus);
  content.innerHTML = `<article class="card">
    <label class="check"><input id="hec-enabled" type="checkbox" ${hec.enabled ? "checked" : ""}> Enable Splunk HEC telemetry</label>
    <label>HEC URL<input id="hec-url" value="${escapeHTML(hec.url || "")}" placeholder="https://splunk.local:8088/services/collector"></label>
    <label>HEC token<input id="hec-token" type="password" placeholder="${hasToken ? "Saved token is set" : "Paste token to save or test"}"></label>
    <label>Index<input id="hec-index" value="${escapeHTML(hec.index || "llama_wrangler")}"></label>
    <label>Source<input id="hec-source" value="${escapeHTML(hec.source || "llama-wrangler")}"></label>
    <label>Sourcetype prefix<input id="hec-prefix" value="${escapeHTML(hec.sourcetype_prefix || "llama_wrangler")}"></label>
    <label class="check"><input id="hec-verify" type="checkbox" ${hec.verify_ssl !== false ? "checked" : ""}> Verify TLS certificates</label>
    ${tlsWarning ? `<p class="warning">${escapeHTML(tlsWarning)}</p>` : ""}
    <p>Disable TLS verification only for trusted self-signed Splunk lab certificates.</p>
    <button data-action="save-splunk">Save settings</button>
    <button class="primary" data-action="test-splunk">Send sample event</button>
  </article>`;
  action("save-splunk", () => api("/wrangler/telemetry/splunk-hec", { method: "PUT", body: JSON.stringify(hecPayload(hec)) }).then(load));
  action("test-splunk", () => api("/wrangler/setup/test-hec", { method: "POST", body: JSON.stringify(hecPayload(hec)) }).then(() => alert("Sample event accepted")));
}

function splunkTLSWarning(hec, status) {
  if (hec.verify_ssl !== false && !status.tls_verification_disabled) return "";
  return status.tls_warning || "TLS certificate verification is disabled. Use this only for trusted self-signed Splunk lab certificates.";
}

function renderIDE() {
  title.textContent = "IDE / Agent Setup";
  subtitle.textContent = "Use one endpoint for OpenAI-compatible or Ollama-compatible clients.";
  const keys = state.client_api_keys || [];
  const lastKey = localStorage.getItem("llama_wrangler_last_client_key") || "";
  const presets = state.client_presets || fallbackClientPresets();
  content.innerHTML = `<div class="grid preset-grid">
    ${presets.map((preset, index) => clientPresetCard(preset, lastKey, index)).join("")}
    <article class="card">
      <h2>Client API Keys</h2>
      <label>Key name<input id="client-key-name" value="local-ide"></label>
      <button class="primary" data-action="create-client-key">Generate API key</button>
      <p>Generated keys are shown once. Store them in your IDE or agent config.</p>
      ${lastKey ? `<pre>${escapeHTML(lastKey)}</pre>` : ""}
      <div>${keys.map((key) => `<div class="row"><span>${escapeHTML(key.name)} ${key.enabled ? "" : "(revoked)"}</span><strong>${escapeHTML(key.hint)}</strong></div><div class="row"><button data-key-id="${escapeHTML(key.id)}" data-key-action="rotate">Regenerate</button><button class="danger" data-key-id="${escapeHTML(key.id)}" data-key-action="revoke">Revoke</button></div>`).join("") || "<p>No client keys yet.</p>"}</div>
    </article>
  </div>`;
  action("create-client-key", () => api("/wrangler/auth/api-keys", { method: "POST", body: JSON.stringify({ name: value("client-key-name") }) }).then((result) => {
    localStorage.setItem("llama_wrangler_last_client_key", result.token);
    alert(`Save this API key now: ${result.token}`);
    return load();
  }));
  document.querySelectorAll("[data-key-action]").forEach((button) => {
    button.addEventListener("click", () => api(`/wrangler/auth/api-keys/${button.dataset.keyId}/${button.dataset.keyAction}`, { method: "POST" }).then((result) => {
      if (result.token) {
        localStorage.setItem("llama_wrangler_last_client_key", result.token);
        alert(`Save this regenerated API key now: ${result.token}`);
      }
      return load();
    }).catch((err) => alert(err.message)));
  });
  document.querySelectorAll("[data-copy-preset]").forEach((button) => {
    button.addEventListener("click", () => {
      const preset = presets[Number(button.dataset.copyPreset)];
      copyText(presetSnippetBody(preset, lastKey)).then(() => alert("Preset copied")).catch((err) => alert(err.message));
    });
  });
}

function clientPresetCard(preset, apiKey, index) {
  const fields = preset.fields || [];
  const snippet = presetSnippetBody(preset, apiKey);
  return `<article class="card preset-card">
    <div class="row preset-title"><span class="status">${escapeHTML(preset.protocol || "OpenAI-compatible")}</span><strong>${escapeHTML(preset.name || preset.client || "Client")}</strong></div>
    <div class="preset-fields">
      ${fields.map((field) => `<div class="row"><span>${escapeHTML(field.label)}</span><strong>${escapeHTML(presetFieldValue(preset, field.value, apiKey))}</strong></div>`).join("")}
    </div>
    <pre>${escapeHTML(snippet)}</pre>
    <button data-copy-preset="${index}">Copy preset</button>
  </article>`;
}

function presetFieldValue(preset, value, apiKey) {
  const placeholder = preset?.api_key_placeholder || "<client-api-key>";
  if (value === placeholder && apiKey) return apiKey;
  return value;
}

function presetSnippetBody(preset, apiKey) {
  const snippet = (preset?.snippets || [])[0] || {};
  const placeholder = preset?.api_key_placeholder || "<client-api-key>";
  const body = snippet.body || "";
  return apiKey ? body.split(placeholder).join(apiKey) : body;
}

function fallbackClientPresets() {
  const baseURL = `http://${location.host}/v1`;
  const placeholder = "<client-api-key>";
  const fields = (model) => [
    { label: "Base URL", value: baseURL },
    { label: "Model", value: model },
    { label: "API key", value: placeholder },
  ];
  return [
    {
      id: "cline",
      name: "Cline",
      protocol: "OpenAI-compatible",
      api_key_placeholder: placeholder,
      fields: fields("local-code"),
      snippets: [{ body: JSON.stringify({ provider: "openai-compatible", baseUrl: baseURL, apiKey: placeholder, model: "local-code" }, null, 2) }],
    },
    {
      id: "continue",
      name: "Continue",
      protocol: "OpenAI-compatible",
      api_key_placeholder: placeholder,
      fields: fields("local-code"),
      snippets: [{ body: JSON.stringify({ models: [{ title: "Llama Wrangler local-code", provider: "openai", model: "local-code", apiBase: baseURL, apiKey: placeholder }] }, null, 2) }],
    },
    {
      id: "open-webui",
      name: "Open WebUI",
      protocol: "OpenAI-compatible",
      api_key_placeholder: placeholder,
      fields: fields("local-fast"),
      snippets: [{ body: `OpenAI API Base URL: ${baseURL}\nAPI Key: ${placeholder}\nModel: local-fast` }],
    },
    {
      id: "openai-sdk",
      name: "OpenAI SDK",
      protocol: "OpenAI-compatible",
      api_key_placeholder: placeholder,
      fields: fields("local-fast"),
      snippets: [{ body: `import OpenAI from "openai";\n\nconst client = new OpenAI({\n  apiKey: process.env.LLAMA_WRANGLER_API_KEY || "${placeholder}",\n  baseURL: "${baseURL}",\n});` }],
    },
  ];
}

function renderSettings() {
  title.textContent = "Settings";
  subtitle.textContent = "Local access control and service safety settings.";
  const safe = state.safe_defaults || {};
  const rateLimit = state.auth_rate_limit || {};
  const secretStorage = state.secret_storage || {};
  const lanEnabled = safe.lan_access_enabled || false;
  content.innerHTML = `<div class="grid">
    <article class="card">
      <h2>Admin Access</h2>
      <p>Admin token hint: <code>${escapeHTML(state.admin_token_hint || "not available")}</code></p>
      <button class="primary" data-action="rotate-admin">Rotate admin token</button>
      <button data-action="logout">Logout on this browser</button>
      <p>Rotating the admin token invalidates the previous token immediately.</p>
      <div class="row"><span>Failed auth limit</span><strong>${escapeHTML(rateLimit.max_failures || "")} attempts</strong></div>
      <div class="row"><span>Lockout window</span><strong>${escapeHTML(rateLimit.cooldown_seconds || "")} seconds</strong></div>
    </article>
    <article class="card">
      <h2>Support Bundle</h2>
      <p>Export schema, config, queue, node, and metadata-only audit state for troubleshooting.</p>
      <button data-action="support-bundle">Download support bundle</button>
    </article>
    <article class="card">
      <h2>Secret Storage</h2>
      <div class="row"><span>Backend</span><strong>${escapeHTML(secretStorage.backend || "")}</strong></div>
      <div class="row"><span>Encrypted</span><strong>${secretStorage.encrypted ? "yes" : "no"}</strong></div>
      <div class="row"><span>Key source</span><strong>${escapeHTML(secretStorage.key_source || "")}</strong></div>
      <div class="row"><span>Local rekey</span><strong>${secretStorage.rekey_supported ? "available" : "unavailable"}</strong></div>
      <p>${escapeHTML(secretStorage.rekey_description || "")}</p>
      <h3>Backup & Restore</h3>
      <p>${escapeHTML(secretStorage.backup_description || "Back up encrypted secrets with their matching key source.")}</p>
      ${secretBackupList(secretStorage.backup_required_files || [])}
      <p>${escapeHTML(secretStorage.restore_description || "Restore the encrypted secret file and its matching key source before starting Llama Wrangler.")}</p>
      ${(secretStorage.backup_warnings || []).map((warning) => `<p class="warning">${escapeHTML(warning)}</p>`).join("")}
      <h3>OS Keychain</h3>
      <div class="row"><span>Integration</span><strong>${escapeHTML(secretStorage.os_keychain_status || "planned")}</strong></div>
      <div class="row"><span>Platform</span><strong>${escapeHTML(secretStorage.os_keychain_platform || "unknown")}</strong></div>
      <div class="row"><span>Runtime</span><strong>${escapeHTML(secretStorage.os_keychain_runtime || "unknown")}</strong></div>
      <div class="row"><span>Fallback</span><strong>${secretStorage.fallback_available ? escapeHTML(secretStorage.fallback_backend || "encrypted_file") : "unavailable"}</strong></div>
      ${secretStorage.os_keychain_migrated ? `<div class="row"><span>Migrated values</span><strong>${escapeHTML(secretStorage.os_keychain_migrated)}</strong></div>` : ""}
      ${secretStorage.os_keychain_warning ? `<p class="warning">${escapeHTML(secretStorage.os_keychain_warning)}</p>` : ""}
      <p>${escapeHTML(secretStorage.os_keychain_next_step || "Encrypted fallback remains active until an OS keychain backend is added and verified.")}</p>
      <button data-action="rekey-secrets" ${secretStorage.rekey_supported ? "" : "disabled"}>Rotate fallback key</button>
    </article>
    <article class="card">
      <h2>Network Exposure</h2>
      <p><span class="status ${lanEnabled ? "warn" : ""}">${lanEnabled ? "LAN exposed" : "localhost only"}</span></p>
      <div class="row"><span>Listen address</span><strong>${escapeHTML(safe.marshal_listen || "")}</strong></div>
      <div class="row"><span>LAN by default</span><strong>${safe.lan_access_by_default ? "yes" : "no"}</strong></div>
      <div class="row"><span>Explicit enablement</span><strong>${safe.lan_requires_explicit_enablement ? "required" : "unknown"}</strong></div>
      ${safe.lan_access_warning ? `<p class="warning">${escapeHTML(safe.lan_access_warning)}</p>` : `<p>Localhost binding is the active safe default.</p>`}
    </article>
    <article class="card">
      <h2>Safe Defaults</h2>
      <div class="row"><span>State schema</span><strong>${escapeHTML(state.schema_version || "")}</strong></div>
      <div class="row"><span>Config version</span><strong>${escapeHTML(state.config_version || "")}</strong></div>
      <div class="row"><span>Local-only mode</span><strong>${safe.local_only ? "enabled" : "disabled"}</strong></div>
      <div class="row"><span>Frontier Delta</span><strong>${safe.frontier_enabled ? "enabled" : "disabled"}</strong></div>
      <div class="row"><span>Payload logging</span><strong>${safe.prompt_body_logging ? "enabled" : "disabled"}</strong></div>
    </article>
  </div>`;
  action("rotate-admin", () => api("/wrangler/auth/admin-token/rotate", { method: "POST" }).then((result) => {
    adminToken = result.token;
    localStorage.setItem("llama_wrangler_admin_token", adminToken);
    alert(`Save this new admin token now: ${result.token}`);
    return load();
  }));
  action("logout", () => {
    adminToken = "";
    localStorage.removeItem("llama_wrangler_admin_token");
    localStorage.removeItem("llama_wrangler_last_client_key");
    return load();
  });
  action("support-bundle", () => api("/wrangler/support-bundle/export", { method: "POST" }).then(downloadSupportBundle));
  action("rekey-secrets", () => {
    if (!confirm("Rotate the local encrypted fallback key now? Stored secrets will stay encrypted and remain usable.")) return Promise.resolve();
    return api("/wrangler/secrets/rekey", { method: "POST" }).then((result) => {
      alert(`Secret storage rekeyed: ${result.status?.backend || "encrypted_file"}`);
      return load();
    });
  });
}

function secretBackupList(files) {
  if (!files.length) return "";
  return `<div class="queue-tags">${files.map((file) => `<span class="status">${escapeHTML(file)}</span>`).join("")}</div>`;
}

function renderAudit() {
  title.textContent = "Logs & Audit";
  subtitle.textContent = "Metadata-only request trail. Prompt and response bodies stay out by default.";
  const rows = (state.config ? [] : []);
  api("/wrangler/audit/recent").then((audit) => {
    content.innerHTML = `<div class="card">${audit.slice(0, 80).map((event) => `<div class="row"><span>${new Date(event.timestamp).toLocaleString()}</span><strong>${event.type}</strong></div>`).join("") || "<p>No audit events yet.</p>"}</div>`;
  });
}

function action(id, handler) {
  const el = document.querySelector(`[data-action="${id}"]`);
  if (el) el.addEventListener("click", () => handler().catch((err) => alert(err.message)));
}

function value(id) {
  return document.querySelector(`#${id}`).value;
}

function checked(id) {
  return document.querySelector(`#${id}`).checked;
}

function numberValue(id, fallback) {
  const n = Number(value(id));
  return Number.isFinite(n) && n > 0 ? Math.round(n) : fallback;
}

function queueSchedulingPayload(existing) {
  return {
    ...existing,
    queue_scheduling_policy: value("queue-policy"),
    queue_priority_weights: {
      high: numberValue("queue-weight-high", 3),
      normal: numberValue("queue-weight-normal", 2),
      low: numberValue("queue-weight-low", 1),
    },
  };
}

function hecPayload(existing) {
  return {
    ...existing,
    enabled: checked("hec-enabled"),
    url: value("hec-url"),
    token: value("hec-token"),
    index: value("hec-index"),
    source: value("hec-source"),
    sourcetype_prefix: value("hec-prefix"),
    verify_ssl: checked("hec-verify"),
  };
}

function formatTime(value) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleTimeString();
}

function downloadSupportBundle(bundle) {
  const stamp = new Date().toISOString().replace(/[:.]/g, "-");
  const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `llama-wrangler-support-${stamp}.json`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

async function copyText(text) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "readonly");
  textarea.style.position = "fixed";
  textarea.style.left = "-9999px";
  document.body.appendChild(textarea);
  textarea.select();
  const copied = document.execCommand("copy");
  textarea.remove();
  if (!copied) throw new Error("Clipboard copy failed");
}

function escapeHTML(value) {
  return String(value || "").replace(/[&<>"']/g, (ch) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#039;" }[ch]));
}

load().catch((err) => {
  content.innerHTML = `<article class="card"><h2>Could not load Llama Wrangler</h2><p>${escapeHTML(err.message)}</p></article>`;
});
