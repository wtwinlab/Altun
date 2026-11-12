#!/bin/bash
# 检查是否提供了第一个参数
if [ -z "$1" ]; then
  echo "Error: No Worker argument provided."
  echo "Usage: $0 <argument>"
  exit 1
fi

echo "Copy Altun-core into worker disk"
bash check_and_copy.sh $1

#/data-ssd/tianwen/workers/worker0${1}_Altun/
project_dir=/data-ssd/tianwen/workers/worker0${1}_Altun/Altun-core/bin
config_cft_file=./systems/config_bft_W_experiment.json
top_k=3
reward_discount=0.99
mutate_env_fn=comparing_dist
fn_name=only_best_and_pre_1_wo_ratio
checkpoint=/data-ssd/tianwen/workers/worker080_Altun/log_only_best_and_pre_1_wo_ratio_env_mutation_loop_new_w_tps_new_experiments/models/MADDPG_trained_agent_400.pt
saved_folder=/data-ssd/tianwen/workers/worker0${1}_Altun/online_log_${fn_name}_env_mutation_${mutate_env_fn}_tps_w_400

rm -rf $saved_folder
[ -d $saved_folder ] | mkdir $saved_folder
# Run the configuration script
# Check if the file exists, and if it does, remove it
[ -f $project_dir/config_RL.json ] && rm $project_dir/config_RL.json
[ -f $project_dir/benchmark.log ] && rm $project_dir/benchmark.log
cp $project_dir/config.json $project_dir/config_RL.json

python online_maddpg.py \
    --config_file $config_cft_file \
    --project_dir $project_dir \
    --batch_size 8 \
    --max_steps 1001 \
    --record_file $saved_folder/log.csv \
    --saved_folder $saved_folder \
    --reward_fn $fn_name \
    --reward_top_num $top_k \
    --reward_discount $reward_discount \
    --mutate_env_fn ${mutate_env_fn} \
    --saved_steps 20 \
    --checkpoint $checkpoint \
    --reward TPS


