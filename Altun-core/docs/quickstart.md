# Quickstart: control plane and protocol selector

## Processes

```
altun-controller            gRPC ConsensusInstance
  ├── virtual log      ───►  cft-adapter  (paxos / epaxos / vpaxos / wpaxos, in process)
  ├── reconfiguration        bft-adapter  (chainedhotstuff / fasthotstuff, bft-server subprocess)
  └── HTTP API  ◄───  selector (Altun-RL/online_maddpg.py) or operator
```

| Process | Role | Source |
|---|---|---|
| `altun-controller` | virtual log, policy windows, four-phase reconfiguration | `cmd/altun-controller/` |
| `cft-adapter` | gRPC wrapper around an in-process CFT replica | `cmd/cft-adapter/` |
| `bft-adapter` | gRPC wrapper that manages a `bft-server` subprocess | `cmd/bft-adapter/` |

## Prerequisites

- Go 1.25 or later
- `jq` on the driver host
- password-less SSH from the driver to the cluster hosts named in `bin/env.sh`
- the Python environment of `Altun-RL` (see its README) for the selector
- `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc` only when `proto/consensus.proto` changes

## 1. Configure the cluster

Edit `bin/env.sh`:

| Variable | Meaning |
|---|---|
| `WORKER_PREFIX`, `START_NODE` | cluster hosts are `${WORKER_PREFIX}<index>`; `START_NODE` is the first index; replica hosts first, then client hosts. `START_NODE=127.0.0.1` runs everything locally |
| `NODES_PER_WORKER`, `CLIENTS_PER_WORKER` | replicas and clients per host |
| `ssh_user`, `private_key_file` | SSH credentials for the cluster hosts |
| `REMOTE_DIR` | working directory on every cluster host |
| `FORENSICS_DIR` | directory for forensics logs; must be visible from every host |
| `ALTUNWAN`, `WAN_DELAY_MS`, ... | Docker/WAN emulation (see `bin/altunwan.sh`) |

## 2. Build

```bash
make all          # Go binaries into bin/, BFT engines via bft/build.sh
```

## 3. Start the control plane

```bash
cd bin
bash start_altun.sh                          # controller on this host, cluster from env.sh
DRIVER_HOST=driver1 INIT_PROTOCOL=epaxos bash start_altun.sh
INIT_PROTOCOL=chainedhotstuff INIT_MODE=BFT bash start_altun.sh
```

| Variable | Default | Meaning |
|---|---|---|
| `DRIVER_HOST` | `localhost` | host that runs the controller and the adapters |
| `DRIVER_DIR` | `$REMOTE_BASE/altun-driver` | copy of Altun-core on the driver host |
| `INIT_PROTOCOL` | `paxos` | first consensus protocol |
| `INIT_MODE` | `CFT` | first fault model (`CFT` or `BFT`) |
| `CONTROLLER_PORT` | `8080` | HTTP API port |
| `GRPC_BASE_PORT` | `9100` | first gRPC port of the adapters |

The equivalent manual start:

```bash
./altun-controller --http-port 8080 \
  --cft-adapter $PWD/cft-adapter --bft-adapter $PWD/bft-adapter \
  --base-grpc-port 9100 --meta-file vplane_meta.json \
  --init-protocol paxos --init-mode CFT --timeout 30 \
  --verifier-bin $PWD/forensic-verify --forensics-log $FORENSICS_DIR/current.jsonl
curl http://localhost:8080/status
curl http://localhost:8080/instances
```

## 4. Reconfigure through the API

Intra-family switch (same fault model, no policy window required):

```bash
curl -X POST http://localhost:8080/reconfig \
  -H "Content-Type: application/json" \
  -d '{"trigger":"intra","target":"epaxos"}'
```

Cross-family switch (requires a policy window signed by the operator key; the
controller generates an ephemeral key pair when none is configured):

```bash
curl -X POST http://localhost:8080/reconfig \
  -H "Content-Type: application/json" \
  -d '{"trigger":"cross","target":"chainedhotstuff","mode":"BFT",
       "replicas":4,"clients":8,"batch_size":1,"payload_size":128,
       "rate_limit":10000,"duration":30,"output_dir":"/tmp/bft-output"}'
```

The response reports the new instance and the duration of each phase:

| Phase | Work |
|---|---|
| admission | verify the policy signature, the quorum bound `n >= max(3 f_BF + 1, 2 f_CF + 1)` and the protocol pool |
| instantiation | launch the new adapter, register its metadata, restore the checkpoint |
| activation | wait for the new instance to answer, redirect traffic (abort on timeout) |
| seal | boundary checkpoint on the old instance; for CFT, `forensic-verify` runs first |

## 5. Run the selector

```bash
cd ../../Altun-RL
python online_maddpg.py \
  --controller_addr localhost:8080 \
  --config_file systems/config_default.json \
  --project_dir <path to Altun-core/bin> \
  --saved_folder results/run1 \
  --max_steps 1000 --reward TPS --reward_fn best_and_pre \
  --mutate_env_fn block_schedule_mutation
```

With `--controller_addr` the environment requests switches through
`POST /reconfig`. Without it the environment calls `run_system.sh`, which stops
the running instance and starts the selected protocol through
`bin/run_config.sh`.

## 6. Stop

```bash
bash stop_altun.sh                 # local driver
bash stop_altun.sh driver1
```

## Troubleshooting

- Controller does not start: see `controller.log` in `DRIVER_DIR`; a port in use
  is changed with `CONTROLLER_PORT`.
- Adapter fails on a switch: adapter output is in `controller.log`; check SSH
  access to the first cluster host and that `jq` is on the PATH.
- gRPC connection refused: an adapter needs a few seconds to listen; the
  controller retries, and `--timeout` raises the activation timeout.

## Controller HTTP API

| Endpoint | Method | Description |
|---|---|---|
| `/reconfig` | POST | trigger a protocol switch |
| `/status` | GET | active instance, commit status, policy window |
| `/instances` | GET | metadata of all instances |
