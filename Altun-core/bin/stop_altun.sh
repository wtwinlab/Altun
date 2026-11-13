#!/bin/bash
# stop_altun.sh — stop the control plane (controller, adapters) and any consensus
# processes on the driver host.
# Usage: bash stop_altun.sh [DRIVER_HOST]      (default: localhost)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${SCRIPT_DIR}/env.sh"
DRIVER_HOST="${1:-${DRIVER_HOST:-localhost}}"

STOP='
  pkill -f altun-controller 2>/dev/null && echo "  controller stopped" || echo "  no controller"
  pkill -f cft-adapter 2>/dev/null && echo "  cft-adapter stopped" || echo "  no cft-adapter"
  pkill -f bft-adapter 2>/dev/null && echo "  bft-adapter stopped" || echo "  no bft-adapter"
  pkill -f cft-server 2>/dev/null && echo "  cft-server stopped" || echo "  no cft-server"
  pkill -f bft-server 2>/dev/null && echo "  bft-server stopped" || echo "  no bft-server"
'
echo "Stopping the Altun control plane on ${DRIVER_HOST}..."
if [ "$DRIVER_HOST" = localhost ]; then bash -c "$STOP"; else ssh $ssh_options "$ssh_user@$DRIVER_HOST" "$STOP"; fi
echo "Done."
