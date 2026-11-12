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
config_cft_file=./systems/config_bft_switch_cft_new.json
top_k=3
reward_discount=0.99
mutate_env_fn=new_mutation
for fn_name in only_best_and_pre_1_wo_ratio
do
saved_folder=/data-ssd/tianwen/workers/worker0${1}_Altun/log_${fn_name}_env_${mutate_env_fn}_latency_bft_random_new
rm -rf $saved_folder
[ -d $saved_folder ] | mkdir $saved_folder
# Delete history_action directory if it exists
[ -d $saved_folder/history_action ] && rm -rf $saved_folder/history_action
# Delete performance_history directory if it exists
[ -d $saved_folder/performance_history ] && rm -rf $saved_folder/performance_history
# Delete models if it exists
[ -d $saved_folder/models ] && rm -rf $saved_folder/models

# Run the configuration script
# Check if the file exists, and if it does, remove it
[ -f $project_dir/config_RL.json ] && rm $project_dir/config_RL.json
[ -f $project_dir/benchmark.log ] && rm $project_dir/benchmark.log
cp $project_dir/config.json $project_dir/config_RL.json

python online_maddpg.py \
    --config_file $config_cft_file \
    --project_dir $project_dir \
    --batch_size 8 \
    --max_steps 5000 \
    --record_file $saved_folder/log.csv \
    --saved_folder $saved_folder \
    --reward_fn $fn_name \
    --reward_top_num $top_k \
    --reward_discount $reward_discount \
    --mutate_env_fn ${mutate_env_fn} \
    --saved_steps 250 \
    --reward latency
done

