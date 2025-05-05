#!/bin/bash
saved_folder=log_78_maddpg_env_mutation
project_dir=/users/tianwen/go/src/github.com/wtwinlab/Altun-core/bin
config_cft_file=./systems/config_cft.json

top_k=3
reward_discount=0.8
fn_name=only_consider_pre_1
saved_config=inferece_config.json
checkpoint=
python inference_maddpg.py \
    --config_file $config_cft_file \
    --project_dir $project_dir \
    --record_file $saved_folder/inference_log.csv \
    --reward_fn $fn_name \
    --reward_top_num $top_k \
    --reward_discount $reward_discount \
    --saved_config $saved_config \
    --checkpoint $checkpoint \
    --reward TPS


