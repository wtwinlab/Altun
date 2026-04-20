# Altun
**Altun** is a tunable distributed ledger framework that achieves adaptivity along two dimensions: workload and fault model. An operator schedules policy windows (CFT/BFT) through a signed control-plane input; within each window, an online MADDPG agent selects a consensus protocol to optimize long-term throughput and latency. A virtual-log reconfiguration mechanism carries ordered state across protocol boundaries with bounded disruption, and an off-path auditor provides post-hoc forensic evidence during CFT execution.

# Altun-Core and Altun-RL
### Altun-core (Go)

Consensus protocol implementations behind a uniform API (propose/advance/learn, view-change hooks, checkpoint callbacks). Currently integrates eight protocols spanning CFT and BFT families:

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

Also includes the forensic auditor (off-path accountability checker) and YCSB-based benchmarking infrastructure.

### Altun_RL (Python)

MADDPG-based online learning framework for protocol selection.

| File | Description |
|---|---|
| `online_maddpg.py` | Main training script |
| `inference_maddpg.py` | Inference with trained checkpoints |
| `maddpg_env.py` | Environment wrapper — bridges Altun-core and Altun_RL |
| `reward.py` | Reward function (`best_and_pre`) |
| `altun_env_mutation.py` | Workload drift / environment mutation logic |
| `baseline_selectors.py` | Baseline selectors (Thompson Sampling, UCB1, ε-greedy, etc.) |
| `run_selector_comparison.py` | Selector strategy comparison experiments |
| `utils.py` | Auxiliary functions |

---

### MADDPG Agent Design

Two cooperative agents under centralized-training-decentralized-execution (CTDE):

- **CFT agent** — selects among: Paxos, EPaxos, VPaxos, WPaxos
- **BFT agent** — selects among: PBFT, MirBFT, Chained HotStuff, Fast HotStuff

Only one agent is active per step (determined by the operator's policy window); the other is masked.


## Quick Start

### Prerequisites

```bash
# Altun_RL
pip install -r Altun_RL/requirements.txt

# Altun-core
cd Altun-core && make        # requires Go 1.19+
```

### Clone (fresh machine)

```bash
bash setup.sh
```

### Training

```bash
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

### Inference

```bash
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
