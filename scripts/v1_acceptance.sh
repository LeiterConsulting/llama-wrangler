#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEMP_ROOT=""
SERVICE_PID=""
PASS_COUNT=0
TESTS_SKIPPED=0

pass() {
  PASS_COUNT=$((PASS_COUNT + 1))
  printf 'PASS  %s\n' "$1"
}

fail() {
  printf 'FAIL  %s\n' "$1" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

stop_service() {
  if [[ -n "${SERVICE_PID}" ]]; then
    kill "${SERVICE_PID}" >/dev/null 2>&1 || true
    wait "${SERVICE_PID}" >/dev/null 2>&1 || true
    SERVICE_PID=""
  fi
}

cleanup() {
  stop_service
  if [[ -n "${TEMP_ROOT}" && -d "${TEMP_ROOT}" ]]; then
    rm -rf "${TEMP_ROOT}"
  fi
}

trap cleanup EXIT INT TERM

run_gate() {
  local label="$1"
  shift
  "$@"
  pass "${label}"
}

http_status() {
  curl -sS -o /dev/null -w '%{http_code}' "$@"
}

file_mode() {
  local path="$1"
  if stat -f '%Lp' "${path}" >/dev/null 2>&1; then
    stat -f '%Lp' "${path}"
  else
    stat -c '%a' "${path}"
  fi
}

wait_for_health() {
  local attempt
  for attempt in $(seq 1 80); do
    if curl -fsS "${BASE_URL}/healthz" >/dev/null 2>&1; then
      return 0
    fi
    if ! kill -0 "${SERVICE_PID}" >/dev/null 2>&1; then
      fail "disposable service exited before becoming healthy"
    fi
    sleep 0.1
  done
  fail "disposable service did not become healthy"
}

start_service() {
  HOME="${ACCEPTANCE_HOME}" \
  XDG_CONFIG_HOME="${ACCEPTANCE_XDG}" \
  LLAMA_WRANGLER_SECRET_BACKEND="encrypted_file" \
    "${ACCEPTANCE_BINARY}" standalone --config "${ACCEPTANCE_CONFIG}" >"${SERVICE_LOG}" 2>&1 &
  SERVICE_PID=$!
  wait_for_health
}

cd "${ROOT_DIR}"

for command_name in go curl jq python3 xmllint node bash grep stat mktemp find dirname; do
  require_command "${command_name}"
done

printf 'Llama Wrangler V1 acceptance harness\n'
printf 'Workspace: %s\n' "${ROOT_DIR}"
printf 'Safety: disposable app data, loopback listener, no OS service mutation\n\n'

if [[ "${LLAMA_WRANGLER_ACCEPTANCE_SKIP_TESTS:-0}" != "1" ]]; then
  run_gate "A01 state, config, migration, and encrypted secret boundaries" \
    go test -count=1 ./internal/config ./internal/appstate ./internal/secrets \
      -run 'Test(DefaultSafePosture|Open|SaveConfig|EncryptedStore|LegacyPlaintextSecrets|SecretStatus|Rekey)'

  run_gate "A02 setup, admin, client authentication, rotation, and rate limits" \
    go test -count=1 ./internal/httpapi \
      -run 'Test(ManagementAuth|ClientAPIKey|AdminToken|SecretRekey|AdminAuthFailures|ClientAuthFailures)'

  run_gate "A03 Managed Node enrollment, heartbeat identity, approval, and Passive Endpoint limits" \
    go test -count=1 ./internal/httpapi \
      -run 'Test(PassiveAdd|PassiveEndpoint|ManualSubscriber|NodeApprove|NodeTrust|ManagedEnrollment|SubscriberHeartbeat|ManagedNodeHeartbeat)'

  run_gate "A04 routing, session policy, consensus, and streaming posture" \
    go test -count=1 ./internal/routing ./internal/session ./internal/consensus ./internal/httpapi \
      -run 'Test(Select|ApplyAffinity|ConsensusRealClients|MarshalConsensus|ForwardConsensus|FetchConsensusParticipant|OpenAIStreaming|OllamaStreaming|ForwardRetries|ForwardDoesNotRetry|ForwardEmitsCancellation)'

  run_gate "A05 benchmark scheduler, runner, workload, and model lifecycle actions" \
    go test -count=1 ./internal/httpapi \
      -run 'Test(Benchmark|SubscriberBenchmark|ManagedBenchmark|PassiveBenchmark|Model|ManagedModel|PassiveEndpointModel)'

  run_gate "A06 Splunk package, HEC TLS, operation stats, and support-bundle privacy" \
    go test -count=1 ./internal/hec ./internal/httpapi \
      -run 'Test(Splunk|Send|SupportBundle|SummarizeOperations|BootstrapAndMetricsIncludeOperationStats)'

  run_gate "A07 service-wrapper dry-run safety" \
    go test -count=1 ./internal/servicewrap -run 'TestLaunchdDryRun'

  run_gate "A08 complete repository test suite" go test -count=1 ./...
else
  TESTS_SKIPPED=1
fi

run_gate "A09 embedded UI JavaScript syntax" node -c internal/httpapi/static/app.js
run_gate "A10 HEC and support-bundle JSON schemas" jq empty schemas/hec_events.schema.json schemas/support_bundle.schema.json
run_gate "A11 Splunk Simple XML and navigation" xmllint --noout \
  splunk_app/default/data/ui/views/llama_wrangler_overview.xml \
  splunk_app/default/data/ui/views/llama_wrangler_operations.xml \
  splunk_app/default/data/ui/nav/default.xml
run_gate "A12 Splunk conf stanza parsing" python3 -c \
  'import configparser, pathlib; files=sorted(pathlib.Path("splunk_app/default").glob("*.conf")); [(lambda p: (lambda c: c.read(p))(configparser.RawConfigParser(strict=True)))(p) for p in files]; assert len(files) == 7'

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/llama-wrangler-v1-acceptance.XXXXXX")"
ACCEPTANCE_HOME="${TEMP_ROOT}/home"
ACCEPTANCE_XDG="${TEMP_ROOT}/xdg"
ACCEPTANCE_BINARY="${TEMP_ROOT}/llama-wrangler"
ACCEPTANCE_CONFIG="${TEMP_ROOT}/acceptance.yaml"
SERVICE_LOG="${TEMP_ROOT}/service.log"
mkdir -p "${ACCEPTANCE_HOME}" "${ACCEPTANCE_XDG}"

if [[ -n "${LLAMA_WRANGLER_ACCEPTANCE_PORT:-}" ]]; then
  ACCEPTANCE_PORT="${LLAMA_WRANGLER_ACCEPTANCE_PORT}"
else
  ACCEPTANCE_PORT="$(python3 -c 'import socket; sock=socket.socket(); sock.bind(("127.0.0.1", 0)); print(sock.getsockname()[1]); sock.close()')"
fi
BASE_URL="http://127.0.0.1:${ACCEPTANCE_PORT}"

cat >"${ACCEPTANCE_CONFIG}" <<EOF
server:
  mode: standalone
  listen: "127.0.0.1:${ACCEPTANCE_PORT}"

routing:
  request_timeout_seconds: 2
  queue_max_depth: 16
  queue_scheduling_policy: weighted_priority
  queue_priority_weights:
    high: 3
    normal: 2
    low: 1

frontier:
  enabled: false
  local_only: true

telemetry:
  json_logs: false
  logging_level: metadata_only
  store_payloads: false
  splunk_hec:
    enabled: false
    verify_ssl: true
EOF

run_gate "A13 build disposable acceptance binary" go build -o "${ACCEPTANCE_BINARY}" ./cmd/llama-wrangler

start_service
pass "A14 disposable loopback service start"

BOOTSTRAP="$(curl -fsS "${BASE_URL}/wrangler/ui/bootstrap")"
printf '%s' "${BOOTSTRAP}" | jq -e '
  .schema_version == 2 and
  .setup_complete == false and
  .safe_defaults.local_only == true and
  .safe_defaults.frontier_enabled == false and
  .safe_defaults.lan_access_enabled == false and
  .safe_defaults.prompt_body_logging == false and
  .safe_defaults.telemetry_level == "metadata_only" and
  .queue.scheduling.policy == "weighted_priority" and
  .secret_storage.backend == "encrypted_file" and
  .secret_storage.encrypted == true and
  .telemetry.splunk_hec.enabled == false and
  .telemetry.splunk_hec.verify_ssl == true
' >/dev/null || fail "disposable bootstrap safe-default assertions"
pass "A15 live safe defaults and schema v2"
unset BOOTSTRAP

curl -fsS "${BASE_URL}/ui/" | grep -q 'Llama Wrangler' || fail "embedded UI did not render"
pass "A16 embedded UI route"

SETUP_RESULT="$(curl -fsS -X POST "${BASE_URL}/wrangler/setup/complete")"
ADMIN_TOKEN="$(printf '%s' "${SETUP_RESULT}" | jq -er '.admin_token')"
CLIENT_KEY="$(printf '%s' "${SETUP_RESULT}" | jq -er '.client_api_key')"
[[ "${ADMIN_TOKEN}" == lw_admin_* ]] || fail "setup did not return an admin token"
[[ "${CLIENT_KEY}" == lw_client_* ]] || fail "setup did not return a client API key"
SETUP_RESULT=""

[[ "$(http_status "${BASE_URL}/wrangler/config")" == "401" ]] || fail "management API accepted missing admin auth"
curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" "${BASE_URL}/wrangler/config" >/dev/null || fail "management API rejected valid admin auth"
[[ "$(http_status "${BASE_URL}/v1/models")" == "401" ]] || fail "inference API accepted missing client auth"
curl -fsS -H "Authorization: Bearer ${CLIENT_KEY}" "${BASE_URL}/v1/models" >/dev/null || fail "inference API rejected valid client auth"
pass "A17 live setup, admin auth, and client auth"

ENROLLMENT_RESULT="$(curl -fsS -X POST "${BASE_URL}/wrangler/enrollment-tokens" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  --data '{"node_id":"acceptance-worker","subscriber_url":"http://127.0.0.1:19436","trust_level":"lan_trusted","ttl_minutes":5}')"
ENROLLMENT_TOKEN="$(printf '%s' "${ENROLLMENT_RESULT}" | jq -er '.token')"
[[ "${ENROLLMENT_TOKEN}" == lw_enroll_* ]] || fail "enrollment token was not generated"
ENROLLMENT_RESULT=""

ENROLL_RESULT="$(curl -fsS -X POST "${BASE_URL}/subscriber/enroll" \
  -H 'Content-Type: application/json' \
  --data "{\"token\":\"${ENROLLMENT_TOKEN}\",\"node_id\":\"acceptance-worker\",\"display_name\":\"Acceptance Worker\",\"subscriber_url\":\"http://127.0.0.1:19436\",\"hostname\":\"acceptance.local\",\"platform\":\"test\",\"arch\":\"test\",\"ollama_available\":true,\"models\":[{\"name\":\"acceptance-model\",\"state\":\"installed\"}]}")"
printf '%s' "${ENROLL_RESULT}" | jq -e '
  .status == "pending_approval" and
  .node.control_level == "managed" and
  .node.trust_level == "lan_trusted" and
  .node.approval_state == "pending" and
  .node.approved == false and
  .heartbeat_auth.method == "shared_secret"
' >/dev/null || fail "Managed Node enrollment pending-state assertions"
[[ "${ENROLL_RESULT}" != *"${ENROLLMENT_TOKEN}"* ]] || fail "enrollment response echoed enrollment token"
ENROLL_RESULT=""

APPROVED_NODE="$(curl -fsS -X POST "${BASE_URL}/wrangler/nodes/acceptance-worker/approve" -H "Authorization: Bearer ${ADMIN_TOKEN}")"
printf '%s' "${APPROVED_NODE}" | jq -e '.control_level == "managed" and .approval_state == "approved" and .approved == true' >/dev/null || fail "Managed Node approval assertions"
APPROVED_NODE=""
pass "A18 live Managed Node enrollment and approval"

SECRET_STORE_PATH="$(find "${ACCEPTANCE_HOME}" "${ACCEPTANCE_XDG}" -name secrets.enc.json -type f -print -quit)"
[[ -n "${SECRET_STORE_PATH}" ]] || fail "encrypted secret store was not created"
APP_DATA_DIR="$(dirname "${SECRET_STORE_PATH}")"
for secret_file in "${APP_DATA_DIR}/secrets.enc.json" "${APP_DATA_DIR}/secrets.key"; do
  [[ -f "${secret_file}" ]] || fail "encrypted secret artifact missing: ${secret_file##*/}"
  [[ "$(file_mode "${secret_file}")" == "600" ]] || fail "secret artifact permissions are not 0600: ${secret_file##*/}"
  grep -aF "${ADMIN_TOKEN}" "${secret_file}" >/dev/null 2>&1 && fail "admin token persisted in plaintext"
  grep -aF "${CLIENT_KEY}" "${secret_file}" >/dev/null 2>&1 && fail "client key persisted in plaintext"
  grep -aF "${ENROLLMENT_TOKEN}" "${secret_file}" >/dev/null 2>&1 && fail "enrollment token persisted in plaintext"
done
[[ ! -e "${APP_DATA_DIR}/secrets.json" ]] || fail "legacy plaintext secret file exists"
pass "A19 encrypted secret artifacts and file permissions"

SUPPORT_BUNDLE="$(curl -fsS -X POST "${BASE_URL}/wrangler/support-bundle/export" -H "Authorization: Bearer ${ADMIN_TOKEN}")"
printf '%s' "${SUPPORT_BUNDLE}" | jq -e '
  .bundle_schema.version == 1 and
  .service.schema_version == 2 and
  .privacy.prompt_bodies_included == false and
  .privacy.response_bodies_included == false and
  .privacy.secrets_included == false
' >/dev/null || fail "support-bundle schema/privacy assertions"
for secret_value in "${ADMIN_TOKEN}" "${CLIENT_KEY}" "${ENROLLMENT_TOKEN}"; do
  [[ "${SUPPORT_BUNDLE}" != *"${secret_value}"* ]] || fail "support bundle exposed disposable credential"
done
printf '%s' "${SUPPORT_BUNDLE}" | grep -Eq 'SECRET_PROMPT|SECRET_RESPONSE|CONSENSUS_OUTPUT_SECRET' && fail "support bundle exposed payload marker"
SUPPORT_BUNDLE=""
pass "A20 live support-bundle privacy"

stop_service
start_service
pass "A21 disposable service restart"

curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" "${BASE_URL}/wrangler/config" >/dev/null || fail "admin auth did not survive restart"
curl -fsS -H "Authorization: Bearer ${CLIENT_KEY}" "${BASE_URL}/v1/models" >/dev/null || fail "client auth did not survive restart"
RESTART_BOOTSTRAP="$(curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" "${BASE_URL}/wrangler/ui/bootstrap")"
printf '%s' "${RESTART_BOOTSTRAP}" | jq -e '
  .schema_version == 2 and
  .setup_complete == true and
  .nodes["acceptance-worker"].control_level == "managed" and
  .nodes["acceptance-worker"].approval_state == "approved" and
  .secret_storage.backend == "encrypted_file"
' >/dev/null || fail "state/auth/enrollment persistence assertions after restart"
RESTART_BOOTSTRAP=""
pass "A22 state, auth, and Managed Node persistence after restart"

ADMIN_TOKEN=""
CLIENT_KEY=""
ENROLLMENT_TOKEN=""

if [[ "${TESTS_SKIPPED}" == "1" ]]; then
  printf '\nV1 lifecycle subset completed: %d gates passed; grouped/full test gates were skipped.\n' "${PASS_COUNT}"
  printf 'This is not complete automated release evidence.\n'
else
  printf '\nV1 automated acceptance completed: %d gates passed.\n' "${PASS_COUNT}"
fi
printf 'External acceptance remains pending where listed in docs/21_v1_acceptance_security_matrix.md.\n'
