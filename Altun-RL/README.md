# Altun-RL

Altun-RL is the learning-based protocol selector of Altun. Within the policy
window scheduled by the operator (CFT or BFT), it observes the throughput and
latency reported by the running deployment, selects the consensus protocol for
the next interval, and asks Altun-core to switch to it. Selection is learned
online with multi-agent deep deterministic policy gradient (MADDPG); Thompson
sampling, UCB1, greedy and fixed-protocol selectors are provided for
comparison.

## Layout

| File | Role |
|---|---|
| `online_maddpg.py` | Online training and inference loop of the MADDPG selector |
| `inference_maddpg.py` | Inference from a saved checkpoint |
| `maddpg_env.py` | PettingZoo-compatible environment: observation and action spaces, family gate, hysteresis gate, switch and drift logging |
| `reward.py` | Reward functions (`best_and_pre` is the default) |
| `altun_env_mutation.py` | Workload schedules applied to the deployment during a run |
| `utils.py` | Observation parsing, configuration updates, deployment restart and controller API client |
| `baseline_selectors.py` | Thompson sampling, UCB1, greedy, best-fixed and fixed-protocol selectors |
| `run_selector_comparison.py` | Runs the baseline selectors (and optionally a MADDPG checkpoint) on one workload schedule |
| `parse_exp_log.py`, `tools/` | Post-processing of run logs |
| `systems/` | Observation and action space definitions |
| `run_system.sh`, `kill.sh` | Restart and stop the deployment from `Altun-core/bin` |
| `run_train.sh`, `run_inference.sh`, `run_selectors.sh` | Launchers |

## Agent design

Two cooperative agents are used. The CFT agent selects among Paxos, EPaxos, VPaxos and
WPaxos; the BFT agent selects among PBFT, MirBFT, chained HotStuff and fast
HotStuff. The observation is the normalised vector of the last measured
throughput and latency together with the workload descriptors (replica count,
clients, concurrency, rate limit, batch size, payload size, write ratio and
fault model). The reward compares the measured performance with the previous
step and with the mean of the top-k historical values; see `reward.py`.

An optional hysteresis gate (`--cooldown_steps`, `--switch_margin`) suppresses
switches that follow a previous switch too closely or promise too small a
gain. Every switch is logged to `switch_detail.csv` together with the timing
reported by Altun-core, and every workload change to `drift_events.jsonl`.

## Interaction with Altun-core

The selector writes the chosen protocol and workload parameters to
`config_RL.json` in the Altun-core `bin` directory and then either

* restarts the deployment through `run_system.sh`, which calls
  `Altun-core/bin/run_config.sh` (default), or
* sends a reconfiguration request to the `altun-controller` HTTP API when
  `--controller_addr` (or `ALTUN_CONTROLLER_ADDR`) is set, in which case the
  virtual-log reconfiguration plane carries state across the switch.

After each step the benchmark result is read from `benchmark.log` in the same
directory.

## Installation

```bash
conda create -n altun-rl python=3.12
conda activate altun-rl
pip install -r requirements.txt
```

## Running

```bash
# Online training against a deployment whose scripts live in $PROJECT_DIR
PROJECT_DIR=/path/to/Altun-core/bin bash run_train.sh

# Inference from a checkpoint
PROJECT_DIR=/path/to/Altun-core/bin CHECKPOINT=/path/to/model.pt bash run_inference.sh

# Baseline selector comparison on the block-schedule workload
PROJECT_DIR=/path/to/Altun-core/bin bash run_selectors.sh
```

`python online_maddpg.py --help` lists all options, including the network
hyperparameters, the reward function (`--reward_fn`), the reward parameters
(`--reward_discount`, `--reward_top_num`) and the workload schedule
(`--mutate_env_fn`, a function name in `altun_env_mutation.py`).

## Configuration

`systems/config_default.json` defines the observation space (performance
ranges and workload descriptors) and the action space (`bft_alg` and
`cft_alg` pools), the location of the files exchanged with Altun-core
(`action_file`, `current_performance_file`, `default_config_file`) and the
step timeout. The other files in `systems/` restrict the workload descriptor
ranges for single-dimension sweeps (clients, concurrency, replica count,
payload size, rate limit, batch size, write ratio) and for random workloads;
`config_cft_only.json` restricts the action pool to the CFT protocols.
