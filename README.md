# Altun
Altun: On the Design and Implementation of A Tunable Distributed Ledger Framework

# Altun-Core-RL
The repo contains the reinforcement learning code of Altun-Core, Altun-Core-RL.

## Pre-requirement

`pip install -r requriements.txt`

## Introduction

* `system_config/config.json` contains the configurations of  Altun-Core-RL.
* `online_maddpg.py` is the main script of Altun-Core-RL.

    - `$config_cft_file` is the path to `system_config/config.json`
    - `$project_dir` is the path to Altun-Core
    - `$saved_folder` is the saved folder for Altun-Core-RL
    - `$fn_name` is the reward function name, default `altun_reward`.
    - `$top_k` and `$reward_discount` are two parametners for the reward function.
    ```shell
    python online_maddpg.py \
            --config_file $config_cft_file \
            --project_dir $project_dir \
            --batch_size 8 \
            --max_steps 1500 \
            --record_file $saved_folder/log.csv \
            --saved_folder $saved_folder \
            --reward_fn $fn_name \
            --reward_top_num $top_k \
            --reward_discount $reward_discount \
            --mutate_env_fn ${mutate_env_fn} \
            --saved_steps 20 \
            --reward TPS
    ```
* `reward.py` implements the reward function.
* `utils.py` implements some auxiliary functions
* `maddpg_env.py` wraps the interaction betwenn `Altun-Core` and `Altun-Core-RL`.
