#!/usr/bin/env bash
# =============================================================================
# RedisMeter Full Showcase Smoke Test
# =============================================================================
# Flow:
#   1. Preflight checks (tools, server health, Azure login)
#   2. Discover workloads & run profiles
#   3. Create infrastructure profile (stored template)
#   4. Deploy Azure infrastructure (AMR + runner VMs via Terraform)
#   5. Wait for infra to be ready
#   6. Run benchmark 1 (cloud benchmark – workload: cache, profile: default)
#   7. Wait for benchmark 1 to complete
#   8. Run benchmark 2 (second run for comparison)
#   9. Wait for benchmark 2 to complete
#  10. Save run 1 as baseline
#  11. Compare run 1 vs run 2
#  12. Print final report
#
# Features:
#   - Progress state file – safe to restart; each phase is idempotent
#   - Retry logic with exponential back-off for all API calls
#   - Full structured logging (console + log file)
#   - Infra is NOT destroyed at the end
#
# Prerequisites:
#   - curl, jq
#   - RedisMeter server running on localhost:8080 (or set REDISMETER_URL)
#   - Azure CLI (az) logged in (or AZURE_SUBSCRIPTION_ID env var set)
#
# Configurable env vars:
#   REDISMETER_URL          – server base URL      (default: http://localhost:8080)
#   AZURE_REGION            – Azure region          (default: eastus)
#   AZURE_RESOURCE_GROUP    – resource group name   (optional, auto-generated if blank)
#   INFRA_SKU               – AMR SKU               (default: Balanced_B5)
#   INFRA_HA                – high availability     (default: false)
#   RUNNER_VM_SIZE          – Azure VM size         (default: Standard_D4s_v3)
#   RUNNER_COUNT            – number of runner VMs  (default: 1)
#   BENCHMARK_WORKLOAD      – workload name         (default: cache)
#   BENCHMARK_RUN_PROFILE   – run profile name      (default: default)
#   SMOKE_STATE_FILE        – state persistence file (default: ~/.redismeter/smoke-test-state.json)
#   SMOKE_LOG_FILE          – log file path          (default: ~/.redismeter/smoke-test-<ts>.log)
#   MAX_INFRA_WAIT_SECS     – max seconds to wait for infra ready (default: 1800 = 30 min)
#   MAX_BENCH_WAIT_SECS     – max seconds to wait for benchmark   (default: 600 = 10 min)
# =============================================================================

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration (all can be overridden via env vars)
# ---------------------------------------------------------------------------
REDISMETER_URL="${REDISMETER_URL:-http://localhost:8080}"
AZURE_REGION="${AZURE_REGION:-eastus}"
INFRA_SKU="${INFRA_SKU:-Balanced_B5}"
INFRA_HA="${INFRA_HA:-false}"
RUNNER_VM_SIZE="${RUNNER_VM_SIZE:-Standard_D4s_v3}"
RUNNER_COUNT="${RUNNER_COUNT:-1}"
BENCHMARK_WORKLOAD="${BENCHMARK_WORKLOAD:-cache}"
BENCHMARK_RUN_PROFILE="${BENCHMARK_RUN_PROFILE:-default}"
MAX_INFRA_WAIT_SECS="${MAX_INFRA_WAIT_SECS:-1800}"
MAX_BENCH_WAIT_SECS="${MAX_BENCH_WAIT_SECS:-600}"

STATE_DIR="${HOME}/.redismeter"
SMOKE_STATE_FILE="${SMOKE_STATE_FILE:-${STATE_DIR}/smoke-test-state.json}"
TS=$(date +%Y%m%d_%H%M%S)
SMOKE_LOG_FILE="${SMOKE_LOG_FILE:-${STATE_DIR}/smoke-test-${TS}.log}"

INFRA_NAME="redismeter-prod"

# ---------------------------------------------------------------------------
# Colours
# ---------------------------------------------------------------------------
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

# ---------------------------------------------------------------------------
# Logging helpers
# ---------------------------------------------------------------------------
mkdir -p "${STATE_DIR}"

log() {
    local level="$1"; shift
    local msg="$*"
    local ts; ts=$(date '+%Y-%m-%dT%H:%M:%S')
    local coloured
    case "${level}" in
        INFO)  coloured="${CYAN}[INFO]${RESET} ${msg}" ;;
        OK)    coloured="${GREEN}[OK]${RESET}   ${msg}" ;;
        WARN)  coloured="${YELLOW}[WARN]${RESET} ${msg}" ;;
        ERROR) coloured="${RED}[ERROR]${RESET} ${msg}" ;;
        STEP)  coloured="${BOLD}${GREEN}>>> ${msg}${RESET}" ;;
        *)     coloured="${msg}" ;;
    esac
    echo -e "${ts}  ${coloured}" | tee -a "${SMOKE_LOG_FILE}"
}

log_raw() { echo "$*" | tee -a "${SMOKE_LOG_FILE}"; }
banner() {
    log_raw ""
    log_raw "=================================================================="
    log_raw "  $*"
    log_raw "=================================================================="
    log_raw ""
}
die() { log ERROR "$*"; exit 1; }

# ---------------------------------------------------------------------------
# State file helpers (JSON-backed, jq-driven)
# ---------------------------------------------------------------------------
state_init() {
    if [[ ! -f "${SMOKE_STATE_FILE}" ]]; then
        echo '{"phases":{}}' > "${SMOKE_STATE_FILE}"
        log INFO "Created new state file: ${SMOKE_STATE_FILE}"
    else
        log INFO "Resuming from existing state file: ${SMOKE_STATE_FILE}"
    fi
}

state_get() {
    local key="$1"
    jq -r ".${key} // empty" "${SMOKE_STATE_FILE}" 2>/dev/null || true
}

state_set() {
    local key="$1"
    local value="$2"
    local tmp; tmp=$(mktemp)
    jq --arg k "${key}" --arg v "${value}" \
        'setpath(($k | split(".") | map(select(length>0))); $v)' \
        "${SMOKE_STATE_FILE}" > "${tmp}" && mv "${tmp}" "${SMOKE_STATE_FILE}"
}

phase_done() {
    local phase="$1"
    [[ "$(state_get "phases.${phase}")" == "done" ]]
}

phase_mark_done() {
    local phase="$1"
    state_set "phases.${phase}" "done"
    log OK "Phase '${phase}' marked complete"
}

# ---------------------------------------------------------------------------
# HTTP / retry helpers
# ---------------------------------------------------------------------------

# curl wrapper: retries with exponential back-off
# Usage: api_call <method> <path> [body-json]
# Returns the response body; exits non-zero on persistent failure
api_call() {
    local method="$1"
    local path="$2"
    local body="${3:-}"
    local url="${REDISMETER_URL}${path}"
    local max_retries=5
    local delay=2
    local attempt=0
    local response
    local http_code

    while (( attempt < max_retries )); do
        attempt=$(( attempt + 1 ))
        local curl_args=( -s -w "\n%{http_code}" -X "${method}" -H "Content-Type: application/json" )
        if [[ -n "${body}" ]]; then
            curl_args+=( --data "${body}" )
        fi
        curl_args+=( "${url}" )

        local raw; raw=$(curl "${curl_args[@]}" 2>>"${SMOKE_LOG_FILE}") || true
        http_code=$(echo "${raw}" | tail -n 1)
        response=$(echo "${raw}" | sed '$d')

        if [[ "${http_code}" =~ ^[23] ]]; then
            echo "${response}"
            return 0
        fi

        log WARN "Attempt ${attempt}/${max_retries}: ${method} ${path} → HTTP ${http_code}"
        if [[ "${attempt}" -lt "${max_retries}" ]]; then
            log INFO "Retrying in ${delay}s…"
            sleep "${delay}"
            delay=$(( delay * 2 ))
        fi
    done

    log ERROR "All ${max_retries} attempts failed for ${method} ${path} (last HTTP ${http_code})"
    log ERROR "Last response: ${response}"
    return 1
}

# Poll a GET endpoint until a JSON field equals a value (or hits an error value)
# Usage: wait_for_status <path> <field> <ok_value> <err_value> <max_secs> <poll_interval>
wait_for_status() {
    local path="$1"
    local field="$2"
    local ok_value="$3"
    local err_value="$4"
    local max_secs="$5"
    local poll_interval="${6:-10}"
    local elapsed=0

    log INFO "Waiting for ${field}=${ok_value} on ${path} (timeout ${max_secs}s)"
    while (( elapsed < max_secs )); do
        local resp; resp=$(api_call GET "${path}") || true
        local current; current=$(echo "${resp}" | jq -r ".${field} // empty" 2>/dev/null || true)

        if [[ "${current}" == "${ok_value}" ]]; then
            log OK "${field}=${current} ✓"
            echo "${resp}"
            return 0
        fi

        if [[ -n "${err_value}" && "${current}" == "${err_value}" ]]; then
            local err_msg; err_msg=$(echo "${resp}" | jq -r '.error // .message // "unknown error"' 2>/dev/null || true)
            die "${field}=${current} (error). Message: ${err_msg}"
        fi

        log INFO "  ${field}=${current} – waiting ${poll_interval}s (${elapsed}/${max_secs}s elapsed)"
        sleep "${poll_interval}"
        elapsed=$(( elapsed + poll_interval ))
    done

    die "Timeout waiting for ${field}=${ok_value} on ${path} after ${max_secs}s"
}

# ---------------------------------------------------------------------------
# Phase 0: Preflight
# ---------------------------------------------------------------------------
phase_preflight() {
    banner "Phase 0 – Preflight checks"
    phase_done "preflight" && { log OK "Preflight already passed, skipping"; return; }

    log STEP "Checking required tools…"
    for tool in curl jq; do
        if ! command -v "${tool}" &>/dev/null; then
            die "Required tool '${tool}' not found. Please install it."
        fi
        log OK "  ${tool} found: $(command -v "${tool}")"
    done

    log STEP "Checking server health…"
    local health; health=$(api_call GET /health) || die "Cannot reach RedisMeter server at ${REDISMETER_URL}"
    local status; status=$(echo "${health}" | jq -r '.status')
    local storage_ok; storage_ok=$(echo "${health}" | jq -r '.storage.healthy')
    log OK "Server status: ${status}, storage healthy: ${storage_ok}"
    [[ "${status}" == "healthy" ]] || die "Server is not healthy: ${health}"
    [[ "${storage_ok}" == "true" ]] || die "Storage is not healthy: ${health}"

    log STEP "Checking Azure CLI…"
    if command -v az &>/dev/null; then
        local az_account; az_account=$(az account show --query '{sub:id,name:name}' -o json 2>/dev/null || true)
        if [[ -n "${az_account}" ]]; then
            local sub_id; sub_id=$(echo "${az_account}" | jq -r '.sub')
            local sub_name; sub_name=$(echo "${az_account}" | jq -r '.name')
            log OK "Azure CLI logged in: sub=${sub_id} (${sub_name})"
            state_set "azure.subscription_id" "${sub_id}"
            state_set "azure.subscription_name" "${sub_name}"
        else
            log WARN "Azure CLI found but not logged in – infra provisioning may fail"
            log WARN "Run: az login"
        fi
    else
        log WARN "Azure CLI (az) not found – infrastructure provisioning via Terraform may not work"
    fi

    phase_mark_done "preflight"
}

# ---------------------------------------------------------------------------
# Phase 1: Discover workloads & run profiles
# ---------------------------------------------------------------------------
phase_discover() {
    banner "Phase 1 – Discover workloads & run profiles"
    phase_done "discover" && { log OK "Discovery already done, skipping"; return; }

    log STEP "Listing workloads…"
    local workloads; workloads=$(api_call GET "/api/v1/workloads")
    local wl_count; wl_count=$(echo "${workloads}" | jq '.workloads | length')
    log OK "Found ${wl_count} workloads:"
    echo "${workloads}" | jq -r '.workloads[] | "    \(.name): \(.description)"' | tee -a "${SMOKE_LOG_FILE}"

    # Confirm target workload exists
    local found_wl; found_wl=$(echo "${workloads}" | jq -r --arg n "${BENCHMARK_WORKLOAD}" '.workloads[] | select(.name==$n) | .name' || true)
    [[ -n "${found_wl}" ]] || die "Workload '${BENCHMARK_WORKLOAD}' not found in server!"

    log STEP "Listing run profiles…"
    local profiles; profiles=$(api_call GET "/api/v1/run-profiles")
    local rp_count; rp_count=$(echo "${profiles}" | jq '.profiles | length // (.run_profiles | length) // 0')
    log OK "Found ${rp_count} run profiles:"
    echo "${profiles}" | jq -r '(.profiles // .run_profiles) [] | "    \(.name): \(.description // "")"' | tee -a "${SMOKE_LOG_FILE}" || true

    phase_mark_done "discover"
}

# ---------------------------------------------------------------------------
# Phase 2: Create infrastructure profile (stored template)
# ---------------------------------------------------------------------------
phase_create_infra_profile() {
    banner "Phase 2 – Create infrastructure profile"
    phase_done "infra_profile" && { log OK "Infra profile already created, skipping"; return; }

    log STEP "Creating Azure infrastructure profile '${INFRA_NAME}'…"
    local sub_id; sub_id=$(state_get "azure.subscription_id")
    local profile_body
    profile_body=$(jq -n \
        --arg name "${INFRA_NAME}" \
        --arg sub "${sub_id}" \
        --arg region "${AZURE_REGION}" \
        --arg sku "${INFRA_SKU}" \
        --argjson ha "${INFRA_HA}" \
        '{
            "name": $name,
            "description": "Production Azure AMR profile – showcase smoke test",
            "provider": "azure",
            "tags": ["production","smoke-test"],
            "config": {
                "azure": {
                    "subscription_id": $sub,
                    "location": $region,
                    "sku": $sku,
                    "high_availability": $ha,
                    "clustering_policy": "non-clustered",
                    "eviction_policy": "allkeys-lru",
                    "access_keys_auth": true,
                    "allow_public_access": true,
                    "estimated_monthly_cost": 0
                }
            }
        }')

    local resp; resp=$(api_call POST "/api/v1/infrastructure-profiles" "${profile_body}") || die "Failed to create infra profile"
    local profile_id; profile_id=$(echo "${resp}" | jq -r '.id')
    log OK "Created infra profile: id=${profile_id}"
    state_set "infra_profile.id" "${profile_id}"
    state_set "infra_profile.name" "${INFRA_NAME}"

    phase_mark_done "infra_profile"
}

# ---------------------------------------------------------------------------
# Phase 3: Deploy Azure infrastructure
# ---------------------------------------------------------------------------
phase_deploy_infra() {
    banner "Phase 3 – Deploy Azure infrastructure"
    phase_done "deploy_infra" && { log OK "Infrastructure already deployed, skipping"; return; }

    log STEP "Provisioning Azure AMR + runner VMs…"

    # Read SSH public key if available
    local ssh_pub_key=""
    for keyfile in "${HOME}/.ssh/id_ed25519.pub" "${HOME}/.ssh/id_rsa.pub"; do
        if [[ -f "${keyfile}" ]]; then
            ssh_pub_key=$(cat "${keyfile}")
            log INFO "Using SSH public key: ${keyfile}"
            break
        fi
    done

    local infra_body
    infra_body=$(jq -n \
        --arg name "${INFRA_NAME}" \
        --arg region "${AZURE_REGION}" \
        --arg sku "${INFRA_SKU}" \
        --argjson ha "${INFRA_HA}" \
        --argjson runner_count "${RUNNER_COUNT}" \
        --arg vm_size "${RUNNER_VM_SIZE}" \
        --arg ssh_key "${ssh_pub_key}" \
        '{
            "name": $name,
            "provider": "azure",
            "region": $region,
            "tags": {"environment": "production", "managed-by": "redismeter", "purpose": "smoke-test"},
            "amr": {
                "sku": $sku,
                "high_availability": $ha,
                "clustering_policy": "non-clustered",
                "eviction_policy": "allkeys-lru"
            },
            "runners": {
                "count": $runner_count,
                "instance_type": $vm_size,
                "spot_instances": false,
                "ssh_public_key": $ssh_key,
                "ssh_user": "azureuser"
            }
        }')

    local resp; resp=$(api_call POST "/api/v1/infrastructures" "${infra_body}") || die "Failed to create infrastructure"
    local infra_id; infra_id=$(echo "${resp}" | jq -r '.id')
    local infra_status; infra_status=$(echo "${resp}" | jq -r '.status')
    log OK "Infrastructure provisioning started: id=${infra_id}, status=${infra_status}"
    state_set "infra.id" "${infra_id}"

    phase_mark_done "deploy_infra"
}

# ---------------------------------------------------------------------------
# Phase 4: Wait for infrastructure ready
# ---------------------------------------------------------------------------
# find_infra_state_file: locate the on-disk redismeter-state.json for INFRA_NAME
# Returns the path, or empty if not found.
find_infra_state_file() {
    local name="${1:-${INFRA_NAME}}"
    local tf_dir="${HOME}/.redismeter/terraform"
    for f in "${tf_dir}"/rm-*/redismeter-state.json; do
        [[ -f "${f}" ]] || continue
        local n; n=$(jq -r '.name // empty' "${f}" 2>/dev/null || true)
        if [[ "${n}" == "${name}" ]]; then
            echo "${f}"
            return 0
        fi
    done
}

phase_wait_infra() {
    banner "Phase 4 – Wait for infrastructure to be ready"
    phase_done "wait_infra" && { log OK "Infrastructure ready check already passed, skipping"; return; }

    local infra_id; infra_id=$(state_get "infra.id")
    [[ -n "${infra_id}" ]] || die "infra.id not found in state file. Did phase 3 run?"

    # ------------------------------------------------------------------
    # The API returns a transient "pending-*" ID. The real ID is generated
    # by Terraform and stored on disk as rm-<name>-<timestamp>.
    # Resolve the real ID first by reading on-disk state files.
    # ------------------------------------------------------------------
    if [[ "${infra_id}" == pending-* ]]; then
        log INFO "Resolving pending infra ID '${infra_id}' → real ID (polling disk, max 120s)…"
        local elapsed=0 state_file=""
        while (( elapsed < 120 )); do
            state_file=$(find_infra_state_file "${INFRA_NAME}")
            if [[ -n "${state_file}" ]]; then
                infra_id=$(jq -r '.id' "${state_file}")
                log OK "Resolved real infra ID: ${infra_id}"
                state_set "infra.id" "${infra_id}"
                break
            fi
            log INFO "  State file not yet on disk – waiting 10s (${elapsed}/120s)…"
            sleep 10
            elapsed=$(( elapsed + 10 ))
        done
        [[ -n "${state_file}" ]] || die "Could not resolve real infra ID from pending ID. State file not found after 120s."
    fi

    # ------------------------------------------------------------------
    # Poll until ready.  Primary strategy: read on-disk state file
    # (bypasses the API mutex that blocks while Terraform is running).
    # Secondary: try the API once the mutex is released.
    # ------------------------------------------------------------------
    local tf_dir="${HOME}/.redismeter/terraform"
    local state_file="${tf_dir}/${infra_id}/redismeter-state.json"
    local elapsed=0
    local poll_interval=15

    log INFO "Polling on-disk state for '${infra_id}' (max ${MAX_INFRA_WAIT_SECS}s)…"
    while (( elapsed < MAX_INFRA_WAIT_SECS )); do
        local status="" err_msg=""
        if [[ -f "${state_file}" ]]; then
            status=$(jq -r '.status // empty' "${state_file}" 2>/dev/null || true)
            err_msg=$(jq -r '.error // empty' "${state_file}" 2>/dev/null || true)
        fi

        case "${status}" in
            ready)
                log OK "Infrastructure status=ready ✓ (disk)"
                break
                ;;
            failed)
                die "Infrastructure provisioning failed: ${err_msg}"
                ;;
            "")
                log INFO "  State file missing or unreadable – waiting ${poll_interval}s (${elapsed}/${MAX_INFRA_WAIT_SECS}s)…"
                ;;
            *)
                log INFO "  status=${status} – waiting ${poll_interval}s (${elapsed}/${MAX_INFRA_WAIT_SECS}s)…"
                ;;
        esac

        sleep "${poll_interval}"
        elapsed=$(( elapsed + poll_interval ))
    done

    if [[ "${status}" != "ready" ]]; then
        # One last check: try the API (may be unblocked now)
        log INFO "Disk polling timed out – attempting API check…"
        local api_resp; api_resp=$(curl -s --max-time 30 "${REDISMETER_URL}/api/v1/infrastructures/${infra_id}" 2>/dev/null || true)
        status=$(echo "${api_resp}" | jq -r '.status // empty' 2>/dev/null || true)
        [[ "${status}" == "ready" ]] || die "Timeout: infrastructure not ready after ${MAX_INFRA_WAIT_SECS}s (last status=${status})"
    fi

    # ------------------------------------------------------------------
    # Read outputs.  First try the API (clean), fall back to disk state.
    # ------------------------------------------------------------------
    local final_resp; final_resp=$(curl -s --max-time 30 "${REDISMETER_URL}/api/v1/infrastructures/${infra_id}" 2>/dev/null || true)
    if [[ -z "${final_resp}" || "$(echo "${final_resp}" | jq -r '.status // empty' 2>/dev/null)" != "ready" ]]; then
        log INFO "API not yet responsive; reading outputs from disk state file"
        final_resp=$(cat "${state_file}" 2>/dev/null || echo '{}')
    fi

    local redis_host; redis_host=$(echo "${final_resp}" | jq -r '.outputs.redis_hostname // empty')
    local redis_port; redis_port=$(echo "${final_resp}" | jq -r '.outputs.redis_port // empty')
    local redis_key; redis_key=$(echo "${final_resp}" | jq -r '.outputs.redis_primary_key // empty')
    local runner_ips; runner_ips=$(echo "${final_resp}" | jq -r '(.outputs.runner_ips // []) | join(",")' 2>/dev/null || true)

    log OK "Redis endpoint : ${redis_host}:${redis_port}"
    log OK "Runner IPs     : ${runner_ips}"

    state_set "infra.redis_host" "${redis_host}"
    state_set "infra.redis_port" "${redis_port}"
    state_set "infra.redis_key"  "${redis_key}"
    state_set "infra.runner_ips" "${runner_ips}"

    phase_mark_done "wait_infra"
}

# ---------------------------------------------------------------------------
# Helper: Start a cloud benchmark and return the benchmark ID
# ---------------------------------------------------------------------------
start_cloud_benchmark() {
    local label="$1"
    local infra_id; infra_id=$(state_get "infra.id")
    [[ -n "${infra_id}" ]] || die "infra.id not in state"

    log STEP "Starting cloud benchmark '${label}' (workload=${BENCHMARK_WORKLOAD})…" >&2
    local body
    body=$(jq -n \
        --arg infra "${infra_id}" \
        --arg wl "${BENCHMARK_WORKLOAD}" \
        '{
            "infrastructure_id": $infra,
            "workload": $wl
        }')

    local resp; resp=$(api_call POST "/api/v1/cloud/benchmark" "${body}") || die "Failed to start benchmark ${label}"
    local bench_id; bench_id=$(echo "${resp}" | jq -r '.benchmark_id')
    local status; status=$(echo "${resp}" | jq -r '.status')
    log OK "Benchmark started: id=${bench_id}, status=${status}" >&2
    echo "${bench_id}"
}

# ---------------------------------------------------------------------------
# Helper: Wait for benchmark to complete and return its stored run ID
# ---------------------------------------------------------------------------
wait_for_benchmark() {
    local bench_id="$1"
    log INFO "Polling benchmark ${bench_id} (max ${MAX_BENCH_WAIT_SECS}s)…" >&2

    local final_resp; final_resp=$(wait_for_status \
        "/api/v1/benchmark/${bench_id}" \
        "status" \
        "completed" \
        "failed" \
        "${MAX_BENCH_WAIT_SECS}" \
        10)

    local progress; progress=$(echo "${final_resp}" | jq -r '.progress // "?"')
    log OK "Benchmark ${bench_id} completed (progress=${progress})" >&2

    # The cloud benchmark saves to storage using the benchmark_id as the run ID
    echo "${bench_id}"
}

# ---------------------------------------------------------------------------
# Phase 5: Run benchmark 1
# ---------------------------------------------------------------------------
phase_benchmark_1() {
    banner "Phase 5 – Benchmark run 1"
    phase_done "benchmark_1" && { log OK "Benchmark 1 already complete, skipping"; return; }

    local bench_id; bench_id=$(start_cloud_benchmark "run-1")
    state_set "benchmark1.bench_id" "${bench_id}"

    local run_id; run_id=$(wait_for_benchmark "${bench_id}")
    state_set "benchmark1.run_id" "${run_id}"
    log OK "Benchmark 1 run ID: ${run_id}"

    # Verify run is queryable
    local run_resp; run_resp=$(api_call GET "/api/v1/runs/${run_id}") || true
    if [[ -n "${run_resp}" ]]; then
        local ops; ops=$(echo "${run_resp}" | jq -r '.results.summary.ops_per_second // "N/A"')
        local p99; p99=$(echo "${run_resp}" | jq -r '.results.summary.p99_latency_ms // "N/A"')
        log OK "Run 1 metrics → ops/sec=${ops}, p99_latency_ms=${p99}"
    fi

    phase_mark_done "benchmark_1"
}

# ---------------------------------------------------------------------------
# Phase 6: Run benchmark 2
# ---------------------------------------------------------------------------
phase_benchmark_2() {
    banner "Phase 6 – Benchmark run 2"
    phase_done "benchmark_2" && { log OK "Benchmark 2 already complete, skipping"; return; }

    local bench_id; bench_id=$(start_cloud_benchmark "run-2")
    state_set "benchmark2.bench_id" "${bench_id}"

    local run_id; run_id=$(wait_for_benchmark "${bench_id}")
    state_set "benchmark2.run_id" "${run_id}"
    log OK "Benchmark 2 run ID: ${run_id}"

    local run_resp; run_resp=$(api_call GET "/api/v1/runs/${run_id}") || true
    if [[ -n "${run_resp}" ]]; then
        local ops; ops=$(echo "${run_resp}" | jq -r '.results.summary.ops_per_second // "N/A"')
        local p99; p99=$(echo "${run_resp}" | jq -r '.results.summary.p99_latency_ms // "N/A"')
        log OK "Run 2 metrics → ops/sec=${ops}, p99_latency_ms=${p99}"
    fi

    phase_mark_done "benchmark_2"
}

# ---------------------------------------------------------------------------
# Phase 7: Save runs as baselines
# ---------------------------------------------------------------------------
phase_save_baselines() {
    banner "Phase 7 – Save runs as baselines"
    phase_done "baselines" && { log OK "Baselines already saved, skipping"; return; }

    local run1_id; run1_id=$(state_get "benchmark1.run_id")
    local run2_id; run2_id=$(state_get "benchmark2.run_id")
    [[ -n "${run1_id}" ]] || die "benchmark1.run_id not in state"
    [[ -n "${run2_id}" ]] || die "benchmark2.run_id not in state"

    log STEP "Fetching run 1 to build baseline…"
    local run1_resp; run1_resp=$(api_call GET "/api/v1/runs/${run1_id}") || die "Cannot fetch run 1"
    local ops1; ops1=$(echo "${run1_resp}" | jq -r '.results.summary.ops_per_second // 0')
    local lat1; lat1=$(echo "${run1_resp}" | jq -r '.results.summary.avg_latency_ms // 0')
    local p99_1; p99_1=$(echo "${run1_resp}" | jq -r '.results.summary.p99_latency_ms // 0')

    log STEP "Saving run 1 as baseline 'production-cache-baseline-1'…"
    local bl1_entity_id="production-cache-baseline-1-${run1_id}"
    local bl1_body
    bl1_body=$(jq -n \
        --arg id "${bl1_entity_id}" \
        --arg run_id "${run1_id}" \
        --arg name "production-cache-baseline-1" \
        --argjson ops "${ops1}" \
        --argjson lat "${lat1}" \
        --argjson p99 "${p99_1}" \
        '{
            "id": $id,
            "run_id": $run_id,
            "name": $name,
            "description": "Baseline from smoke-test run 1",
            "tags": ["production","cache","smoke-test"],
            "active": true,
            "metrics": {
                "total_requests": 0,
                "total_ops": 0,
                "ops_per_second": $ops,
                "avg_latency_ms": $lat,
                "p99_latency_ms": $p99,
                "errors": 0,
                "error_rate": 0
            },
            "workload": {
                "name": "cache",
                "type": "cache",
                "operations": [
                    {"command": "GET", "ratio": 0.8},
                    {"command": "SET", "ratio": 0.2}
                ]
            },
            "thresholds": {
                "max_throughput_regression": 0.10,
                "max_latency_regression": 0.20,
                "max_p99_regression": 0.25,
                "max_error_rate_increase": 0.01
            }
        }')

    local bl1_resp; bl1_resp=$(api_call POST "/api/v1/baselines" "${bl1_body}") || die "Failed to save baseline 1"
    local bl1_id; bl1_id=$(echo "${bl1_resp}" | jq -r '.id')
    log OK "Baseline 1 created: id=${bl1_id}"
    state_set "baseline1.id" "${bl1_id}"

    log STEP "Saving run 2 as baseline 'production-cache-baseline-2'…"
    local bl2_entity_id="production-cache-baseline-2-${run2_id}"
    local run2_resp; run2_resp=$(api_call GET "/api/v1/runs/${run2_id}") || die "Cannot fetch run 2"
    local ops2; ops2=$(echo "${run2_resp}" | jq -r '.results.summary.ops_per_second // 0')
    local lat2; lat2=$(echo "${run2_resp}" | jq -r '.results.summary.avg_latency_ms // 0')
    local p99_2; p99_2=$(echo "${run2_resp}" | jq -r '.results.summary.p99_latency_ms // 0')

    local bl2_body
    bl2_body=$(jq -n \
        --arg id "${bl2_entity_id}" \
        --arg run_id "${run2_id}" \
        --arg name "production-cache-baseline-2" \
        --argjson ops "${ops2}" \
        --argjson lat "${lat2}" \
        --argjson p99 "${p99_2}" \
        '{
            "id": $id,
            "run_id": $run_id,
            "name": $name,
            "description": "Baseline from smoke-test run 2",
            "tags": ["production","cache","smoke-test"],
            "active": true,
            "metrics": {
                "total_requests": 0,
                "total_ops": 0,
                "ops_per_second": $ops,
                "avg_latency_ms": $lat,
                "p99_latency_ms": $p99,
                "errors": 0,
                "error_rate": 0
            },
            "workload": {
                "name": "cache",
                "type": "cache",
                "operations": [
                    {"command": "GET", "ratio": 0.8},
                    {"command": "SET", "ratio": 0.2}
                ]
            },
            "thresholds": {
                "max_throughput_regression": 0.10,
                "max_latency_regression": 0.20,
                "max_p99_regression": 0.25,
                "max_error_rate_increase": 0.01
            }
        }')

    local bl2_resp; bl2_resp=$(api_call POST "/api/v1/baselines" "${bl2_body}") || die "Failed to save baseline 2"
    local bl2_id; bl2_id=$(echo "${bl2_resp}" | jq -r '.id')
    log OK "Baseline 2 created: id=${bl2_id}"
    state_set "baseline2.id" "${bl2_id}"

    phase_mark_done "baselines"
}

# ---------------------------------------------------------------------------
# Phase 8: Compare runs
# ---------------------------------------------------------------------------
phase_compare_runs() {
    banner "Phase 8 – Compare runs"
    phase_done "compare" && { log OK "Comparison already done, skipping"; return; }

    local run1_id; run1_id=$(state_get "benchmark1.run_id")
    local run2_id; run2_id=$(state_get "benchmark2.run_id")
    [[ -n "${run1_id}" ]] || die "benchmark1.run_id not in state"
    [[ -n "${run2_id}" ]] || die "benchmark2.run_id not in state"

    log STEP "Comparing run 1 (${run1_id}) vs run 2 (${run2_id})…"
    local compare_body
    compare_body=$(jq -n \
        --arg r1 "${run1_id}" \
        --arg r2 "${run2_id}" \
        '{"run_id_1": $r1, "run_id_2": $r2}')

    local resp; resp=$(api_call POST "/api/v1/compare" "${compare_body}") || die "Failed to compare runs"

    log OK "Comparison result:"
    echo "${resp}" | jq '.' | tee -a "${SMOKE_LOG_FILE}"

    local throughput_change; throughput_change=$(echo "${resp}" | jq -r '.throughput_change_pct // .metrics.throughput_change // .metrics.throughput.change_pct // "N/A"')
    local latency_change; latency_change=$(echo "${resp}" | jq -r '.latency_change_pct // .metrics.avg_latency_change // .metrics.avg_latency.change_pct // "N/A"')
    local verdict; verdict=$(echo "${resp}" | jq -r '.verdict // (if (.metrics.throughput.change_pct != null and .metrics.avg_latency.change_pct != null) then (if (.metrics.throughput.change_pct >= 0 and .metrics.avg_latency.change_pct <= 0) then "pass" else "warning" end) else "N/A" end)')
    local comparable; comparable=$(echo "${resp}" | jq -r '.comparable // "unknown"')
    local quality; quality=$(echo "${resp}" | jq -r '.comparison_quality // "unknown"')
    local blocker_count; blocker_count=$(echo "${resp}" | jq -r '(.blocking_differences // []) | length')
    local warning_count; warning_count=$(echo "${resp}" | jq -r '(.warnings // []) | length')

    state_set "compare.throughput_change" "${throughput_change}"
    state_set "compare.latency_change" "${latency_change}"
    state_set "compare.verdict" "${verdict}"
    state_set "compare.comparable" "${comparable}"
    state_set "compare.quality" "${quality}"
    state_set "compare.blocker_count" "${blocker_count}"
    state_set "compare.warning_count" "${warning_count}"

    phase_mark_done "compare"
}

# ---------------------------------------------------------------------------
# Phase 9: Final report
# ---------------------------------------------------------------------------
phase_final_report() {
    banner "Smoke Test Final Report"

    local infra_id; infra_id=$(state_get "infra.id")
    local redis_host; redis_host=$(state_get "infra.redis_host")
    local redis_port; redis_port=$(state_get "infra.redis_port")
    local run1_id; run1_id=$(state_get "benchmark1.run_id")
    local run2_id; run2_id=$(state_get "benchmark2.run_id")
    local bl1_id; bl1_id=$(state_get "baseline1.id")
    local bl2_id; bl2_id=$(state_get "baseline2.id")
    local throughput_chg; throughput_chg=$(state_get "compare.throughput_change")
    local latency_chg; latency_chg=$(state_get "compare.latency_change")
    local verdict; verdict=$(state_get "compare.verdict")
    local comparable; comparable=$(state_get "compare.comparable")
    local quality; quality=$(state_get "compare.quality")
    local blocker_count; blocker_count=$(state_get "compare.blocker_count")
    local warning_count; warning_count=$(state_get "compare.warning_count")

    echo -e "${BOLD}${GREEN}"
    cat <<EOF
  ╔══════════════════════════════════════════════════════╗
  ║          RedisMeter Smoke Test – COMPLETE            ║
  ╚══════════════════════════════════════════════════════╝

  Infrastructure
  ──────────────
  ID          : ${infra_id}
  Redis host  : ${redis_host}:${redis_port}
  NOTE: Infrastructure is PRESERVED (not destroyed)

  Benchmark Runs
  ──────────────
  Run 1 ID    : ${run1_id}
  Run 2 ID    : ${run2_id}

  Baselines
  ─────────
  Baseline 1  : ${bl1_id}
  Baseline 2  : ${bl2_id}

  Comparison (Run 1 → Run 2)
  ──────────────────────────
    Comparable   : ${comparable}
    Quality      : ${quality}
    Blockers     : ${blocker_count}
    Warnings     : ${warning_count}
  Throughput Δ : ${throughput_chg}%
  Latency Δ    : ${latency_chg}%
  Verdict      : ${verdict}

  Artefacts
  ─────────
  State file   : ${SMOKE_STATE_FILE}
  Log file     : ${SMOKE_LOG_FILE}
EOF
    echo -e "${RESET}"

    log OK "Smoke test completed successfully!"
    log INFO "To re-run any phase, delete it from the state file: ${SMOKE_STATE_FILE}"
    log INFO "To destroy infra later: curl -s -X DELETE ${REDISMETER_URL}/api/v1/infrastructures/${infra_id}"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    banner "RedisMeter Full Showcase Smoke Test"
    log INFO "Server    : ${REDISMETER_URL}"
    log INFO "Workload  : ${BENCHMARK_WORKLOAD}"
    log INFO "Profile   : ${BENCHMARK_RUN_PROFILE}"
    log INFO "Azure SKU : ${INFRA_SKU}  region=${AZURE_REGION}  HA=${INFRA_HA}"
    log INFO "State file: ${SMOKE_STATE_FILE}"
    log INFO "Log file  : ${SMOKE_LOG_FILE}"
    log_raw ""

    state_init

    phase_preflight
    phase_discover
    phase_create_infra_profile
    phase_deploy_infra
    phase_wait_infra
    phase_benchmark_1
    phase_benchmark_2
    phase_save_baselines
    phase_compare_runs
    phase_final_report
}

main "$@"
