#!/bin/bash
# start_altun.sh — Launch the full Altun stack:
#   1. Build all binaries (cft-adapter, bft-adapter, altun-controller)
#   2. Start altun-controller (manages virtual log + reconfiguration)
#   3. Controller auto-launches the initial cft-adapter
#
# Usage:
#   bash start_altun.sh [--protocol paxos] [--mode CFT] [--workers 098]
#
# Environment (set before running, or use defaults):
#   WORKER         — worker number for cluster allocation (default: 098)
#   START_NODE     — first cluster worker (default: 50)
#   INIT_PROTOCOL  — initial consensus protocol (default: paxos)
#   INIT_MODE      — initial fault mode: CFT or BFT (default: CFT)
#   CONTROLLER_PORT — HTTP port for controller API (default: 8080)
#   GRPC_BASE_PORT  — base port for adapter gRPC (default: 9100)

set -euo pipefail

# ── Paths ──
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CORE_DIR="$(dirname "$SCRIPT_DIR")"
RL_DIR="/users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL"

# ── Defaults ──
WORKER="${WORKER:-098}"
START_NODE="${START_NODE:-50}"
INIT_PROTOCOL="${INIT_PROTOCOL:-paxos}"
INIT_MODE="${INIT_MODE:-CFT}"
CONTROLLER_PORT="${CONTROLLER_PORT:-8080}"
GRPC_BASE_PORT="${GRPC_BASE_PORT:-9100}"

# Worker disk paths
WORKER_DIR="/data-ssd/tianwen/workers/worker${WORKER}_Altun"
PROJECT_DIR="${WORKER_DIR}/Altun-core/bin"

echo "=========================================="
echo " Altun Stack Launcher"
echo "=========================================="
echo " Worker:         ${WORKER} (cluster ${START_NODE}-$((START_NODE+5)))"
echo " Protocol:       ${INIT_PROTOCOL} (${INIT_MODE})"
echo " Controller:     http://localhost:${CONTROLLER_PORT}"
echo " gRPC base port: ${GRPC_BASE_PORT}"
echo "=========================================="

# ── Step 0: Copy Altun-core to worker disk ──
echo "[0/4] Copying Altun-core to worker-${WORKER}..."
ssh worker-${WORKER} "
  mkdir -p ${WORKER_DIR}
  rm -rf ${WORKER_DIR}/Altun-core
  cp -rf /users/tianwen/go/src/github.com/wtwinlab/Altun-core ${WORKER_DIR}/
  sed -i 's/^START_NODE=.*/START_NODE=${START_NODE}/' ${WORKER_DIR}/Altun-core/bin/env.sh
  echo 'Worker copy OK, START_NODE=${START_NODE}'
"

# ── Step 1: Build binaries ──
echo "[1/4] Building binaries..."
cd "${CORE_DIR}"
go build -o bin/cft-adapter   ./cmd/cft-adapter/
go build -o bin/bft-adapter   ./cmd/bft-adapter/
go build -o bin/altun-controller ./cmd/altun-controller/
echo "  cft-adapter, bft-adapter, altun-controller built OK"

# Copy adapter binaries to worker
scp -q bin/cft-adapter bin/bft-adapter worker-${WORKER}:${WORKER_DIR}/Altun-core/bin/

# ── Step 2: Generate cluster config ──
echo "[2/4] Generating cluster config..."
# The cft-adapter needs a config.json with node addresses.
# Reuse the existing config_cft.json format from Altun-core.
ssh worker-${WORKER} "
  cd ${PROJECT_DIR}
  # Ensure config files exist
  test -f config_cft.json || cp config_RL_default.json config_cft.json
  echo 'Config ready'
"

# ── Step 3: Start altun-controller on the worker ──
echo "[3/4] Starting altun-controller on worker-${WORKER}..."
ssh worker-${WORKER} "
  export PATH=/users/tianwen/.local/bin:\$PATH
  cd ${WORKER_DIR}/Altun-core/bin

  # Kill any existing controller/adapter processes
  pkill -f altun-controller 2>/dev/null || true
  pkill -f cft-adapter 2>/dev/null || true
  pkill -f bft-adapter 2>/dev/null || true
  sleep 1

  nohup ./altun-controller \
    --http-port ${CONTROLLER_PORT} \
    --cft-adapter ${PROJECT_DIR}/cft-adapter \
    --bft-adapter ${PROJECT_DIR}/bft-adapter \
    --base-grpc-port ${GRPC_BASE_PORT} \
    --meta-file ${WORKER_DIR}/vplane_meta.json \
    --init-protocol ${INIT_PROTOCOL} \
    --init-mode ${INIT_MODE} \
    --timeout 30 \
    > ${WORKER_DIR}/controller.log 2>&1 &

  echo \"Controller PID: \$!\"
  sleep 5

  # Verify controller is up
  curl -s http://localhost:${CONTROLLER_PORT}/status | head -c 200 && echo ''
" 2>&1

echo ""
echo "=========================================="
echo " Altun Stack Running"
echo "=========================================="
echo ""
echo " Controller API:  http://worker-${WORKER}:${CONTROLLER_PORT}"
echo " Cluster workers: ${START_NODE} - $((START_NODE+5))"
echo ""
echo " Test reconfiguration:"
echo "   curl -X POST http://worker-${WORKER}:${CONTROLLER_PORT}/reconfig \\"
echo "     -d '{\"trigger\":\"intra\",\"target\":\"epaxos\"}'"
echo ""
echo " Check status:"
echo "   curl http://worker-${WORKER}:${CONTROLLER_PORT}/status"
echo ""
echo " Run RL agent with controller:"
echo "   python online_maddpg.py \\"
echo "     --controller_addr worker-${WORKER}:${CONTROLLER_PORT} \\"
echo "     --config_file systems/config_default.json \\"
echo "     --project_dir ${PROJECT_DIR} \\"
echo "     --saved_folder results/test \\"
echo "     --max_steps 10 --reward TPS --reward_fn best_and_pre"
echo ""
echo " Logs: ssh worker-${WORKER} tail -f ${WORKER_DIR}/controller.log"
echo "=========================================="
