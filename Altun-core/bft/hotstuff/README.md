# hotstuff (Altun fork)

This directory is a fork of [`relab/hotstuff`](https://github.com/relab/hotstuff), a modular
framework for HotStuff-style consensus protocols. Altun uses it as the engine for the
Chained HotStuff and Fast HotStuff members of its BFT protocol pool (`cmd/hotstuff` is
deployed as `bft-server`). The upstream license (MIT, see `LICENSE`) applies to this
directory. The fork adds the following, each kept behind an environment variable or a
separate command so that the upstream behaviour is unchanged by default:

| Extension | Location | Purpose |
|---|---|---|
| Timed closed-loop client | `client/client.go`, `metrics/clientlatency.go` | `HS_WINDOW_MS` bounds a measurement window; each client reports the commands completed with `f+1` replies inside the window and their exact latencies. |
| Window summary tool | `cmd/hswindow` | Aggregates the per-client window records into one throughput/latency line (deployed as `bft-window`). |
| Fault injection | `faultinject/`, `backend/server.go`, `consensus/consensus.go` | `SLOW_PROPOSAL_MS`, `BFT_SLOW_IDS`, `BFT_ABSENT_IDS` make selected replicas propose late or take no part, mirroring the fault hooks of the CFT protocols. |
| Container orchestration | `internal/orchestration/docker.go` | `HS_DOCKER=1` runs one worker per container (`HS_DOCKER_EXE`, `HS_DOCKER_TMP`, `HS_DOCKER_ASSIGN`, `HS_DOCKER_IPS`, `HS_DOCKER_CPREFIX`, `HS_REPLICA_PORT`, `HS_CLIENT_PORT`) for emulated wide-area deployments. |
| KV front end | `replica/kvfront.go`, `cmd/hskv` | A replica variant that executes committed commands on the Altun replicated key-value state machine (`altunsm`) and serves the HTTP interface used by the Altun proxy for online protocol switching and checkpoint certification. |

Build from the Altun-core root with `bash bft/build.sh`, or in this directory with `make`.
The remainder of this file is the upstream documentation.

---

## Contents

- [hotstuff](#hotstuff)
  - [Contents](#contents)
  - [Build Dependencies](#build-dependencies)
  - [Getting Started](#getting-started)
    - [Linux and macOS](#linux-and-macos)
    - [Windows](#windows)
  - [Running Experiments](#running-experiments)
  - [Safety Testing with Twins](#safety-testing-with-twins)
  - [Modules](#modules)
  - [Consensus Interfaces](#consensus-interfaces)
  - [Crypto Interfaces](#crypto-interfaces)
  - [References](#references)

## Build Dependencies

- [Go](https://go.dev) (at least version 1.18)

If you modify any of the protobuf files, you will need the following to compile them:

- [Protobuf compiler](https://github.com/protocolbuffers/protobuf) (at least version 3.15)

- The gRPC and gorums plugins for protobuf.
  - Linux and macOS users can run `make tools` to install these.
  - The Windows build script downloads them automatically.
  - They can also be installed manually:
    - `go install github.com/relab/gorums/cmd/protoc-gen-gorums`
    - `go install google.golang.org/protobuf/cmd/protoc-gen-go`

## Getting Started

### Linux and macOS

- Compile command line utilities by running `make`.

### Windows

- Run `.\build.ps1` with PowerShell.

**NOTE**: you should use `./hotstuff.exe` instead of `./hotstuff` when running commands.

## Running Experiments

See the [experimentation documentation](docs/experimentation.md) for details.

The `hotstuff` command line utility can be used to run experiments locally, or on remote servers using SSH.
For a list of options, run `./hotstuff help run`.

The `plot` command line utility can be used to create graphs from measurements.
Run `./plot --help` for a list of options.

## Safety Testing with Twins

We have implemented the Twins strategy [6] for testing the safety of the consensus implementations.
See the [twins documentation](docs/twins.md) for details.

## Modules

The following components are modular:

- Consensus
  - The "core" of the consensus protocol, which decides when a replica should vote for a proposal,
    and when a block should be committed.
  - 3 implementations:
    - `chainedhotstuff`: The three-phase pipelined HotStuff protocol presented in the HotStuff paper [1].
    - `fasthotstuff`: A two-chain version of HotStuff designed to prevent forking attacks [3].
    - `simplehotstuff`: A simplified version of chainedhotstuff [4].
- Crypto
  - Implements the cryptographic primitives used by HotStuff, namely quorum certificates.
  - 2 implementations:
    - `ecdsa`: A very simple implementation where quorum certificates are represented by arrays of ECDSA signatures.
    - `bls12`: An implementation of threshold signatures based on BLS12-381 aggregated signatures.
- Synchronizer
  - Implements a view-synchronization algorithm. It is responsible for synchronizing replicas to the same view number.
  - Current implementation based on [DiemBFT's RoundState](https://github.com/diem/diem/tree/main/consensus/src/liveness) [5].
- Blockchain
  - Implements storage for the block chain. Currently we have an in-memory cache of a fixed size.
- Leader rotation
  - Decides which replica should be the leader of a view.
  - Currently either a fixed leader or round-robin.
- Networking/Backend
  - Using [Gorums](https://github.com/relab/gorums) [2]

These modules are designed to be interchangeable and simple to implement.
It is also possible to add custom modules that can interact with the modules we listed above.

## Consensus Interfaces

There are two ways to implement a new consensus protocol, depending on the requirements of the new protocol.
The `Consensus` module interface is the most general, but also the most difficult to implement.
That is because it is oriented towards the other modules, and the other modules rely on it to perform certain operations,
such as verifying proposals, interacting with the `BlockChain`, `Acceptor`, `Executor`, and `Synchronizer` modules,
and so on. That is why we provide a simplified `Rules` interface and the optional `ProposeRuler` interface.
These interfaces are only required to implement the core rules of the consensus protocol, namely deciding whether or not
to vote for a proposal, whether or not it is safe to commit a block, and optionally how to create new blocks.
These interfaces do not require any interaction with other modules, as that is taken care of by a
[default implementation](consensus/consensus.go) of the `Consensus` interface.

The `Consensus` and `Rules` interfaces can also be used to override the behavior of other consensus implementations.
The `consensus/byzantine` package contains some examples of byzantine behaviors that can be implemented by wrapping
implementations of these interfaces.

## Crypto Interfaces

In a similar way to the `Consensus` interface, the `Crypto` interface also introduces a simplified, lower-level interface
that should be used to implement new crypto algorithms.
The `Crypto` interface itself provides methods for working with the high-level structures such as quorum certificates.
However, to make it easy to implement new crypto algorithms, the `CryptoImpl` interface can be used instead.
This interface deals with signatures and threshold signatures directly, which are used by the `Crypto` interface
to implement quorum certificates.

## References

[1] M. Yin, D. Malkhi, M. K. Reiter, G. Golan Gueta, and I. Abraham, “HotStuff: BFT Consensus in the Lens of Blockchain,” Mar 2018.

[2] Tormod Erevik Lea, Leander Jehl, and Hein Meling. Towards New Abstractions for Implementing Quorum-based Systems. In 37th International Conference on Distributed Computing Systems (ICDCS), Jun 2017.

[3] Mohammad M. Jalalzai, Jianyu Niu, Chen Feng, Fangyu Gai. Fast-HotStuff: A Fast and Resilient HotStuff Protocol, Oct 2020.

[4] Leander Jehl. Formal Verification of HotStuff, Jun 2021.

[5] Baudet, Mathieu, et al. "State machine replication in the libra blockchain." The Libra Assn., Tech. Rep (2019).

[6]: S. Bano, A. Sonnino, A. Chursin, D. Perelman, en D. Malkhi, “Twins: White-Glove Approach for BFT Testing”, 2020.
