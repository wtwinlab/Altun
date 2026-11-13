# MirBFT (Altun BFT pool: PBFT and MirBFT)

This directory contains the MirBFT research prototype
([Mir-BFT: High-Throughput Robust BFT for Decentralized Networks](https://arxiv.org/abs/1906.05552),
upstream repository [hyperledger-labs/mirbft](https://github.com/hyperledger-labs/mirbft), research
branch; Go module path `github.com/IBM/mirbft`). It is distributed under the Apache License 2.0
(see `LICENSE`). Altun uses it as the engine for two members of its BFT protocol pool:

| Pool member | Server configuration |
|---|---|
| MirBFT | `maxLeaders` = number of replicas, bucket rotation enabled |
| PBFT | `maxLeaders: 1`, a single leader for the whole run |

The protocol implementation (`mir/`) is unchanged. The client (`client/`) is extended with a
timed measurement window: `clientRunTime` closes the window, submission stops, and the client
writes `window.out` with the number of requests that received `f+1` deliveries inside the window,
their summed latency, the number of requests sent, and per-second completion counts. The
`receivers` option controls how many replicas each request is sent to. The deployment scripts in
`../../mir/deployment` generate certificates and configuration files from
`mir/deployment/config-file-templates`, start the processes, and aggregate the `window.out`
records into one throughput/latency result.

## Layout

| Directory | Content |
|---|---|
| `mir/` | Mir-BFT protocol: batching, buckets, epochs, checkpoints, view change |
| `backend/`, `connection/`, `crypto/`, `persist/`, `tracing/` | Networking, TLS, signatures, persistence, request tracing |
| `server/`, `client/` | Server and client executables |
| `config/` | YAML configuration loader |
| `protos/` | Protocol buffer definitions and generated code (`run-protoc.sh` regenerates them) |
| `sampleconfig/` | Sample server and client configurations for a local run |
| `tools/perf-eval.py` | Throughput/latency evaluation from server logs and client traces |

## Build

```bash
go build -o bin/mir-server ./server
go build -o bin/mir-client ./client
```

`bash ../build.sh` (from `Altun-core/bft`) builds both together with the HotStuff binaries and
places them in `Altun-core/mir/bin`, where the deployment scripts expect them.

## Local run with the sample configuration

The sample configurations reference certificates under `sampleconfig/certs/ecdsa`
(`ca.pem`, `server<i>.key`, `server<i>.pem`). Generate them with the certificate helpers of the
deployment scripts (`generate-ca.sh` and `genCert` in `lib.sh`), or run
`../../mir/deployment/config-gen-local.sh` with `local=true` to produce a complete configuration
under `mir/deployment/config`.

```bash
cd server
for i in 1 2 3 4; do ./server ../sampleconfig/serverconfig/config$i.yml server$i > server-$i.out 2>&1 & done
cd ../client
./client ../sampleconfig/clientconfig/4peer-config.yml client
```

The first argument is the configuration file, the second a prefix for the trace file. Client
processes stop according to `requestsPerClient` and `clientRunTime`; server processes are stopped
by the deployment scripts (`stop.sh`).

## Configuration

Comments in `sampleconfig/serverconfig/*.yml` and `sampleconfig/clientconfig/*.yml` describe every
field. Constraints that must hold:

* `signatureVerification: true` enables client authentication; `sigSharding: true` enables
  signature verification sharding; `payloadSharding: true` enables light total order broadcast.
* `watermarkDist` must be at least `checkpointDist` and at least the number of nodes;
  `bucketRotationPeriod` must be at least `watermarkDist`.
* `clientWatermarkDist` should be large so that few client processes can saturate the system.
* `serverConnectionBuffer` may need to be increased for large deployments.
* In the `servers` section, `certFiles` and `addresses` are ordered by server id; servers listen on
  `0.0.0.0:<server-port>`, clients connect to `<server-port>+2`.
* PBFT: `maxLeaders: 1`; with `bucketRotationPeriod` larger than the number of batches of the run,
  the leader never changes.
* Fault emulation: `byzantineDelay`, `byzantineAfter`, `byzantineUntil`, `censoring`,
  `byzantineDuplication` in the `Byzantine behavior` section.

## Performance evaluation

`tools/perf-eval.py n m server-1.out ... server-n.out client-001.trc ... client-m.trc x y`
computes the average end-to-end latency, the request rate per client, and the throughput from
`n` server logs and `m` client traces, ignoring the first `x` and last `y` requests; it also
writes a latency CDF to `latency.out`. The deployment script `run.sh` instead aggregates the
clients' `window.out` records, which report exactly the requests completed inside the
measurement window.
