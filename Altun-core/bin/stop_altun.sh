#!/bin/bash
# stop_altun.sh — Stop all Altun stack processes on a worker.
# Usage: bash stop_altun.sh [WORKER]

WORKER="${1:-098}"

echo "Stopping Altun stack on worker-${WORKER}..."
ssh worker-${WORKER} "
  pkill -f altun-controller 2>/dev/null && echo '  controller stopped' || echo '  no controller'
  pkill -f cft-adapter 2>/dev/null && echo '  cft-adapter stopped' || echo '  no cft-adapter'
  pkill -f bft-adapter 2>/dev/null && echo '  bft-adapter stopped' || echo '  no bft-adapter'
  pkill -f cft-server 2>/dev/null && echo '  cft-server stopped' || echo '  no cft-server'
  pkill -f bft-server 2>/dev/null && echo '  bft-server stopped' || echo '  no bft-server'
"
echo "Done."
