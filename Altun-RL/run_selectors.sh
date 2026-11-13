#!/bin/bash
# Compare the baseline selectors (Thompson sampling, UCB1, Greedy, BestFixed)
# and optionally a MADDPG checkpoint on the same workload schedule.
#
# Required environment:
#   PROJECT_DIR   Altun-core bin directory of the deployment
# Optional:
#   CONFIG_FILE, SAVED_FOLDER, MAX_STEPS, MUTATE_ENV_FN, SELECTORS
#   CHECKPOINT    when set, the MADDPG agent is included (--include_ddpg)
set -e

PROJECT_DIR=${PROJECT_DIR:?set PROJECT_DIR to the Altun-core bin directory}
CONFIG_FILE=${CONFIG_FILE:-./systems/config_default.json}
SAVED_FOLDER=${SAVED_FOLDER:-./selectors_$(date +"%Y%m%d%H%M")}
MAX_STEPS=${MAX_STEPS:-96}
MUTATE_ENV_FN=${MUTATE_ENV_FN:-block_schedule_mutation}

extra=()
[ -n "$SELECTORS" ]  && extra+=(--selectors "$SELECTORS")
[ -n "$CHECKPOINT" ] && extra+=(--include_ddpg --checkpoint "$CHECKPOINT")

python run_selector_comparison.py \
    --config_file "$CONFIG_FILE" \
    --project_dir "$PROJECT_DIR" \
    --saved_folder "$SAVED_FOLDER" \
    --max_steps "$MAX_STEPS" \
    --mutate_env_fn "$MUTATE_ENV_FN" \
    "${extra[@]}"
