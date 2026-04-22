# Altun
**Altun** is a tunable distributed ledger framework that achieves adaptivity along two dimensions: workload and fault model. An operator schedules policy windows (CFT/BFT) through a signed control-plane input; within each window, an online MADDPG agent selects a consensus protocol to optimize long-term throughput and latency. A virtual-log reconfiguration mechanism carries ordered state across protocol boundaries with bounded disruption, and an off-path auditor provides post-hoc forensic evidence during CFT execution.

## Repository Structure

The repository (**Altun_RL**) contains the RL-based protocol selector. The [Altun-core] repository contains consensus protocol implementations, the forensic auditor, and the virtual-log reconfiguration plane.

# Altun-Core and Altun-RL
### Altun-core (Go)

Consensus protocol implementations behind a uniform gRPC `ConsensusInstance` API (propose, seal, checkpoint, restore). Currently integrates eight protocols spanning CFT and BFT families:

| Protocol | Family | Key Property |
|---|---|---|
| Paxos | CFT | Single-leader, two-phase |
| EPaxos | CFT | Leaderless, parallel commits |
| VPaxos | CFT | Dynamic leader reconfiguration |
| WPaxos | CFT | Geo-distributed, locality-aware quorums |
| PBFT | BFT | Classic three-phase, single leader |
| MirBFT | BFT | Parallel leaders, pipelined |
| Chained HotStuff | BFT | Pipelined views, linear communication |
| Fast HotStuff | BFT | Fast-path optimization, reduced RTT |


### Altun_RL (Python)

MADDPG-based online learning framework for protocol selection.

| File | Description |
|---|---|
| `online_maddpg.py` | Training / inference entry point |
| `maddpg_env.py` | PettingZoo-compatible environment; hysteresis gate; switch logging |
| `reward.py` | Reward functions (`best_and_pre` is the default) |
| `altun_env_mutation.py` | Workload mutation strategies (block schedules, drift benchmarks) |
| `baseline_selectors.py` | Thompson Sampling, UCB1, Greedy, BestFixed selectors |
| `run_selector_comparison.py` | Selector strategy comparison driver (Exp 1) |
| `utils.py` | Config management, benchmark bridge, controller API client |
| `systems/config_default.json` | Observation and action space definition |

---

### MADDPG Agent Design

Two cooperative agents under centralized-training-decentralized-execution (CTDE):

- **CFT agent** — selects among: Paxos, EPaxos, VPaxos, WPaxos
- **BFT agent** — selects among: PBFT, MirBFT, Chained HotStuff, Fast HotStuff

Only one agent is active per step (determined by the operator's policy window); the other is masked.


## Quick Start

### Prerequisites

```bash
# Python (Altun_RL)
conda create -n rl python=3.12
conda activate rl
pip install agilerl pettingzoo gymnasium torch numpy pandas

# Go (Altun-core)
cd Altun-core
go build ./...                           # requires Go 1.21+
go build -o bin/cft-adapter      ./cmd/cft-adapter/
go build -o bin/bft-adapter      ./cmd/bft-adapter/
go build -o bin/altun-controller ./cmd/altun-controller/
```

### Clone (fresh machine)

```bash
bash setup.sh
```

### Start the Altun stack

```bash
# 1: Start the Altun stack
cd Altun-core/bin
bash start_altun.sh
# Protocol switching through the altun-controller

### Run RL agent 

```bash
# 2: Training
python Altun_RL/online_maddpg.py \
    --config_file system_config/config.json \
    --project_dir <path-to-Altun-core> \
    --batch_size 8 \
    --max_steps 1500 \
    --record_file <saved_folder>/log.csv \
    --saved_folder <saved_folder> \
    --reward_fn altun_reward \
    --reward_top_num <top_k> \
    --reward_discount <discount> \
    --mutate_env_fn <mutation_fn> \
    --saved_steps 20 \
    --reward TPS
```

```bash
#  2: Inference
python Altun_RL/inference_maddpg.py \
    --config_file system_config/config.json \
    --project_dir <path-to-Altun-core> \
    --checkpoint <path-to-saved-model>
```

---

* `system_config/config.json` contains the configurations of  Altun-RL.
* `online_maddpg.py` is the main script of Altun-RL.

    - `$config_cft_file` is the path to `system_config/config.json`
    - `$project_dir` is the path to Altun-Core
    - `$saved_folder` is the saved folder for Altun-Core-RL
    - `$fn_name` is the reward function name, default `altun_reward`.
    - `$top_k` and `$reward_discount` are two parameters for the reward function.
    ```shell
    
* `reward.py` implements the reward function.
* `utils.py` implements some auxiliary functions
* `maddpg_env.py` wraps the interaction between `Altun-Core` and `Altun-RL`.
