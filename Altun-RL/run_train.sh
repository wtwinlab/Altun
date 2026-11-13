#!/bin/bash
# Online MADDPG training against a live Altun-core deployment.
#
# Required environment:
#   PROJECT_DIR   Altun-core bin directory of the deployment (run_config.sh,
#                 config_RL_default.json and benchmark.log live there)
# Optional:
#   CONFIG_FILE   observation/action space definition (default systems/config_default.json)
#   SAVED_FOLDER  output directory for logs, action history and checkpoints
#   TOTAL_STEPS   number of control steps (default 1000)
#   MUTATE_ENV_FN workload mutation function in altun_env_mutation.py
#   REWARD_FN     reward function in reward.py
#   REWARD_DISCOUNT, TOP_K  reward parameters
set -e

PROJECT_DIR=${PROJECT_DIR:?set PROJECT_DIR to the Altun-core bin directory}
CONFIG_FILE=${CONFIG_FILE:-./systems/config_default.json}
TOTAL_STEPS=${TOTAL_STEPS:-1000}
MUTATE_ENV_FN=${MUTATE_ENV_FN:-adaptive_consensus_mutation}
REWARD_FN=${REWARD_FN:-adaptive_consensus_reward}
REWARD_DISCOUNT=${REWARD_DISCOUNT:-0.8}
TOP_K=${TOP_K:-3}
timestamp=$(date +"%Y%m%d%H%M")
SAVED_FOLDER=${SAVED_FOLDER:-./log_${REWARD_FN}_env_${MUTATE_ENV_FN}_${timestamp}}

mkdir -p "$SAVED_FOLDER"
LOG_FILE=${SAVED_FOLDER}/experiment.log

# Start from the default protocol configuration of the deployment
rm -f "$PROJECT_DIR/config_RL.json" "$PROJECT_DIR/benchmark.log"
cp "$PROJECT_DIR/config_RL_default.json" "$PROJECT_DIR/config_RL.json"

python online_maddpg.py \
    --config_file "$CONFIG_FILE" \
    --project_dir "$PROJECT_DIR" \
    --batch_size 8 \
    --max_steps "$TOTAL_STEPS" \
    --total_steps "$TOTAL_STEPS" \
    --record_file "$SAVED_FOLDER/log.csv" \
    --saved_folder "$SAVED_FOLDER" \
    --reward_fn "$REWARD_FN" \
    --reward_top_num "$TOP_K" \
    --reward_discount "$REWARD_DISCOUNT" \
    --mutate_env_fn "$MUTATE_ENV_FN" \
    --saved_steps 200 \
    --reward TPS 2>&1 | tee -a "$LOG_FILE"
