#!/bin/bash
# Run a trained MADDPG checkpoint in inference mode against a live deployment.
#
# Required environment:
#   PROJECT_DIR   Altun-core bin directory of the deployment
#   CHECKPOINT    path to a checkpoint saved by online_maddpg.py
# Optional:
#   CONFIG_FILE, SAVED_FOLDER, REWARD_FN, REWARD_DISCOUNT, TOP_K
set -e

PROJECT_DIR=${PROJECT_DIR:?set PROJECT_DIR to the Altun-core bin directory}
CHECKPOINT=${CHECKPOINT:?set CHECKPOINT to a saved model}
CONFIG_FILE=${CONFIG_FILE:-./systems/config_default.json}
SAVED_FOLDER=${SAVED_FOLDER:-./inference_$(date +"%Y%m%d%H%M")}
REWARD_FN=${REWARD_FN:-best_and_pre}
REWARD_DISCOUNT=${REWARD_DISCOUNT:-0.8}
TOP_K=${TOP_K:-3}

mkdir -p "$SAVED_FOLDER"

python inference_maddpg.py \
    --config_file "$CONFIG_FILE" \
    --project_dir "$PROJECT_DIR" \
    --record_file "$SAVED_FOLDER/inference_log.csv" \
    --reward_fn "$REWARD_FN" \
    --reward_top_num "$TOP_K" \
    --reward_discount "$REWARD_DISCOUNT" \
    --saved_config "$SAVED_FOLDER/inference_config.json" \
    --checkpoint "$CHECKPOINT" \
    --reward TPS
