#!/usr/bin/env bash
# =============================================================================
# post-terraform.sh
# Run after manual terraform apply to:
#  1. Extract terraform outputs
#  2. Update the redismeter-state.json on disk (status=ready + outputs)
#  3. Resume the smoke test
# =============================================================================
set -euo pipefail

INFRA_ID="${1:-rm-redismeter-prod-20260506-201347}"
TF_DIR="${HOME}/.redismeter/terraform/${INFRA_ID}"
STATE_FILE="${TF_DIR}/redismeter-state.json"

echo "=== Extracting terraform outputs for ${INFRA_ID} ==="
cd "${TF_DIR}"

REDIS_HOST=$(terraform output -raw redis_hostname 2>/dev/null || true)
REDIS_PORT=$(terraform output -raw redis_port 2>/dev/null || echo "10000")
REDIS_KEY=$(terraform output -raw redis_primary_key 2>/dev/null || true)
RUNNER_IPS=$(terraform output -json runner_ips 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); print(','.join(d) if isinstance(d,list) else '')" 2>/dev/null || true)
RESOURCE_GROUP=$(terraform output -raw resource_group_name 2>/dev/null || true)

echo "  Redis Host : ${REDIS_HOST}"
echo "  Redis Port : ${REDIS_PORT}"
echo "  Runner IPs : ${RUNNER_IPS}"
echo "  RG         : ${RESOURCE_GROUP}"

echo "=== Updating on-disk state file ==="
UPDATED=$(python3 - <<PYEOF
import json, sys
with open('${STATE_FILE}') as f:
    s = json.load(f)
s['status'] = 'ready'
s['error'] = ''
s['outputs'] = {
    'redis_hostname':      '${REDIS_HOST}',
    'redis_port':          int('${REDIS_PORT}') if '${REDIS_PORT}' else 10000,
    'redis_primary_key':   '${REDIS_KEY}',
    'resource_group_name': '${RESOURCE_GROUP}',
    'runner_ips':          [ip for ip in '${RUNNER_IPS}'.split(',') if ip],
    'runner_private_ips':  []
}
print(json.dumps(s, indent=2, default=str))
PYEOF
)
echo "${UPDATED}" > "${STATE_FILE}"
echo "State file updated to status=ready"

echo "=== Running smoke test (will resume from phase 4 / wait_infra) ==="
cd /Users/thomas.findelkind/Code/RedisMeter
bash scripts/smoke-test.sh
