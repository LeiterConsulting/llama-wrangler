let state = {};
let view = "wizard";
let adminToken = localStorage.getItem("llama_wrangler_admin_token") || "";
let lastPeerDiscovery = null;
let lastCredentialInstall = null;

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
      ${step("2", "Safe LAN discovery", "Run one-shot mDNS discovery for Llama Wrangler or Ollama services. Subnet scanning stays disabled.", "Discover peers", "discover")}
      ${step("3", "Recommended setup", "Apply local-only, metadata-only telemetry, soft sessions, and default aliases.", "Apply recommendations", "recommend")}
      ${step("4", "Splunk HEC", "Optional observability setup with safe sample-event testing.", "Test HEC", "hec")}
      ${step("5", "Launch", "Mark setup complete and start using the endpoint.", "Complete setup", "complete")}
    </div>
    ${peerDiscoveryResultCard()}
  `;
  action("scan", () => api("/wrangler/setup/scan-local", { method: "POST" }).then(load));
  action("discover", () => api("/wrangler/setup/discover-peers", { method: "POST" }).then((result) => {
    lastPeerDiscovery = result;
    render();
  }));
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

function peerDiscoveryResultCard() {
  if (!lastPeerDiscovery) return "";
  const candidates = lastPeerDiscovery.candidates || [];
  return `<article class="card">
    <h2>Peer Discovery Results</h2>
    <div class="row"><span>mDNS</span><strong>${escapeHTML(formatNodeMeta(lastPeerDiscovery.mdns?.status || "unknown"))}</strong></div>
    <div class="row"><span>Candidates</span><strong>${numberText(candidates.length)}</strong></div>
    <div class="row"><span>Subnet scan</span><strong>${escapeHTML(formatNodeMeta(lastPeerDiscovery.subnet_scan?.status || "disabled"))}</strong></div>
    <p>${escapeHTML(lastPeerDiscovery.mdns?.message || "Discovery completed.")}</p>
    <p class="warning">Candidates are not saved, approved, or routed automatically. Use Managed Node enrollment or Add existing Ollama endpoint to adopt one.</p>
    ${candidates.length ? `<div class="warning-list">${candidates.slice(0, 6).map(peerDiscoveryCandidateRow).join("")}</div>` : ""}
  </article>`;
}

function peerDiscoveryCandidateRow(candidate) {
  return `<div class="warning-item">
    <div>
      <strong>${escapeHTML(candidate.display_name || candidate.instance || "Discovered candidate")}</strong>
      <span>${escapeHTML(candidate.endpoint_url || candidate.host || candidate.service || "No endpoint URL advertised")}</span>
    </div>
    <p class="badge-row">
      <span class="status">${escapeHTML(formatNodeMeta(candidate.adoption_path || "review"))}</span>
      <span class="status warn">${escapeHTML(formatNodeMeta(candidate.trust_level || "lan_unverified"))}</span>
      <span class="status">${escapeHTML(formatNodeMeta(candidate.approval_state || "not_added"))}</span>
    </p>
  </div>`;
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
      ${consensusOperationsCard(ops.consensus || {})}
      ${routingPolicyWarningsCard()}
      ${benchmarkPolicyCard()}
      ${benchmarkSchedulerCard()}
      ${benchmarkWorkloadCard()}
      ${benchmarkRunnerCard()}
      ${modelLifecycleCard()}
      ${modelLifecycleActionHistoryCard()}
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
  action("save-benchmark-scheduler-policy", () => api("/wrangler/benchmarks/scheduler/policy", { method: "PUT", body: JSON.stringify(benchmarkSchedulerPolicyPayload()) }).then(load));
  action("reconcile-benchmark-scheduler", () => api("/wrangler/benchmarks/scheduler/reconcile", { method: "POST" }).then(load));
  action("filter-model-action-history", () => api(modelLifecycleActionHistoryQuery()).then((result) => {
    state.model_lifecycle_action_history = result;
    render();
  }));
  action("reset-model-action-history", () => api("/wrangler/models/lifecycle/action-history?limit=20").then((result) => {
    state.model_lifecycle_action_history = result;
    render();
  }));
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

function consensusOperationsCard(consensus) {
  const failureReasons = consensus.failure_reasons || {};
  return `<article class="card">
    <h2>Consensus Outcomes</h2>
    <div class="row"><span>Runs</span><strong>${numberText(consensus.total)}</strong></div>
    <div class="row"><span>Reached</span><strong>${numberText(consensus.reached)}</strong></div>
    <div class="row"><span>No majority</span><strong>${numberText(consensus.no_majority)}</strong></div>
    <div class="row"><span>Failed / timed out</span><strong>${numberText(consensus.failed)} / ${numberText(consensus.timed_out)}</strong></div>
    <div class="row"><span>Cancelled</span><strong>${numberText(consensus.cancelled)}</strong></div>
    <div class="row"><span>Streaming rejected</span><strong>${numberText(consensus.streaming_rejected)}</strong></div>
    <div class="row"><span>Last participants</span><strong>${numberText(consensus.last_successful_count)} / ${numberText(consensus.last_participant_count)}</strong></div>
    <div class="row"><span>Last agreement</span><strong>${consensus.last_agreement_score ? `${Math.round(Number(consensus.last_agreement_score) * 100)}%` : "None"}</strong></div>
    <div class="row"><span>Last winner</span><strong>${escapeHTML(consensus.last_winner_node || "None")}</strong></div>
    ${Object.keys(failureReasons).length ? `<div class="queue-tags">${Object.entries(failureReasons).map(([reason, count]) => `<span class="status warn">${escapeHTML(formatNodeMeta(reason))} ${numberText(count)}</span>`).join("")}</div>` : `<p>No participant failure reasons in the current audit window.</p>`}
  </article>`;
}

function routingPolicyWarningsCard() {
  const status = state.routing_policy_status || {};
  const warnings = status.warnings || [];
  if (!warnings.length) {
    return `<article class="card">
      <h2>Routing Policy</h2>
      <p><span class="status">No current policy warnings</span></p>
      <p>Approved healthy nodes match the current routing and consensus guardrails.</p>
    </article>`;
  }
  return `<article class="card">
    <h2>Routing Policy Warnings</h2>
    <p>${escapeHTML(routingPolicySummary(status.summary || {}, warnings.length))}</p>
    <div class="warning-list">${warnings.slice(0, 10).map(routingPolicyWarningRow).join("")}</div>
  </article>`;
}

function routingPolicySummary(summary, count) {
  const blocked = Number(summary.blocked || 0);
  const limited = Number(summary.limited || 0);
  return `${count} current warning${count === 1 ? "" : "s"}: ${blocked} blocked, ${limited} limited.`;
}

function routingPolicyWarningRow(warning) {
  return `<div class="warning-item">
    <div>
      <strong>${escapeHTML(warning.node_id || "unknown node")}</strong>
      <span>${escapeHTML(warning.message || formatNodeMeta(warning.code || "policy warning"))}</span>
    </div>
    <p class="badge-row">
      <span class="status ${warning.severity === "blocked" ? "warn" : ""}">${escapeHTML(formatNodeMeta(warning.severity || "warning"))}</span>
      <span class="status">${escapeHTML(formatNodeMeta(warning.scope || "routing"))}</span>
      <span class="status ${trustBadgeClass(warning.trust_level)}">${escapeHTML(formatNodeMeta(warning.trust_level || "unknown"))}</span>
      <span class="status ${warning.approval_state === "approved" ? "" : "warn"}">${escapeHTML(formatNodeMeta(warning.approval_state || "unknown"))}</span>
    </p>
  </div>`;
}

function benchmarkPolicyCard() {
  const status = state.benchmark_policy_status || {};
  const nodes = status.nodes || [];
  if (!nodes.length) {
    return `<article class="card"><h2>Benchmark Policy</h2><p>No nodes available for benchmark policy.</p></article>`;
  }
  return `<article class="card">
    <h2>Benchmark Policy</h2>
    <div class="row"><span>Eligible managed nodes</span><strong>${numberText(status.summary?.eligible)}</strong></div>
    <div class="row"><span>Placement-ready summaries</span><strong>${numberText(status.summary?.placement_eligible)}</strong></div>
    <div class="row"><span>Probe-only endpoints</span><strong>${numberText(status.summary?.marshal_observed_probe_only)}</strong></div>
    <div class="warning-list">${nodes.slice(0, 8).map(benchmarkPolicyRow).join("")}</div>
  </article>`;
}

function benchmarkPolicyRow(item) {
  return `<div class="warning-item">
    <div>
      <strong>${escapeHTML(item.node_id || "unknown node")}</strong>
      <span>${escapeHTML(item.message || "Benchmark policy pending.")}</span>
      <span>${escapeHTML(item.placement_message || "Benchmark placement metadata pending.")}</span>
    </div>
    <p class="badge-row">
      <span class="status ${item.eligible ? "" : "warn"}">${item.eligible ? "Eligible" : "Limited"}</span>
      <span class="status ${item.placement_eligible ? "" : "warn"}">${item.placement_eligible ? "Placement ready" : "Placement limited"}</span>
      <span class="status">${escapeHTML(formatNodeMeta(item.mode || "unknown"))}</span>
      <span class="status">${escapeHTML(formatNodeMeta(item.benchmark_source || "none"))}</span>
    </p>
  </div>`;
}

function benchmarkSchedulerCard() {
  const scheduler = state.benchmark_scheduler || {};
  const summary = scheduler.summary || {};
  const config = scheduler.config || {};
  const limits = scheduler.limits || {};
  const background = scheduler.background || {};
  const history = scheduler.history || {};
  const historySummary = history.summary || {};
  const historyEntries = history.entries || [];
  const jobs = scheduler.jobs || [];
  return `<article class="card">
    <h2>Benchmark Scheduler</h2>
    <div class="row"><span>Policy</span><strong>${escapeHTML(formatNodeMeta(scheduler.policy || "bounded_retry_timeout_v1"))}</strong></div>
    <label class="check-row"><input id="benchmark-background-enabled" type="checkbox" ${config.background_enabled ? "checked" : ""}> Background reconciliation</label>
    <div class="grid compact-grid">
      <label>Max attempts<input id="benchmark-max-attempts" type="number" min="${escapeHTML(limits.max_attempts_min || 1)}" max="${escapeHTML(limits.max_attempts_max || 10)}" value="${escapeHTML(config.max_attempts || 3)}"></label>
      <label>Lease seconds<input id="benchmark-lease-timeout" type="number" min="${escapeHTML(limits.lease_timeout_seconds_min || 30)}" max="${escapeHTML(limits.lease_timeout_seconds_max || 3600)}" value="${escapeHTML(config.lease_timeout_seconds || 600)}"></label>
      <label>Retry seconds<input id="benchmark-retry-delay" type="number" min="${escapeHTML(limits.retry_delay_seconds_min || 5)}" max="${escapeHTML(limits.retry_delay_seconds_max || 1800)}" value="${escapeHTML(config.retry_delay_seconds || 60)}"></label>
      <label>Tick seconds<input id="benchmark-tick-interval" type="number" min="${escapeHTML(limits.tick_interval_seconds_min || 10)}" max="${escapeHTML(limits.tick_interval_seconds_max || 3600)}" value="${escapeHTML(config.tick_interval_seconds || 60)}"></label>
    </div>
    <div class="row"><span>Background tick</span><strong>${background.enabled ? "Enabled" : "Disabled"}</strong></div>
    <div class="row"><span>Next tick</span><strong>${escapeHTML(background.next_tick_at ? formatTime(background.next_tick_at) : "Not scheduled")}</strong></div>
    <div class="row"><span>Last tick</span><strong>${escapeHTML(background.last_tick_at ? formatTime(background.last_tick_at) : "Never")}</strong></div>
    <div class="row"><span>Last tick changes</span><strong>${background.last_changed ? "Changed jobs" : "No changes"}</strong></div>
    <div class="row"><span>Recent reconciliations</span><strong>${numberText(history.count)}</strong></div>
    <div class="row"><span>Recent timeouts / retries</span><strong>${numberText(historySummary.timed_out)} / ${numberText(historySummary.retried)}</strong></div>
    <div class="row"><span>Audit retention</span><strong>${escapeHTML(formatNodeMeta(history.retention || "process_local_reset_on_restart"))}</strong></div>
    <div class="row"><span>Jobs</span><strong>${numberText(summary.jobs)}</strong></div>
    <div class="row"><span>Claimable</span><strong>${numberText(summary.claimable)}</strong></div>
    <div class="row"><span>Retry wait</span><strong>${numberText(summary.retry_wait)}</strong></div>
    <div class="row"><span>Timeout due</span><strong>${numberText(summary.timeout_due)}</strong></div>
    <div class="row"><span>Exhausted</span><strong>${numberText(summary.exhausted)}</strong></div>
    ${jobs.length ? `<div class="warning-list">${jobs.slice(0, 5).map(benchmarkSchedulerRow).join("")}</div>` : `<p>No benchmark jobs are currently tracked.</p>`}
    ${historyEntries.length ? `<h3>Recent reconciliation audit</h3><div class="warning-list">${historyEntries.slice(0, 6).map(benchmarkSchedulerAuditRow).join("")}</div>` : `<p>No scheduler reconciliations recorded in this service run.</p>`}
    <button data-action="save-benchmark-scheduler-policy">Save scheduler policy</button>
    <button data-action="reconcile-benchmark-scheduler">Reconcile scheduler</button>
  </article>`;
}

function benchmarkSchedulerAuditRow(entry) {
  const changes = entry.changed ? "Jobs changed" : "No changes";
  return `<div class="warning-item">
    <div>
      <strong>${escapeHTML(formatNodeMeta(entry.trigger || "internal"))} · ${escapeHTML(formatTime(entry.recorded_at))}</strong>
      <span>${escapeHTML(formatNodeMeta(entry.reason || "internal_reconcile"))}</span>
    </div>
    <p class="badge-row">
      <span class="status ${entry.changed ? "warn" : ""}">${escapeHTML(changes)}</span>
      <span class="status ${Number(entry.timed_out || 0) ? "warn" : ""}">Timed out ${numberText(entry.timed_out)}</span>
      <span class="status">Retried ${numberText(entry.retried)}</span>
      <span class="status ${Number(entry.exhausted || 0) ? "warn" : ""}">Exhausted ${numberText(entry.exhausted)}</span>
    </p>
  </div>`;
}

function benchmarkSchedulerRow(job) {
  const attempt = `${numberText(job.attempt)} / ${numberText(job.max_attempts)}`;
  const due = job.next_attempt_at ? `Next ${formatTime(job.next_attempt_at)}` : (job.timeout_at ? `Timeout ${formatTime(job.timeout_at)}` : "No deadline");
  const suite = job.workload_suite_id ? `Suite ${formatNodeMeta(job.workload_suite_id)}` : "Default suite";
  return `<div class="warning-item">
    <div>
      <strong>${escapeHTML(job.node_id || "managed node")}</strong>
      <span>${escapeHTML(job.benchmark_id || "benchmark job")}</span>
    </div>
    <p class="badge-row">
      <span class="status ${["failed", "exhausted"].includes(job.scheduler_state) ? "warn" : ""}">${escapeHTML(formatNodeMeta(job.status || "unknown"))}</span>
      <span class="status ${job.scheduler_state === "exhausted" ? "warn" : ""}">${escapeHTML(formatNodeMeta(job.scheduler_state || "unknown"))}</span>
      <span class="status">Attempt ${escapeHTML(attempt)}</span>
      <span class="status">${escapeHTML(due)}</span>
      <span class="status">${escapeHTML(suite)}</span>
    </p>
  </div>`;
}

function benchmarkWorkloadCard() {
  const workload = state.benchmark_workload || {};
  const suites = state.benchmark_workload_suites || [];
  return `<article class="card">
    <h2>Benchmark Suites</h2>
    <div class="row"><span>Default</span><strong>${escapeHTML(formatNodeMeta(workload.default_suite_id || "synthetic_smoke_v1"))}</strong></div>
    <div class="row"><span>Suites</span><strong>${numberText(workload.suite_count || suites.length)}</strong></div>
    <div class="row"><span>Storage</span><strong>${escapeHTML(formatNodeMeta(workload.content_storage || "prompt_and_response_bodies_excluded"))}</strong></div>
    <p>Managed Node jobs carry suite IDs and task metadata only. Fixture contents stay local to the subscriber.</p>
    <div class="queue-tags">${suites.map((suite) => `<span class="status">${escapeHTML(suite.display_name || suite.id)}</span>`).join("")}</div>
  </article>`;
}

function benchmarkRunnerCard() {
  const runner = state.benchmark_runner || {};
  const fixture = runner.local_fixture_guidance || {};
  const hooks = runner.packaging_hooks || {};
  const runnerConfig = runner.runner_config || {};
  const suites = runner.supported_suite_ids || [];
  return `<article class="card">
    <h2>Subscriber Benchmark Runner</h2>
    <div class="row"><span>Status</span><strong>${escapeHTML(formatNodeMeta(runner.status || "disabled_by_default"))}</strong></div>
    <div class="row"><span>Control</span><strong>${escapeHTML(formatNodeMeta(runner.control_level || "managed_nodes_only"))}</strong></div>
    <div class="row"><span>Loop hook</span><strong>${escapeHTML(formatNodeMeta(hooks.runner_loop_status || "available_opt_in_dry_run_loop"))}</strong></div>
    <div class="row"><span>Mode</span><strong>${escapeHTML(formatNodeMeta(runnerConfig.mode || "dry_run_v1"))}</strong></div>
    <div class="row"><span>Poll</span><strong>${numberText(runnerConfig.poll_interval_seconds || 60)}s</strong></div>
    <div class="row"><span>Max jobs/tick</span><strong>${numberText(runnerConfig.max_jobs_per_tick || 1)}</strong></div>
    <div class="row"><span>Result policy</span><strong>${escapeHTML(formatNodeMeta(runnerConfig.result_body_policy || "metrics_only"))}</strong></div>
    <div class="row"><span>Fixture storage</span><strong>${escapeHTML(formatNodeMeta(fixture.storage_policy || "fixture_contents_and_full_paths_stay_on_subscriber"))}</strong></div>
    <div class="queue-tags">${suites.map((suite) => `<span class="status">${escapeHTML(formatNodeMeta(suite))}</span>`).join("")}</div>
    <p class="warning">The opt-in subscriber loop claims Managed Node jobs and reports metric summaries only. Dry-run is the default; synthetic built-in mode runs packaged prompts locally and discards responses before reporting.</p>
  </article>`;
}

function modelLifecycleCard() {
  const lifecycle = state.model_lifecycle || {};
  const actionPolicy = state.model_lifecycle_actions || {};
  const summary = lifecycle.summary || {};
  const actionSummary = actionPolicy.summary || {};
  const nodes = lifecycle.nodes || [];
  return `<article class="card">
    <h2>Model Lifecycle</h2>
    <div class="row"><span>Models</span><strong>${numberText(summary.models)}</strong></div>
    <div class="row"><span>Warm</span><strong>${numberText(summary.warm_models)}</strong></div>
    <div class="row"><span>Keep-warm</span><strong>${numberText(summary.keep_warm_models)}</strong></div>
    <div class="row"><span>Action-ready nodes</span><strong>${numberText(actionSummary.eligible_nodes)}</strong></div>
    <div class="row"><span>Blocked action nodes</span><strong>${numberText(actionSummary.blocked_nodes)}</strong></div>
    <div class="row"><span>Pending actions</span><strong>${numberText(actionSummary.pending_model_lifecycle_actions)}</strong></div>
    <div class="row"><span>Inventory-only endpoints</span><strong>${numberText(summary.passive_inventory_only_nodes)}</strong></div>
    <div class="warning-list">${nodes.slice(0, 6).map(modelLifecycleRow).join("")}</div>
  </article>`;
}

function modelLifecycleRow(item) {
  return `<div class="warning-item">
    <div>
      <strong>${escapeHTML(item.node_id || "node")}</strong>
      <span>${escapeHTML(item.message || "Model lifecycle metadata pending.")}</span>
    </div>
    <p class="badge-row">
      <span class="status ${item.control_level === "passive" ? "warn" : ""}">${escapeHTML(formatNodeMeta(item.mode || "unknown"))}</span>
      <span class="status">${numberText(item.model_count)} models</span>
      <span class="status">${numberText(item.warm_count)} warm</span>
      <span class="status">${numberText(item.keep_warm_count)} keep-warm</span>
    </p>
  </div>`;
}

function modelLifecycleActionHistoryCard() {
  const history = state.model_lifecycle_action_history || {};
  const filters = history.filters || {};
  const summary = history.summary || {};
  const actions = history.actions || [];
  const managedNodes = (state.model_lifecycle_actions?.nodes || []).filter((item) => item.control_level === "managed");
  const statuses = ["queued", "running", "completed", "failed", "cancelled"];
  return `<article class="card">
    <h2>Model Action History</h2>
    <div class="grid compact-grid">
      <label>Status<select id="model-action-history-status">
        <option value="" ${filters.status ? "" : "selected"}>All statuses</option>
        ${statuses.map((status) => `<option value="${status}" ${filters.status === status ? "selected" : ""}>${escapeHTML(formatNodeMeta(status))}</option>`).join("")}
      </select></label>
      <label>Managed Node<select id="model-action-history-node">
        <option value="" ${filters.node_id ? "" : "selected"}>All managed nodes</option>
        ${managedNodes.map((item) => `<option value="${escapeHTML(item.node_id)}" ${filters.node_id === item.node_id ? "selected" : ""}>${escapeHTML(item.node_id)}</option>`).join("")}
      </select></label>
    </div>
    <div class="row"><span>Matching actions</span><strong>${numberText(history.total_matches)}</strong></div>
    <div class="row"><span>Queued / running</span><strong>${numberText(summary.queued)} / ${numberText(summary.running)}</strong></div>
    <div class="row"><span>Completed / failed</span><strong>${numberText(summary.completed)} / ${numberText(summary.failed)}</strong></div>
    <div class="row"><span>With safe error code</span><strong>${numberText(summary.with_error_code)}</strong></div>
    <div class="button-row">
      <button data-action="filter-model-action-history">Apply filters</button>
      <button data-action="reset-model-action-history">Reset</button>
    </div>
    ${actions.length ? `<div class="warning-list">${actions.slice(0, 8).map(modelLifecycleActionHistoryRow).join("")}</div>` : `<p>No model lifecycle actions match the current filters.</p>`}
  </article>`;
}

function modelLifecycleActionHistoryQuery() {
  const params = new URLSearchParams({ limit: "20" });
  const status = value("model-action-history-status");
  const nodeID = value("model-action-history-node");
  if (status) params.set("status", status);
  if (nodeID) params.set("node_id", nodeID);
  return `/wrangler/models/lifecycle/action-history?${params.toString()}`;
}

function modelLifecycleActionHistoryRow(item) {
  const desired = item.desired_keep_warm ? "Keep warm" : "Clear keep-warm";
  const outcomeAt = item.completed_at || item.failed_at || item.updated_at || item.claimed_at || item.requested_at;
  const timestamps = [
    item.requested_at ? `Queued ${formatTime(item.requested_at)}` : "",
    item.claimed_at ? `Claimed ${formatTime(item.claimed_at)}` : "",
    item.completed_at ? `Completed ${formatTime(item.completed_at)}` : "",
    item.failed_at ? `Failed ${formatTime(item.failed_at)}` : "",
  ].filter(Boolean).join(" · ");
  return `<div class="warning-item">
    <div>
      <strong>${escapeHTML(item.node_id || "managed node")} · ${escapeHTML(item.model || "model")}</strong>
      <span>${escapeHTML(item.action_id || "model action")} · ${escapeHTML(desired)}</span>
      <span>${escapeHTML(timestamps || (outcomeAt ? formatTime(outcomeAt) : "No action timestamp"))}</span>
    </div>
    <p class="badge-row">
      <span class="status ${["failed", "cancelled"].includes(item.status) ? "warn" : ""}">${escapeHTML(formatNodeMeta(item.status || "unknown"))}</span>
      <span class="status">${escapeHTML(formatNodeMeta(item.action_type || "keep_warm"))}</span>
      ${item.error_code ? `<span class="status warn">${escapeHTML(formatNodeMeta(item.error_code))}</span>` : ""}
    </p>
  </div>`;
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
  const enrollmentQueue = state.enrollment_queue || [];
  content.innerHTML = `
    ${credentialInstallCard()}
    <article class="card">
      <h2>Install/enroll Wrangler subscriber</h2>
      <div class="grid">
        <label>Node ID<input id="enroll-node-id" placeholder="rtx4090"></label>
        <label>Subscriber URL<input id="enroll-subscriber-url" placeholder="http://worker.local:11436"></label>
        <label>Trust level<select id="enroll-trust-level">
          <option value="lan_unverified">LAN unverified</option>
          <option value="lan_trusted">LAN trusted</option>
          <option value="local">Local</option>
          <option value="external">External</option>
        </select></label>
        <label>Token TTL minutes<input id="enroll-ttl" type="number" min="1" max="1440" value="15"></label>
      </div>
      <p>The token is shown once. Subscribers register as pending Managed Nodes and must be approved before routing.</p>
      <button class="primary" data-action="create-enrollment-token">Generate token</button>
      <p class="warning">Manual subscriber add is a compatibility fallback. It creates a pending Managed Node; approve it only after you trust the subscriber and understand it did not use a one-time enrollment token.</p>
      <div class="grid">
        <label>Node ID<input id="node-id" placeholder="manual-rtx4090"></label>
        <label>Subscriber URL<input id="node-url" placeholder="http://worker.local:11436"></label>
      </div>
      <button data-action="add-node">Add pending node</button>
    </article>
    ${enrollmentQueue.length ? `<article class="card"><h2>Managed enrollment queue</h2>${enrollmentQueue.map(enrollmentQueueRow).join("")}</article>` : ""}
    <article class="card">
      <h2>Add existing Ollama endpoint</h2>
      <div class="grid">
        <label>Display name<input id="passive-display-name" placeholder="Studio Ollama"></label>
        <label>Endpoint URL<input id="passive-endpoint-url" placeholder="http://studio.local:11434"></label>
        <label>Trust level<select id="passive-trust-level">
          <option value="lan_unverified">LAN unverified</option>
          <option value="lan_trusted">LAN trusted</option>
          <option value="local">Local</option>
          <option value="external">External</option>
        </select></label>
      </div>
      <p class="warning">Passive Endpoints are limited-control routes. Llama Wrangler validates /api/tags and records marshal-observed metadata, but cannot inspect hardware, warm state, local load, or manage models.</p>
      <button class="primary" data-action="add-passive-endpoint">Add endpoint</button>
    </article>
    ${routingPolicyWarningsCard()}
    <div class="grid">${nodes.map(nodeCard).join("") || `<article class="card"><p>No nodes yet. Run the setup scan.</p></article>`}</div>`;
  action("create-enrollment-token", () => api("/wrangler/enrollment-tokens", {
    method: "POST",
    body: JSON.stringify({
      node_id: value("enroll-node-id"),
      subscriber_url: value("enroll-subscriber-url"),
      trust_level: value("enroll-trust-level"),
      ttl_minutes: Number(value("enroll-ttl") || 15),
    }),
  }).then((result) => {
    alert(`Save this enrollment token now: ${result.token}`);
    return load();
  }));
  action("add-node", () => api("/wrangler/nodes/manual-add", { method: "POST", body: JSON.stringify({ node_id: value("node-id"), url: value("node-url") }) }).then(load));
  action("add-passive-endpoint", () => api("/wrangler/nodes/passive-add", {
    method: "POST",
    body: JSON.stringify({
      display_name: value("passive-display-name"),
      endpoint_url: value("passive-endpoint-url"),
      trust_level: value("passive-trust-level"),
    }),
  }).then(load));
  document.querySelectorAll("[data-node-action]").forEach((button) => {
    button.addEventListener("click", () => {
      const options = { method: "POST" };
      if (button.dataset.nodeAction === "benchmark") {
        options.body = JSON.stringify(benchmarkJobPayload(button));
      }
      return api(`/wrangler/nodes/${button.dataset.node}/${button.dataset.nodeAction}`, options).then(load);
    });
  });
  document.querySelectorAll("[data-node-heartbeat-credential-action]").forEach((button) => {
    button.addEventListener("click", () => api(`/wrangler/nodes/${button.dataset.node}/heartbeat-credential/rotate`, { method: "POST" }).then((result) => {
      lastCredentialInstall = {
        node_id: button.dataset.node,
        credential: result.credential,
        plan: result.subscriber_install || {},
      };
      return load();
    }));
  });
  document.querySelectorAll("[data-node-model-action]").forEach((button) => {
    button.addEventListener("click", () => api(`/wrangler/nodes/${button.dataset.node}/model-actions/keep-warm`, {
      method: "POST",
      body: JSON.stringify({
        model_name: button.dataset.modelName,
        keep_warm: button.dataset.keepWarm === "true",
      }),
    }).then(load));
  });
  document.querySelectorAll("[data-copy-install]").forEach((button) => {
    button.addEventListener("click", () => {
      const text = credentialInstallText(button.dataset.copyInstall);
      return copyText(text).then(() => alert("Copied")).catch((err) => alert(err.message));
    });
  });
  document.querySelectorAll("[data-node-trust-action]").forEach((button) => {
    button.addEventListener("click", () => {
      const select = button.closest("article").querySelector("[data-node-trust-select]");
      return api(`/wrangler/nodes/${button.dataset.node}/trust`, {
        method: "POST",
        body: JSON.stringify({ trust_level: select ? select.value : "" }),
      }).then(load);
    });
  });
  document.querySelectorAll("[data-node-trust-select]").forEach((select) => {
    select.addEventListener("change", () => {
      const warning = select.closest("article").querySelector("[data-node-trust-warning]");
      if (warning) warning.textContent = trustWarningText(select.value);
    });
  });
}

function credentialInstallCard() {
  if (!lastCredentialInstall?.credential || !lastCredentialInstall?.plan) return "";
  const plan = lastCredentialInstall.plan;
  const wrapper = plan.service_wrapper || {};
  const placeholder = plan.credential_placeholder || "<credential-from-rotation-response>";
  const snippet = [
    plan.config_snippet || "",
    "",
    plan.env_file_template ? `# ${plan.env_file_path || "subscriber.env"}\n${plan.env_file_template}` : "",
    "",
    plan.shell_export_command || "",
    plan.launchd_dry_run_command || "",
    ...(wrapper.install_commands || []),
    ...(wrapper.validation_commands || []),
    plan.heartbeat_check_command || "",
  ].filter(Boolean).join("\n");
  return `<article class="card">
    <h2>Subscriber Credential Install</h2>
    <p class="warning">A rotated heartbeat credential is available only from this browser session. It is not shown in node metadata, bootstrap, telemetry, or support bundles. Copy it or the concrete commands before leaving this view.</p>
    <div class="row"><span>Node</span><strong>${escapeHTML(plan.node_id || lastCredentialInstall.node_id || "managed node")}</strong></div>
    <div class="row"><span>Credential hint</span><strong>${escapeHTML(plan.credential_hint || "available once")}</strong></div>
    <div class="row"><span>Environment variable</span><strong>${escapeHTML(plan.environment_variable || "LLAMA_WRANGLER_SUBSCRIBER_HEARTBEAT_CREDENTIAL")}</strong></div>
    <div class="row"><span>Service wrapper</span><strong>${escapeHTML(formatNodeMeta(wrapper.target || "launchd"))}</strong></div>
    <div class="row"><span>Wrapper path</span><strong>${escapeHTML(wrapper.plist_path || "manual install")}</strong></div>
    <pre>${escapeHTML(snippet)}</pre>
    <p class="warning">The commands above use ${escapeHTML(placeholder)}. Use the copy buttons to copy either the raw credential or commands with this session's credential inserted.</p>
    <button data-copy-install="credential">Copy credential</button>
    <button data-copy-install="shell">Copy shell export</button>
    <button data-copy-install="envfile">Copy env file</button>
    <button data-copy-install="launchd">Copy launchd dry-run</button>
    <button data-copy-install="plist">Copy launchd plist</button>
    <button data-copy-install="install">Copy install commands</button>
    <button data-copy-install="validate">Copy validation commands</button>
    <button data-copy-install="uninstall">Copy uninstall commands</button>
    <button data-copy-install="heartbeat">Copy heartbeat check</button>
  </article>`;
}

function credentialInstallText(kind) {
  const plan = lastCredentialInstall?.plan || {};
  const wrapper = plan.service_wrapper || {};
  const credential = lastCredentialInstall?.credential || "";
  const placeholder = plan.credential_placeholder || "<credential-from-rotation-response>";
  const escapedPlaceholder = placeholder.replaceAll("<", "&lt;").replaceAll(">", "&gt;");
  const replaceCredential = (text) => String(text || "").replaceAll(placeholder, credential).replaceAll(escapedPlaceholder, credential);
  if (kind === "credential") return credential;
  if (kind === "envfile") return replaceCredential(plan.env_file_template || "");
  if (kind === "launchd") return replaceCredential(plan.launchd_dry_run_command || "");
  if (kind === "plist") return replaceCredential(wrapper.launchd_plist_template || "");
  if (kind === "install") return replaceCredential((wrapper.install_commands || []).join("\n"));
  if (kind === "validate") return replaceCredential((wrapper.validation_commands || []).join("\n"));
  if (kind === "uninstall") return replaceCredential((wrapper.uninstall_commands || []).join("\n"));
  if (kind === "heartbeat") return replaceCredential(plan.heartbeat_check_command || "");
  return replaceCredential(plan.shell_export_command || "");
}

function enrollmentQueueRow(item) {
  const expires = item.expires_at ? new Date(item.expires_at).toLocaleString() : "not set";
  const registered = item.registered_at ? new Date(item.registered_at).toLocaleString() : "not registered";
  return `<div class="row">
    <span>${escapeHTML(item.node_id || "pending subscriber")}</span>
    <strong>${escapeHTML(formatNodeMeta(item.approval_state || "pending"))}</strong>
  </div>
  <p class="badge-row">
    <span class="status">Managed</span>
    <span class="status ${trustBadgeClass(item.trust_level)}">${escapeHTML(formatNodeMeta(item.trust_level || "unknown"))}</span>
    <span class="status">${item.token_hint ? `Token ${escapeHTML(item.token_hint)}` : "Token consumed"}</span>
  </p>
  <div class="row"><span>Subscriber URL</span><strong>${escapeHTML(item.url || "waiting for registration")}</strong></div>
  <div class="row"><span>Expires</span><strong>${escapeHTML(expires)}</strong></div>
  <div class="row"><span>Registered</span><strong>${escapeHTML(registered)}</strong></div>`;
}

function nodeCard(node) {
  const control = node.control_level || "managed";
  const trust = node.trust_level || "local";
  const approval = node.approval_state || (node.approved ? "approved" : "pending");
  const capability = node.capability_source || "subscriber_reported";
  const isPassive = control === "passive";
  const canApprove = approval !== "approved";
  const canRevoke = approval !== "revoked";
  const freshness = freshnessLabel(node);
  const benchmark = benchmarkPolicyForNode(node);
  const benchmarkResult = benchmarkResultLabel(node);
  const benchmarkJob = benchmarkJobLabel(node);
  const heartbeatAuth = heartbeatAuthLabel(node);
  const lifecycle = modelLifecycleForNode(node);
  const modelActionPolicy = modelLifecycleActionPolicyForNode(node);
  return `<article class="card">
    <h2>${escapeHTML(node.display_name || node.node_id)}</h2>
    <p class="badge-row">
      <span class="status ${node.status === "healthy" ? "" : "warn"}">${escapeHTML(node.status || "unknown")}</span>
      <span class="status ${control === "passive" ? "warn" : ""}">${escapeHTML(formatNodeMeta(control))}</span>
      <span class="status ${trustBadgeClass(trust)}">${escapeHTML(formatNodeMeta(trust))}</span>
      <span class="status ${approval === "approved" ? "" : "warn"}">${escapeHTML(formatNodeMeta(approval))}</span>
    </p>
    <div class="row"><span>Role</span><strong>${escapeHTML(node.role || "unknown")}</strong></div>
    <div class="row"><span>Capability source</span><strong>${escapeHTML(formatNodeMeta(capability))}</strong></div>
    <label>Trust level<select data-node-trust-select>
      ${trustOption("local", trust, "Local")}
      ${trustOption("lan_trusted", trust, "LAN trusted")}
      ${trustOption("lan_unverified", trust, "LAN unverified")}
      ${trustOption("external", trust, "External")}
    </select></label>
    <p class="warning" data-node-trust-warning>${escapeHTML(trustWarningText(trust))}</p>
    <div class="row"><span>Host</span><strong>${escapeHTML(node.hostname || node.url || "local")}</strong></div>
    <div class="row"><span>Platform</span><strong>${escapeHTML(`${node.platform || "unknown"}/${node.arch || "unknown"}`)}</strong></div>
    <div class="row"><span>Freshness</span><strong>${escapeHTML(freshness)}</strong></div>
    <div class="row"><span>Heartbeat auth</span><strong>${escapeHTML(heartbeatAuth)}</strong></div>
    <div class="row"><span>Heartbeat provisioning</span><strong>${escapeHTML(heartbeatProvisioningLabel(node))}</strong></div>
    <div class="row"><span>Benchmark source</span><strong>${escapeHTML(formatNodeMeta(benchmark.source))}</strong></div>
    <div class="row"><span>Benchmark policy</span><strong>${escapeHTML(benchmark.label)}</strong></div>
    <div class="row"><span>Benchmark placement</span><strong>${escapeHTML(benchmark.placement)}</strong></div>
    <div class="row"><span>Benchmark job</span><strong>${escapeHTML(benchmarkJob)}</strong></div>
    <div class="row"><span>Benchmark result</span><strong>${escapeHTML(benchmarkResult)}</strong></div>
    <div class="row"><span>Model lifecycle</span><strong>${escapeHTML(lifecycle.label)}</strong></div>
    <div class="row"><span>Warm models</span><strong>${escapeHTML(lifecycle.warmSummary)}</strong></div>
    <div class="row"><span>Model actions</span><strong>${escapeHTML(modelActionPolicy.label)}</strong></div>
    <p class="${modelActionPolicy.eligible ? "" : "warning"}">${escapeHTML(modelActionPolicy.message)}</p>
    <div class="row"><span>Ollama</span><strong>${node.ollama_available ? "available" : "not detected"}</strong></div>
    ${isPassive ? `<p class="warning">Limited-control endpoint. Health, inventory, and any future benchmark probes are marshal-observed; hardware, local load, warm state, local benchmark control, and model management are unavailable.</p>` : benchmarkSuiteControls()}
    ${modelStateList(node)}
    ${isPassive ? `<button data-node="${node.node_id}" data-node-action="benchmark-probe">Probe /api/tags</button>` : `<button data-node="${node.node_id}" data-node-action="benchmark">Run benchmark</button>`}
    ${canApprove ? `<button class="primary" data-node="${node.node_id}" data-node-action="approve">Approve</button>` : ""}
    ${canRevoke ? `<button class="danger" data-node="${node.node_id}" data-node-action="revoke">Revoke</button>` : ""}
    <button data-node="${node.node_id}" data-node-trust-action="update">Update trust</button>
    ${isPassive ? "" : `<button data-node="${node.node_id}" data-node-heartbeat-credential-action="rotate">Rotate heartbeat credential</button>`}
    <button data-node="${node.node_id}" data-node-action="${node.enabled ? "disable" : "enable"}">${node.enabled ? "Disable" : "Enable"}</button>
  </article>`;
}

function modelLifecycleForNode(node) {
  const observed = node.observed || {};
  const control = node.control_level || "managed";
  if (control === "passive") {
    return {
      label: "Inventory-only; warm state unavailable",
      warmSummary: "Passive Endpoint",
    };
  }
  const source = observed.model_lifecycle_source || node.model_inventory_source || "subscriber_reported";
  const summary = observed.model_lifecycle_summary || {};
  return {
    label: `${formatNodeMeta(source)} ${formatNodeMeta(observed.model_lifecycle_mode || "rich_lifecycle_metadata")}`,
    warmSummary: `${numberText(summary.warm)} warm / ${numberText(summary.keep_warm)} keep-warm`,
  };
}

function modelLifecycleActionPolicyForNode(node) {
  const policy = (state.model_lifecycle_actions?.nodes || []).find((item) => item.node_id === node.node_id);
  if (policy) {
    const actions = (policy.supported_actions || []).map(formatNodeMeta).join(", ");
    const reasons = (policy.reason_codes || []).map(formatNodeMeta).join(", ");
    return {
      eligible: !!policy.eligible,
      label: policy.eligible ? `Ready: ${actions || "keep warm"}` : `Blocked: ${reasons || "policy pending"}`,
      message: policy.message || "Model lifecycle action policy metadata pending.",
    };
  }
  if (canQueueModelLifecycleAction(node)) {
    return {
      eligible: true,
      label: "Ready: keep warm",
      message: "Managed Node can receive metadata-only subscriber model lifecycle actions.",
    };
  }
  const control = node.control_level || "managed";
  return {
    eligible: false,
    label: control === "passive" ? "Blocked: passive inventory only" : "Blocked: support pending",
    message: control === "passive"
      ? "Passive Endpoints are inventory-only and cannot receive model lifecycle actions."
      : "Managed Node must report warm-state and model-management support before actions are queued.",
  };
}

function modelStateList(node) {
  const models = node.models || [];
  if (!models.length) return `<p>No models detected.</p>`;
  const canQueue = canQueueModelLifecycleAction(node);
  return `<div class="queue-tags">${models.slice(0, 12).map((model) => {
    const state = model.state || "installed";
    const keepWarm = model.keep_warm ? " keep-warm" : "";
    const rate = model.tokens_per_second ? ` ${Number(model.tokens_per_second).toFixed(1)} tok/s` : "";
    const nextKeepWarm = !model.keep_warm;
    const action = canQueue ? `<button class="mini" data-node="${escapeHTML(node.node_id)}" data-model-name="${escapeHTML(model.name || "")}" data-keep-warm="${nextKeepWarm}" data-node-model-action="keep-warm">${nextKeepWarm ? "Keep warm" : "Clear keep-warm"}</button>` : "";
    return `<span class="status ${["failed", "evicted", "unknown"].includes(state) ? "warn" : ""}">${escapeHTML(model.name || "model")} · ${escapeHTML(formatNodeMeta(state))}${escapeHTML(keepWarm)}${escapeHTML(rate)}${action}</span>`;
  }).join("")}</div>`;
}

function canQueueModelLifecycleAction(node) {
  const policy = (state.model_lifecycle_actions?.nodes || []).find((item) => item.node_id === node.node_id);
  if (policy) return !!policy.eligible;
  const control = node.control_level || "managed";
  const approval = node.approval_state || (node.approved ? "approved" : "pending");
  return control === "managed" &&
    approval === "approved" &&
    node.approved !== false &&
    node.enabled !== false &&
    node.status === "healthy" &&
    node.management_supported &&
    node.warm_state_supported;
}

function benchmarkSuiteControls() {
  const suites = state.benchmark_workload_suites || [];
  if (!suites.length) return "";
  return `<label>Benchmark suite<select data-node-benchmark-suite-select>
      ${suites.map((suite) => `<option value="${escapeHTML(suite.id)}">${escapeHTML(suite.display_name || suite.id)}</option>`).join("")}
    </select></label>
    <label>Local fixture manifest ID<input data-node-benchmark-fixture-id placeholder="Optional; required for local fixture suite" /></label>
    <p class="warning">Jobs store suite IDs and local fixture references only. Prompt and response bodies are not persisted or exported.</p>`;
}

function benchmarkJobPayload(button) {
  const card = button.closest("article");
  const suiteSelect = card ? card.querySelector("[data-node-benchmark-suite-select]") : null;
  const fixtureInput = card ? card.querySelector("[data-node-benchmark-fixture-id]") : null;
  return {
    suite_id: suiteSelect ? suiteSelect.value : "synthetic_smoke_v1",
    fixture_manifest_id: fixtureInput ? fixtureInput.value.trim() : "",
  };
}

function freshnessLabel(node) {
  const observed = node.observed || {};
  const reportedAt = node.last_reported_at;
  const observedAt = node.last_observed_at;
  if (!observed.heartbeat_required) {
    if (!reportedAt && !observedAt) return "Not heartbeat-managed";
    return `Reported ${formatDateTime(reportedAt || observedAt)}`;
  }
  if (!reportedAt) return "Heartbeat missing";
  const reportedMs = Date.parse(reportedAt);
  if (Number.isNaN(reportedMs)) return "Heartbeat timestamp unknown";
  if (Date.now() - reportedMs > 2 * 60 * 1000) return "Heartbeat stale";
  return `Fresh ${formatTime(reportedAt)}`;
}

function heartbeatAuthLabel(node) {
  const observed = node.observed || {};
  if (observed.heartbeat_auth_method === "shared_secret") {
    return observed.heartbeat_token_hint ? `Shared secret ${observed.heartbeat_token_hint}` : "Shared secret";
  }
  if (observed.heartbeat_auth_method === "legacy_unverified") return "Legacy unverified";
  if (observed.heartbeat_required) return "Required";
  return "Not configured";
}

function heartbeatProvisioningLabel(node) {
  const observed = node.observed || {};
  if (observed.heartbeat_reprovisioning_required) return "Credential must be installed on subscriber";
  if (observed.heartbeat_credential_provisioned_by === "admin_rotation") return "Admin rotated";
  if (observed.heartbeat_credential_derivation === "random_shared_secret_v1") return "Admin provisioned";
  if (observed.heartbeat_credential_derivation === "hmac_sha256_enrollment_token_node_id_v1") return "Enrollment token derived";
  if (observed.heartbeat_auth_method === "legacy_unverified") return "Legacy unverified";
  return "Not provisioned";
}

function benchmarkResultLabel(node) {
  const observed = node.observed || {};
  const result = observed.benchmark_last_result || {};
  const status = result.status || observed.benchmark_status;
  if (!status) return "No benchmark metadata";
  const model = result.model ? ` ${result.model}` : "";
  const rate = result.tokens_per_second ? ` ${Number(result.tokens_per_second).toFixed(1)} tok/s` : "";
  const latency = result.duration_ms ? ` ${result.duration_ms} ms` : "";
  return `${formatNodeMeta(status)}${model}${rate}${latency}`;
}

function benchmarkJobLabel(node) {
  const observed = node.observed || {};
  const jobs = Array.isArray(observed.benchmark_jobs) ? observed.benchmark_jobs : [];
  const job = jobs[0] || {};
  const status = job.status || observed.benchmark_status;
  if (!status) return "No job";
  const id = job.benchmark_id || observed.benchmark_id || "";
  const modelCount = Array.isArray(job.model_candidates) ? ` ${job.model_candidates.length} models` : "";
  const attempt = job.max_attempts ? ` attempt ${Number(job.attempt || 0)} / ${Number(job.max_attempts || 0)}` : "";
  const scheduler = job.scheduler_state ? ` ${formatNodeMeta(job.scheduler_state)}` : "";
  const suite = job.workload_suite?.suite_id ? ` ${formatNodeMeta(job.workload_suite.suite_id)}` : "";
  return `${formatNodeMeta(status)}${scheduler}${id ? ` ${id}` : ""}${suite}${modelCount}${attempt}`;
}

function benchmarkPolicyForNode(node) {
  const control = node.control_level || "managed";
  const source = node.benchmark_source || (control === "passive" ? "none" : "subscriber_reported");
  if (control === "passive") {
    return { source, label: "Probe-only; no local control", placement: "Probe ignored for placement" };
  }
  const approved = (node.approval_state || (node.approved ? "approved" : "pending")) === "approved" && node.approved !== false;
  if (!approved || !node.enabled || ["disabled", "failed"].includes(node.status)) {
    return { source, label: "Requires approved healthy node", placement: "Waiting for eligibility" };
  }
  const observed = node.observed || {};
  const result = observed.benchmark_last_result || {};
  const status = result.status || observed.benchmark_status;
  const updatedAt = result.completed_at || observed.benchmark_updated_at;
  const fresh = updatedAt && Date.now() - Date.parse(updatedAt) <= 24 * 60 * 60 * 1000;
  const rate = Number(result.tokens_per_second || result.output_tokens_per_second || 0);
  if (source !== "subscriber_reported") return { source, label: "Subscriber-reported queue", placement: "Waiting for subscriber summary" };
  if (!status) return { source, label: "Subscriber-reported queue", placement: "No placement summary" };
  if (!fresh) return { source, label: "Subscriber-reported queue", placement: "Placement summary stale" };
  if (rate <= 0) return { source, label: "Subscriber-reported queue", placement: "Token-rate missing" };
  return { source, label: "Subscriber-reported queue", placement: "Fresh summary ready" };
}

function formatDateTime(value) {
  if (!value) return "unknown";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "unknown";
  return date.toLocaleString();
}

function formatTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "unknown";
  return date.toLocaleTimeString();
}

function trustOption(value, current, label) {
  return `<option value="${value}" ${current === value ? "selected" : ""}>${label}</option>`;
}

function trustWarningText(trust) {
  if (trust === "external") return "External endpoints should stay disabled for sensitive workloads unless policy explicitly allows them.";
  if (trust === "lan_unverified") return "LAN unverified endpoints are limited trust and should not receive sensitive workloads until reviewed.";
  return "";
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

function benchmarkSchedulerPolicyPayload() {
  return {
    policy: "bounded_retry_timeout_v1",
    max_attempts: numberValue("benchmark-max-attempts", 3),
    lease_timeout_seconds: numberValue("benchmark-lease-timeout", 600),
    retry_delay_seconds: numberValue("benchmark-retry-delay", 60),
    background_enabled: checked("benchmark-background-enabled"),
    tick_interval_seconds: numberValue("benchmark-tick-interval", 60),
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
