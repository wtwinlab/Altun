#!/usr/bin/env bash
# Builds the BFT engine binaries of the Altun protocol pool.
#
#   bash bft/build.sh            # from the Altun-core root
#   OUT=/some/dir bash bft/build.sh
#
# Output (relative to OUT, default: the Altun-core root):
#   bin/bft-server      HotStuff replica/client orchestrator  (bft/hotstuff/cmd/hotstuff)
#   bin/bft-client      HotStuff measurement reader           (bft/hotstuff/cmd/plot)
#   bin/bft-window      measurement-window summary            (bft/hotstuff/cmd/hswindow)
#   bin/hskv            HotStuff replica with the KV front end (bft/hotstuff/cmd/hskv)
#   mir/bin/mir-server  MirBFT / PBFT server                  (bft/mirbft/server)
#   mir/bin/mir-client  MirBFT / PBFT client                  (bft/mirbft/client)
#
# bin/apply-bft.sh expects bin/bft-server, bin/bft-client and bin/bft-window next to
# itself; mir/deployment/config-gen.sh copies mir/bin/mir-server and mir/bin/mir-client
# to the replica and client hosts.
set -euo pipefail

BFT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="${OUT:-$(dirname "$BFT_DIR")}"
mkdir -p "$OUT/bin" "$OUT/mir/bin"

echo "building HotStuff binaries"
(
  cd "$BFT_DIR/hotstuff"
  go build -o "$OUT/bin/bft-server" ./cmd/hotstuff
  go build -o "$OUT/bin/bft-client" ./cmd/plot
  go build -o "$OUT/bin/bft-window" ./cmd/hswindow
  go build -o "$OUT/bin/hskv"       ./cmd/hskv
)

echo "building MirBFT binaries"
(
  cd "$BFT_DIR/mirbft"
  go build -o "$OUT/mir/bin/mir-server" ./server
  go build -o "$OUT/mir/bin/mir-client" ./client
)

echo "BFT binaries written to $OUT/bin and $OUT/mir/bin"
