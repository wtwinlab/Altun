# Shared settings of the switching scripts.  Source this file or export the
# variables before running cluster.sh / run_switching.sh / scenarios.sh.
#
#   ALTUN_BIN   directory holding cft-kv, hskv, altun-proxy, altun-load,
#               altun-ctl and altun-check (see ../README.md, "Build")
#   ALTUN_KEYS  directory produced by genkeys.sh:
#                 ctl.key / ctl.pub        directive signing key of the controller
#                 att/NAME.key, NAME.pub   ed25519 attestation keys (paxos1..N, hotstuff1..N)
#                 hs/r1.key .. rN.pub      HotStuff replica keys
#   HOST        address every process binds and connects to (single-host deployment)
#   CORES       optional CPU list for taskset (e.g. 0-7); unset = no pinning
#   N, F        membership size and fault threshold of each instance
SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export ALTUN_BIN="${ALTUN_BIN:-$SCRIPTS_DIR/../../bin}"
export ALTUN_KEYS="${ALTUN_KEYS:-$SCRIPTS_DIR/../keys}"
export HOST="${HOST:-127.0.0.1}"
export N="${N:-4}" F="${F:-1}"
# PIN: command prefix of every process (taskset when CORES is set, empty otherwise)
export PIN="${CORES:+taskset -c $CORES}"
