#!/usr/bin/env bash
# Build the Altun-core binaries into this directory, next to the deployment
# scripts that copy them to the cluster hosts.
#
#   cft-server        CFT replica (paxos | epaxos | vpaxos | wpaxos)
#   cft-client        closed-loop benchmark client
#   cft-cmd           administrative command-line client
#   cft-kv            Paxos replica whose executor drives the replicated key-value state machine
#   forensic-verify   post-hoc verifier of the forensics logs
#   cft-adapter       gRPC ConsensusInstance adapter around a CFT replica
#   bft-adapter       gRPC ConsensusInstance adapter around the HotStuff orchestrator
#   altun-controller  virtual log, policy windows and reconfiguration
#   altunwan-probe    TCP RTT probe used by the Docker/WAN emulation
#
# The BFT engines (bft-server, bft-client, bft-window, hskv, mir/bin/*) are built
# by ../bft/build.sh.
set -e
dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "${dir}/.."
go build -o bin/cft-server       ./cmd/cft-server/
go build -o bin/cft-client       ./cmd/cft-client/
go build -o bin/cft-cmd          ./cmd/cft-cmd/
go build -o bin/cft-kv           ./cmd/cft-kv/
go build -o bin/forensic-verify  ./cmd/forensic-verify/
go build -o bin/cft-adapter      ./cmd/cft-adapter/
go build -o bin/bft-adapter      ./cmd/bft-adapter/
go build -o bin/altun-controller ./cmd/altun-controller/
CGO_ENABLED=0 go build -o bin/altunwan-probe ./cmd/altunwan-probe/
