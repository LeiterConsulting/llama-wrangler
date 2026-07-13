#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OPT_IN="${LLAMA_WRANGLER_MACOS_LAUNCHD_ACCEPTANCE:-0}"
TEMP_ROOT=""
BOOTSTRAPPED=0
ADMIN_TOKEN=""
CLIENT_KEY=""
PASS_COUNT=0

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

cleanup() {
  ADMIN_TOKEN=""
  CLIENT_KEY=""
  if [[ "${BOOTSTRAPPED}" == "1" && -n "${SERVICE_TARGET:-}" ]]; then
    launchctl bootout "${SERVICE_TARGET}" >/dev/null 2>&1 || true
    BOOTSTRAPPED=0
  fi
  if [[ -n "${TEMP_ROOT}" && -d "${TEMP_ROOT}" ]]; then
    rm -rf "${TEMP_ROOT}"
  fi
}

trap cleanup EXIT INT TERM

file_mode() {
  local path="$1"
  stat -f '%Lp' "${path}"
}

launchd_pid() {
  launchctl print "${SERVICE_TARGET}" 2>/dev/null | awk '$1 == "pid" && $2 == "=" { print $3; exit }'
}

wait_for_initial_health() {
  local attempt
  for attempt in $(seq 1 120); do
    if curl -fsS "${BASE_URL}/healthz" >/dev/null 2>&1; then
      return 0
    fi
    launchctl print "${SERVICE_TARGET}" >/dev/null 2>&1 || fail "launchd job disappeared before becoming healthy"
    sleep 0.1
  done
  fail "launchd service did not become healthy"
}

wait_for_restarted_health() {
  local old_pid="$1"
  local attempt current_pid
  for attempt in $(seq 1 120); do
    current_pid="$(launchd_pid || true)"
    if [[ -n "${current_pid}" && "${current_pid}" != "${old_pid}" ]] && curl -fsS "${BASE_URL}/healthz" >/dev/null 2>&1; then
      printf '%s' "${current_pid}"
      return 0
    fi
    sleep 0.1
  done
  fail "launchd service did not restart with a new healthy process"
}

wait_for_uninstall() {
  local attempt
  for attempt in $(seq 1 80); do
    if ! launchctl print "${SERVICE_TARGET}" >/dev/null 2>&1 && ! curl -fsS "${BASE_URL}/healthz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.1
  done
  fail "launchd job or service endpoint remained after uninstall"
}

if [[ "$(uname -s)" != "Darwin" ]]; then
  fail "macOS launchd acceptance requires Darwin; no lifecycle status was changed"
fi

for command_name in go curl jq python3 plutil launchctl awk grep stat mktemp find dirname id mv rm; do
  require_command "${command_name}"
done

case "${OPT_IN}" in
  0|1) ;;
  *) fail "LLAMA_WRANGLER_MACOS_LAUNCHD_ACCEPTANCE must be 0 or 1" ;;
esac

cd "${ROOT_DIR}"
TEMP_BASE="${TMPDIR:-/tmp}"
TEMP_ROOT="$(mktemp -d "${TEMP_BASE%/}/llama-wrangler-launchd-acceptance.XXXXXX")"
DISPOSABLE_HOME="${TEMP_ROOT}/home"
DISPOSABLE_XDG="${TEMP_ROOT}/xdg"
PACKAGE_ROOT="${TEMP_ROOT}/package"
BIN_DIR="${PACKAGE_ROOT}/bin"
CONFIG_DIR="${PACKAGE_ROOT}/config"
LOG_DIR="${DISPOSABLE_HOME}/Library/Logs/Llama Wrangler Acceptance"
LAUNCH_AGENTS_DIR="${DISPOSABLE_HOME}/Library/LaunchAgents"
BINARY_PATH="${BIN_DIR}/llama-wrangler"
UPGRADE_PATH="${BIN_DIR}/llama-wrangler.upgrade"
CONFIG_PATH="${CONFIG_DIR}/standalone.yaml"
PLAN_PATH="${TEMP_ROOT}/launchd-plan.json"
LABEL="com.llama-wrangler.acceptance.$(id -u).$$"
PLIST_PATH="${LAUNCH_AGENTS_DIR}/${LABEL}.plist"
SERVICE_DOMAIN="gui/$(id -u)"
SERVICE_TARGET="${SERVICE_DOMAIN}/${LABEL}"

mkdir -p "${DISPOSABLE_HOME}" "${DISPOSABLE_XDG}" "${BIN_DIR}" "${CONFIG_DIR}" "${LOG_DIR}" "${LAUNCH_AGENTS_DIR}"

ACCEPTANCE_PORT="$(python3 -c 'import socket; sock=socket.socket(); sock.bind(("127.0.0.1", 0)); print(sock.getsockname()[1]); sock.close()')"
BASE_URL="http://127.0.0.1:${ACCEPTANCE_PORT}"

cat >"${CONFIG_PATH}" <<EOF
server:
  mode: standalone
  listen: "127.0.0.1:${ACCEPTANCE_PORT}"

routing:
  request_timeout_seconds: 2
  queue_max_depth: 16

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

go build -ldflags '-X main.version=acceptance-initial' -o "${BINARY_PATH}" ./cmd/llama-wrangler
[[ "$("${BINARY_PATH}" version)" == "acceptance-initial" ]] || fail "initial package build identity mismatch"
pass "M01 disposable package layout and initial binary"

"${BINARY_PATH}" service-dry-run \
  --target launchd \
  --binary "${BINARY_PATH}" \
  --mode standalone \
  --config "${CONFIG_PATH}" \
  --label "${LABEL}" \
  --working-dir "${PACKAGE_ROOT}" \
  --log-dir "${LOG_DIR}" \
  --launch-agents-dir "${LAUNCH_AGENTS_DIR}" \
  --env "HOME=${DISPOSABLE_HOME}" \
  --env "XDG_CONFIG_HOME=${DISPOSABLE_XDG}" \
  --env "LLAMA_WRANGLER_SECRET_BACKEND=encrypted_file" >"${PLAN_PATH}"

jq -e --arg label "${LABEL}" --arg plist "${PLIST_PATH}" '
  .target == "launchd" and
  .dry_run == true and
  .supported == true and
  .label == $label and
  .wrapper_path == $plist and
  .environment.LLAMA_WRANGLER_SERVICE_MODE == "1" and
  .environment.LLAMA_WRANGLER_SECRET_BACKEND == "encrypted_file" and
  (.environment.LLAMA_WRANGLER_KEYCHAIN_SERVICE == null) and
  (.launchd_plist | contains("LLAMA_WRANGLER_SERVICE_MODE")) and
  (.launchd_plist | contains("LLAMA_WRANGLER_SECRET_BACKEND"))
' "${PLAN_PATH}" >/dev/null || fail "launchd dry-run plan assertions"

jq -r '.launchd_plist' "${PLAN_PATH}" >"${PLIST_PATH}"
chmod 600 "${PLIST_PATH}"
plutil -lint "${PLIST_PATH}" >/dev/null
[[ "$(file_mode "${PLIST_PATH}")" == "600" ]] || fail "launchd plist permissions are not 0600"
pass "M02 dry-run plan, encrypted fallback, and plist lint"

if [[ "${OPT_IN}" != "1" ]]; then
  printf '\nmacOS launchd dry run completed: %d gates passed.\n' "${PASS_COUNT}"
  printf 'No launchd job was installed or started.\n'
  printf 'Set LLAMA_WRANGLER_MACOS_LAUNCHD_ACCEPTANCE=1 for the disposable current-user lifecycle check.\n'
  exit 0
fi

launchctl print "${SERVICE_TARGET}" >/dev/null 2>&1 && fail "disposable launchd label already exists"
launchctl bootstrap "${SERVICE_DOMAIN}" "${PLIST_PATH}"
BOOTSTRAPPED=1
wait_for_initial_health
INITIAL_PID="$(launchd_pid)"
[[ -n "${INITIAL_PID}" ]] || fail "launchd did not report an initial service pid"
pass "M03 user-level install and start"

SETUP_RESULT="$(curl -fsS -X POST "${BASE_URL}/wrangler/setup/complete")"
ADMIN_TOKEN="$(printf '%s' "${SETUP_RESULT}" | jq -er '.admin_token')"
CLIENT_KEY="$(printf '%s' "${SETUP_RESULT}" | jq -er '.client_api_key')"
SETUP_RESULT=""
[[ "${ADMIN_TOKEN}" == lw_admin_* ]] || fail "setup did not return an admin token"
[[ "${CLIENT_KEY}" == lw_client_* ]] || fail "setup did not return a client API key"

BOOTSTRAP="$(curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" "${BASE_URL}/wrangler/ui/bootstrap")"
printf '%s' "${BOOTSTRAP}" | jq -e '
  .schema_version == 2 and
  .setup_complete == true and
  .safe_defaults.local_only == true and
  .safe_defaults.frontier_enabled == false and
  .safe_defaults.lan_access_enabled == false and
  .safe_defaults.prompt_body_logging == false and
  .safe_defaults.telemetry_level == "metadata_only" and
  .secret_storage.backend == "encrypted_file" and
  .secret_storage.encrypted == true and
  .secret_storage.os_keychain_status == "disabled" and
  .secret_storage.os_keychain_runtime == "service_like" and
  .secret_storage.os_keychain_service_mode == true
' >/dev/null || fail "service-mode safe-default assertions"
BOOTSTRAP=""

SECRET_STORE_PATH="$(find "${DISPOSABLE_HOME}" "${DISPOSABLE_XDG}" -name secrets.enc.json -type f -print -quit)"
[[ -n "${SECRET_STORE_PATH}" ]] || fail "encrypted service secret store was not created"
APP_DATA_DIR="$(dirname "${SECRET_STORE_PATH}")"
for secret_file in "${APP_DATA_DIR}/secrets.enc.json" "${APP_DATA_DIR}/secrets.key"; do
  [[ -f "${secret_file}" ]] || fail "encrypted service secret artifact missing"
  [[ "$(file_mode "${secret_file}")" == "600" ]] || fail "encrypted service secret artifact permissions are not 0600"
  grep -aF "${ADMIN_TOKEN}" "${secret_file}" >/dev/null 2>&1 && fail "admin token persisted in plaintext"
  grep -aF "${CLIENT_KEY}" "${secret_file}" >/dev/null 2>&1 && fail "client key persisted in plaintext"
done
[[ ! -e "${APP_DATA_DIR}/secrets.json" ]] || fail "legacy plaintext service secret file exists"
pass "M04 setup, schema v2, safe defaults, and encrypted service secrets"

launchctl kickstart -k "${SERVICE_TARGET}"
RESTART_PID="$(wait_for_restarted_health "${INITIAL_PID}")"
curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" "${BASE_URL}/wrangler/config" >/dev/null || fail "admin auth did not survive launchd restart"
curl -fsS -H "Authorization: Bearer ${CLIENT_KEY}" "${BASE_URL}/v1/models" >/dev/null || fail "client auth did not survive launchd restart"
pass "M05 launchd restart and persisted authentication"

go build -ldflags '-X main.version=acceptance-upgrade' -o "${UPGRADE_PATH}" ./cmd/llama-wrangler
[[ "$("${UPGRADE_PATH}" version)" == "acceptance-upgrade" ]] || fail "upgrade package build identity mismatch"
mv -f "${UPGRADE_PATH}" "${BINARY_PATH}"
[[ "$("${BINARY_PATH}" version)" == "acceptance-upgrade" ]] || fail "atomic package replacement did not install the upgrade build"
launchctl kickstart -k "${SERVICE_TARGET}"
UPGRADE_PID="$(wait_for_restarted_health "${RESTART_PID}")"
[[ "${UPGRADE_PID}" != "${INITIAL_PID}" ]] || fail "upgrade did not produce a new service process"

UPGRADE_BOOTSTRAP="$(curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" "${BASE_URL}/wrangler/ui/bootstrap")"
printf '%s' "${UPGRADE_BOOTSTRAP}" | jq -e '
  .schema_version == 2 and
  .setup_complete == true and
  .secret_storage.backend == "encrypted_file" and
  .secret_storage.encrypted == true
' >/dev/null || fail "state or encrypted fallback did not survive package upgrade"
UPGRADE_BOOTSTRAP=""
pass "M06 atomic binary upgrade and persisted state"

for service_log in "${LOG_DIR}/launchd.out.log" "${LOG_DIR}/launchd.err.log"; do
  if [[ -f "${service_log}" ]]; then
    grep -aEq 'lw_(admin|client|enroll|heartbeat)_[A-Za-z0-9_-]+' "${service_log}" && fail "service log exposed a credential"
    grep -aEq 'SECRET_PROMPT|SECRET_RESPONSE|CONSENSUS_OUTPUT_SECRET' "${service_log}" && fail "service log exposed a payload marker"
  fi
done
pass "M07 service-log credential and payload exclusion"

launchctl bootout "${SERVICE_TARGET}"
BOOTSTRAPPED=0
wait_for_uninstall
rm -f "${PLIST_PATH}" "${BINARY_PATH}" "${CONFIG_PATH}" "${PLAN_PATH}"
[[ ! -e "${PLIST_PATH}" && ! -e "${BINARY_PATH}" && ! -e "${CONFIG_PATH}" ]] || fail "package artifacts remained after uninstall"
pass "M08 user-level uninstall and artifact removal"

ADMIN_TOKEN=""
CLIENT_KEY=""

printf '\nmacOS disposable launchd lifecycle completed: %d gates passed.\n' "${PASS_COUNT}"
printf 'Validated current-user install/start/restart/upgrade/uninstall mechanics only.\n'
printf 'Signed package-candidate, notarization, and packaged keychain acceptance remain pending.\n'
