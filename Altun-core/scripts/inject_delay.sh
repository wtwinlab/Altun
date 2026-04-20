#!/bin/bash
# inject_delay.sh — Add / remove network delay on loopback (for liveness fault tests)
# Usage:
#   ./inject_delay.sh add   <delay_ms> [jitter_ms]   # inject delay
#   ./inject_delay.sh remove                          # clear delay
#
# Requires: iproute2 (Linux) or pfctl/dnctl (macOS).
# On macOS the script uses dnctl + pfctl; on Linux it uses tc.
#
# NOTE: adding delay requires root (or CAP_NET_ADMIN).
# In the experiment scripts we call this via sudo if needed.

set -e

ACTION=${1:-"add"}
DELAY_MS=${2:-100}
JITTER_MS=${3:-10}
IFACE=${IFACE:-lo}   # override with e.g. IFACE=eth0

OS=$(uname -s)

add_delay_linux() {
    # Clear any existing qdisc first (ignore error if none exists)
    tc qdisc del dev "$IFACE" root 2>/dev/null || true
    tc qdisc add dev "$IFACE" root netem delay "${DELAY_MS}ms" "${JITTER_MS}ms"
    echo "ok: added ${DELAY_MS}ms ± ${JITTER_MS}ms delay on ${IFACE}"
}

remove_delay_linux() {
    tc qdisc del dev "$IFACE" root 2>/dev/null || true
    echo "ok: removed delay on ${IFACE}"
}

add_delay_macos() {
    # macOS: use dnctl pipe + pfctl dummynet
    # Pipe 1: add delay
    sudo dnctl pipe 1 config delay "${DELAY_MS}ms" bw 0
    # Apply to loopback traffic via pfctl anchor
    echo "dummynet-anchor \"altun_delay\"
anchor \"altun_delay\"" | sudo pfctl -f - 2>/dev/null || true
    echo "dummynet in quick on lo0 pipe 1 all
dummynet out quick on lo0 pipe 1 all" | sudo pfctl -a altun_delay -f -
    sudo pfctl -e 2>/dev/null || true
    echo "ok: added ${DELAY_MS}ms delay on lo0 (macOS dummynet)"
}

remove_delay_macos() {
    sudo pfctl -a altun_delay -F all 2>/dev/null || true
    sudo dnctl pipe 1 delete 2>/dev/null || true
    echo "ok: removed delay on lo0 (macOS dummynet)"
}

case "$ACTION" in
    add)
        if [ "$OS" = "Linux" ]; then
            add_delay_linux
        else
            add_delay_macos
        fi
        ;;
    remove)
        if [ "$OS" = "Linux" ]; then
            remove_delay_linux
        else
            remove_delay_macos
        fi
        ;;
    *)
        echo "Usage: $0 {add|remove} [delay_ms] [jitter_ms]"
        exit 1
        ;;
esac
