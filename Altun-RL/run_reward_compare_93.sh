#!/bin/bash
# Script to run experiments with different reward functions

# Check if worker ID is provided
if [ -z "$1" ]; then
  echo "Error: No Worker argument provided."
  echo "Usage: $0 <worker_id>"
  exit 1
fi

echo "Copy Altun-core into worker disk"
bash check_and_copy.sh $1

project_dir=/data-ssd/tianwen/workers/worker${1}_Altun/Altun-core/bin
config_cft_file=./systems/config_ratelimit.json
top_k=3
reward_discount=0.8
mutate_env_fn=ratelimit_mutation
max_steps=1500
batch_size=8
saved_steps=200

# Array of reward functions to test
reward_functions=(
  # "best_and_pre"  # Original baseline 98
  # "dynamic_reward_function" # 97
  # "adaptive_reward_scaling" # 96
  # "env_aware_reward" # 95
  # "progressive_reward_shaping" # 94
  "multi_objective_reward" # 93
)

for fn_name in "${reward_functions[@]}"
do
  echo "Running experiment with reward function: $fn_name"
  timestamp=$(date +"%Y%m%d%H%M")
  # Create experiment directory
  saved_folder=/data-ssd/tianwen/workers/worker${1}_Altun/log_${fn_name}_env_${mutate_env_fn}_${timestamp}
  # saved_folder=/data-ssd/tianwen/workers/worker0${1}_Altun/log_${fn_name}_env_${mutate_env_fn}_tps
  
  # Clean up and prepare folders
  rm -rf $saved_folder
  mkdir -p $saved_folder
  [ -d $saved_folder/history_action ] && rm -rf $saved_folder/history_action
  [ -d $saved_folder/performance_history ] && rm -rf $saved_folder/performance_history
  [ -d $saved_folder/models ] && rm -rf $saved_folder/models
  
  # Reset environment
  [ -f $project_dir/config_RL.json ] && rm $project_dir/config_RL.json
  [ -f $project_dir/benchmark.log ] && rm $project_dir/benchmark.log
  cp $project_dir/config_RL_default.json $project_dir/config_RL.json
  
  # Set extra params based on reward function
  extra_params=""
  if [ "$fn_name" == "adaptive_reward_scaling" ]; then
    extra_params="--window_size 20"
  fi
  
  if [ "$fn_name" == "progressive_reward_shaping" ] || [ "$fn_name" == "multi_objective_reward" ]; then
    extra_params="--total_steps $max_steps"
  fi
  
  # Run experiment
  python online_maddpg.py \
    --config_file $config_cft_file \
    --project_dir $project_dir \
    --batch_size $batch_size \
    --max_steps $max_steps \
    --record_file $saved_folder/log.csv \
    --saved_folder $saved_folder \
    --reward_fn $fn_name \
    --reward_top_num $top_k \
    --reward_discount $reward_discount \
    --mutate_env_fn ${mutate_env_fn} \
    --saved_steps $saved_steps \
    --reward TPS \
    $extra_params
  
  echo "Completed experiment with reward function: $fn_name"
  echo "========================================================"
done
