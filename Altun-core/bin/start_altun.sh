#!/bin/bash
# start_altun.sh — launch the Altun control plane on a driver host:
#   1. build cft-adapter, bft-adapter and altun-controller
#   2. copy this directory to the driver host and start altun-controller there
#   3. the controller launches the initial adapter (cft-adapter or bft-adapter)
#
# Usage:
#   bash start_altun.sh
#
# Environment (defaults in parentheses):
#   DRIVER_HOST     host that runs the controller and the adapters (localhost)
#   DRIVER_DIR      directory on DRIVER_HOST that receives a copy of Altun-core (REMOTE_BASE/altun-driver)
#   START_NODE      first cluster host used by the adapters (env.sh)
#   INIT_PROTOCOL   initial consensus protocol (paxos)
#   INIT_MODE       initial fault mode: CFT or BFT (CFT)
#   CONTROLLER_PORT HTTP port of the controller API (8080)
#   GRPC_BASE_PORT  base port of the adapter gRPC servers (9100)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CORE_DIR="$(dirname "$SCRIPT_DIR")"
. "${SCRIPT_DIR}/env.sh"

DRIVER_HOST="${DRIVER_HOST:-localhost}"
DRIVER_DIR="${DRIVER_DIR:-${REMOTE_BASE}/altun-driver}"
INIT_PROTOCOL="${INIT_PROTOCOL:-paxos}"
INIT_MODE="${INIT_MODE:-CFT}"
CONTROLLER_PORT="${CONTROLLER_PORT:-8080}"
GRPC_BASE_PORT="${GRPC_BASE_PORT:-9100}"
PROJECT_DIR="${DRIVER_DIR}/Altun-core/bin"

run_on_driver() { # command string
  if [ "$DRIVER_HOST" = localhost ]; then bash -c "$1"; else ssh $ssh_options "$ssh_user@$DRIVER_HOST" "$1"; fi
}

echo "=========================================="
echo " Altun control plane"
echo "=========================================="
echo " Driver:         ${DRIVER_HOST}:${DRIVER_DIR}"
echo " Cluster:        $(host_of 1) .. (START_NODE=${START_NODE})"
echo " Protocol:       ${INIT_PROTOCOL} (${INIT_MODE})"
echo " Controller:     http://${DRIVER_HOST}:${CONTROLLER_PORT}"
echo " gRPC base port: ${GRPC_BASE_PORT}"
echo "=========================================="

# ── Step 1: build the control-plane binaries ──
echo "[1/3] Building binaries..."
cd "${CORE_DIR}"
go build -o bin/cft-adapter      ./cmd/cft-adapter/
go build -o bin/bft-adapter      ./cmd/bft-adapter/
go build -o bin/altun-controller ./cmd/altun-controller/
echo "  cft-adapter, bft-adapter, altun-controller built"

# ── Step 2: copy Altun-core to the driver ──
echo "[2/3] Copying Altun-core to ${DRIVER_HOST}:${DRIVER_DIR}..."
run_on_driver "mkdir -p '${DRIVER_DIR}'"
if [ "$DRIVER_HOST" = localhost ]; then
  rsync -a --delete --exclude .git "${CORE_DIR}/" "${DRIVER_DIR}/Altun-core/"
else
  rsync -a --delete --exclude .git -e "ssh $ssh_options" "${CORE_DIR}/" "$ssh_user@$DRIVER_HOST:${DRIVER_DIR}/Altun-core/"
fi
run_on_driver "cd '${PROJECT_DIR}' && test -f config_cft.json || cp template/cft-config-template.json config_cft.json"

# ── Step 3: start altun-controller on the driver ──
echo "[3/3] Starting altun-controller..."
run_on_driver "
  cd '${PROJECT_DIR}'
  pkill -f altun-controller 2>/dev/null || true
  pkill -f cft-adapter 2>/dev/null || true
  pkill -f bft-adapter 2>/dev/null || true
  sleep 1
  START_NODE='${START_NODE}' nohup ./altun-controller \
    --http-port ${CONTROLLER_PORT} \
    --cft-adapter '${PROJECT_DIR}/cft-adapter' \
    --bft-adapter '${PROJECT_DIR}/bft-adapter' \
    --base-grpc-port ${GRPC_BASE_PORT} \
    --meta-file '${DRIVER_DIR}/vplane_meta.json' \
    --init-protocol ${INIT_PROTOCOL} \
    --init-mode ${INIT_MODE} \
    --timeout 30 \
    > '${DRIVER_DIR}/controller.log' 2>&1 &
  echo \"Controller PID: \$!\"
  sleep 5
  curl -s http://localhost:${CONTROLLER_PORT}/status | head -c 200 && echo ''
" 2>&1

echo ""
echo "=========================================="
echo " Altun control plane running"
echo "=========================================="
echo ""
echo " Controller API:  http://${DRIVER_HOST}:${CONTROLLER_PORT}"
echo ""
echo " Trigger a reconfiguration:"
echo "   curl -X POST http://${DRIVER_HOST}:${CONTROLLER_PORT}/reconfig \\"
echo "     -d '{\"trigger\":\"intra\",\"target\":\"epaxos\"}'"
echo ""
echo " Status:"
echo "   curl http://${DRIVER_HOST}:${CONTROLLER_PORT}/status"
echo ""
echo " Run the selector against the controller (from Altun-RL/):"
echo "   python online_maddpg.py \\"
echo "     --controller_addr ${DRIVER_HOST}:${CONTROLLER_PORT} \\"
echo "     --config_file systems/config_default.json \\"
echo "     --project_dir ${PROJECT_DIR} \\"
echo "     --saved_folder results/run1 \\"
echo "     --max_steps 10 --reward TPS --reward_fn best_and_pre"
echo ""
echo " Logs: ${DRIVER_HOST}:${DRIVER_DIR}/controller.log"
echo "=========================================="
