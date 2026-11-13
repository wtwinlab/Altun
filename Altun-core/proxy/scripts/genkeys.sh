#!/bin/bash
# Generate every key a switching deployment needs, under $ALTUN_KEYS:
#   ctl.key / ctl.pub         controller key that signs policy-window directives
#   att/paxosI.{key,pub}      ed25519 attestation keys of the Paxos replicas
#   att/hotstuffI.{key,pub}   ed25519 attestation keys of the HotStuff replicas
#   hs/rI.{key,pub}           HotStuff (ECDSA) replica keys
#
#   genkeys.sh [N]            default N from env.sh
set -eu
source "$(dirname "$0")/env.sh"
N=${1:-$N}
mkdir -p "$ALTUN_KEYS/att" "$ALTUN_KEYS/hs"
"$ALTUN_BIN/altun-ctl" -genkeys "$ALTUN_KEYS/ctl"
for i in $(seq 1 "$N"); do
  "$ALTUN_BIN/altun-ctl" -genkeys "$ALTUN_KEYS/att/paxos$i"
  "$ALTUN_BIN/altun-ctl" -genkeys "$ALTUN_KEYS/att/hotstuff$i"
done
"$ALTUN_BIN/hskv" -genkeys -n "$N" -keys "$ALTUN_KEYS/hs"
echo "keys written to $ALTUN_KEYS (n=$N)"
