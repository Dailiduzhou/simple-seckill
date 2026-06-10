#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

export SCRIPT_PATH="${SCRIPT_PATH:-${ROOT_DIR}/deploy/k6/seckill-degrade.js}"
export JOB_NAME="${JOB_NAME:-seckill-bbr-k6}"
export NAMESPACE="${NAMESPACE:-default}"
export BASE_URL="${BASE_URL:-http://product:8000}"
export SECKILL_PATH="${SECKILL_PATH:-/api/seckill}"
export STATUS_PATH="${STATUS_PATH:-/api/seckill/status}"
export USER_COUNT="${USER_COUNT:-50000}"
export START_USER_ID="${START_USER_ID:-1}"

# BBR is sensitive to load spikes, so this profile ramps quickly and holds pressure.
export K6_STAGES="${K6_STAGES:-10s:200,20s:800,30s:1500,20s:1500,10s:0}"
export HTTP_TIMEOUT="${HTTP_TIMEOUT:-3s}"
export STATUS_TIMEOUT_MS="${STATUS_TIMEOUT_MS:-4000}"
export POLL_INTERVAL_MS="${POLL_INTERVAL_MS:-100}"
export SLEEP_MS="${SLEEP_MS:-0}"

echo "Running BBR load test with:"
echo "  SCRIPT_PATH=${SCRIPT_PATH}"
echo "  JOB_NAME=${JOB_NAME}"
echo "  BASE_URL=${BASE_URL}"
echo "  K6_STAGES=${K6_STAGES}"
echo "  HTTP_TIMEOUT=${HTTP_TIMEOUT}"
echo "  STATUS_TIMEOUT_MS=${STATUS_TIMEOUT_MS}"
echo "  POLL_INTERVAL_MS=${POLL_INTERVAL_MS}"

exec "${ROOT_DIR}/deploy/k6/run-k6.sh"
