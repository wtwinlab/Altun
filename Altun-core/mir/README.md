# MirBFT / PBFT deployment

Deployment scripts for the two pool members served by the MirBFT engine
(`../bft/mirbft`): MirBFT (`NUM_LEADERS` = number of replicas) and PBFT
(`NUM_LEADERS = 1`). `bin/run_config.sh` invokes `deployment/apply-mir.sh` for a
configuration; the scripts can also be used directly.

```
deployment/apply-mir.sh NUM_NODES NUM_CLIENTS CONCURRENCY RATE_LIMIT PAYLOAD_SIZE BATCH_SIZE REQUESTS NUM_LEADERS
```

`apply-mir.sh` runs `config-gen.sh` (certificates, server and client configuration
files, one directory per process copied to its host) and `run.sh` (start servers and
clients, wait for every client to finish its measurement window, collect
`window.out`, stop the processes), then writes `{"TPS": ..., "latency": ...}` to
`bin/benchmark.log`.

| File | Purpose |
|---|---|
| `deployment/apply-mir.sh` | One benchmark step: configuration, run, result line |
| `deployment/config-gen.sh` | Cluster configuration: certificates, YAML files from `config-file-templates`, transfer to hosts |
| `deployment/config-gen-local.sh` | Same for a single-host run (`local=true` in `bin/env.sh`) |
| `deployment/run.sh`, `run-local.sh` | Start servers and clients, collect results |
| `deployment/run-server.sh`, `run-client.sh`, `stop.sh` | Per-host helpers copied next to the binaries |
| `deployment/generate-ca.sh`, `lib.sh` | Certificate authority and per-server certificates |
| `deployment/config-file-templates/` | Server, client and OpenSSL templates (upper-case fields are substituted) |
| `tools/perf-eval.py` | Throughput/latency from server logs and client traces (local runs) |

Binaries: `mir/bin/mir-server` and `mir/bin/mir-client`, built by `bash bft/build.sh`.

Environment (from `bin/env.sh`): `ssh_user`, `ssh_options`, `private_key_file`,
`START_NODE`, `NODES_PER_WORKER`, `CLIENTS_PER_WORKER`, `WORKER_PREFIX`, `REMOTE_DIR`,
`REMOTE_BASE`, `local`, `ALTUNWAN`, `ALTUNWAN_CPREFIX`. Run-time options:
`MIR_WINDOW_MS` (measurement window, default 10000), `MIR_WAIT_EXTRA_S` (grace period
after the window, default 60), `MIR_RECEIVERS` (replicas each request is sent to,
default all), `MIR_DEDICATED`, `MIR_SERVER_LOG`, `MIR_CLIENT_LOG`.
