# Altun-core

Altun-core is the Go part of Altun: the consensus protocols, the control plane
that moves a replicated service between protocols at run time, and the
deployment scripts that drive a cluster. The Python protocol selector lives in
`../Altun-RL`.

## Components

| Directory | Content |
|---|---|
| `consensus/cft/` | CFT replication framework and protocols: Paxos, EPaxos, VPaxos, WPaxos (`paxos/`, `epaxos/`, `vpaxos/`, `wpaxos/`); fault injection (`fault.go`); the forensics auditor (`auditor/`); the gRPC adapter (`adapter/`). Derived from the Paxi framework (`LICENSE.paxi`). |
| `consensus/bft/adapter/` | gRPC adapter that runs a HotStuff-family protocol as a managed subprocess. |
| `bft/hotstuff/` | HotStuff fork: Chained HotStuff, Fast HotStuff, Simple HotStuff, the `bft-server` orchestrator, fault injection and the `hskv` state-machine replica. |
| `bft/mirbft/` | MirBFT fork: PBFT and MirBFT (ISS) replicas and clients; deployment scripts in `mir/deployment/`. |
| `vplane/` | Virtual log, operator policy windows, the four-phase reconfiguration procedure, the boundary checkpoint and the forensic-verifier hook. |
| `altunsm/` | Replicated key-value state machine shared by `cft-kv` and `hskv`: segments, checkpoints, checkpoint attestations and certificates. |
| `proxy/` | Client-facing proxy that performs online switches between a CFT and a BFT instance, with the load generator and the linearizability checker. |
| `cmd/` | Binaries: `altun-controller`, `cft-adapter`, `bft-adapter`, `cft-server`, `cft-client`, `cft-cmd`, `cft-kv`, `forensic-verify`, `altunwan-probe`. |
| `proto/` | `ConsensusInstance` gRPC interface (`consensus.proto`, generated code in `altunpb/`). |
| `benchmark/`, `storage/`, `config/`, `lib/` | Closed-loop benchmark and linearizability checker, Merkle-tree ledger and key-value storage, configuration, utilities. |
| `bin/` | Deployment and benchmark scripts (see below) and configuration templates. |
| `scripts/` | `gen_proto.sh` (protobuf generation), `inject_delay.sh` (local network delay). |
| `docs/` | `quickstart.md`: running the control plane and the selector. |

## Protocols

| Protocol | Family | Implementation | Launched by |
|---|---|---|---|
| Paxos, EPaxos, VPaxos, WPaxos | CFT | `consensus/cft/` | `bin/apply-cft.sh` |
| Chained HotStuff, Fast HotStuff, Simple HotStuff | BFT | `bft/hotstuff/` | `bin/apply-bft.sh` |
| PBFT, MirBFT | BFT | `bft/mirbft/` | `mir/deployment/apply-mir.sh` |

## Building

Go 1.25 or later.

```bash
make core     # bin/cft-server, cft-client, cft-cmd, cft-kv, forensic-verify,
              # cft-adapter, bft-adapter, altun-controller, altunwan-probe
make bft      # bin/bft-server, bft-client, bft-window, hskv, mir/bin/mir-server, mir/bin/mir-client
make all      # both
make test
```

`altunsm`, `proxy`, `bft/hotstuff` and `bft/mirbft` are separate Go modules;
`go.mod` files point at each other with `replace` directives, so the tree builds
without network access to this repository.

## Running a benchmark step

`bin/env.sh` holds the cluster description: host naming (`WORKER_PREFIX`,
`START_NODE`), replicas and clients per host, SSH user and key, the working
directory on the cluster hosts (`REMOTE_DIR`) and the Docker/WAN settings. Every
script in `bin/` and `mir/deployment/` sources it.

A step is described by a JSON file (`bin/config_RL_default.json` is the
template):

```json
{"algorithm": "paxos", "nodesize": 4, "clients": 1, "concurrency": 4,
 "ratelimit": 100000, "requests": 100000, "writeratio": 0.9,
 "batchsize": 1, "payloadsize": 64, "distribution": "uniform"}
```

```bash
cd bin
bash run_config.sh config_RL.json      # dispatches to apply-cft.sh / apply-bft.sh / apply-mir.sh
cat benchmark.log                      # {"TPS": ..., "latency": ..., "p50": ..., "p95": ..., "p99": ...}
```

`run_config.sh` records a step as failed when its latency is below the network
floor or its data rate exceeds the client NICs, so that a selector never learns
from a run that did not execute the protocol.

### Fault injection

Set before `run_config.sh`; the scripts forward the variables to every replica.

| Variable | Effect |
|---|---|
| `SLOW_PROPOSAL_MS`, `SLOW_PROPOSAL_IDS` | throttle the proposal rate of the listed replicas |
| `ABSENT_IDS` | the listed replicas drop every message (CFT); `BFT_ABSENT_IDS` for HotStuff |
| `INJECT_EQUIVOCATION=1` | the Paxos leader sends a conflicting proposal to one acceptor |
| `FORENSICS_MODE=log\|full` | collect the forensics log; `full` also runs `forensic-verify` |

### Docker/WAN emulation

`bin/altunwan.sh` runs every replica and client in its own container with a
netem qdisc that delays cross-region traffic (`WAN_DELAY_MS`, `WAN_JITTER_MS`,
`WAN_CLIENT_REGION`, `WAN_KEYS`). Build the image once with
`bin/docker/mkrootfs.sh`, create the containers with `altunwan.sh up`, and set
`ALTUNWAN=1` in `env.sh`; `run_config.sh` then applies the delay of each step
before it runs.

## Control plane

`altun-controller` keeps the virtual log and the active policy window, launches
`cft-adapter` / `bft-adapter` instances behind the `ConsensusInstance` gRPC
interface, and executes reconfigurations requested through its HTTP API
(`POST /reconfig`, `GET /status`, `GET /instances`). A cross-family request
carries an operator-signed policy window; admission verifies the signature, the
quorum bound and the protocol pool before a new instance is instantiated,
activated and the old one sealed behind a boundary checkpoint. For a CFT
instance the boundary checkpoint runs `forensic-verify` over the forensics log
first. `docs/quickstart.md` walks through a run; `bin/start_altun.sh` starts the
control plane.

## Online switching and checkpoint certification

`proxy/` and `altunsm/` implement the switch of a live replicated key-value
service between a Paxos instance (`cft-kv`) and a Chained HotStuff instance
(`hskv`). The proxy drains the outgoing instance, seals its last segment, and
installs the snapshot on the incoming one only with a checkpoint certificate:
`q_c = n - f_B` matching, signed attestations of the sealed prefix. `altun-load`
and `altun-check` generate the workload and check the combined history for
linearizability. `proxy/README.md` describes the protocol and the command
lines; `../formal/` holds the TLA+ models of the boundary and of certification.

## Licenses

`consensus/cft` is derived from Paxi (MIT, `consensus/cft/LICENSE.paxi`),
`bft/hotstuff` from relab/hotstuff (MIT) and `bft/mirbft` from the MirBFT
research prototype (Apache 2.0); each fork keeps its license file.
