#!/bin/bash
# Check if worker argument is provided
if [ -z "$1" ]; then
  echo "Error: No Worker argument provided."
  echo "Usage: $0 <worker_number>"
  exit 1
fi

echo "Copy Altun-core into worker disk"
# bash check_and_copy.sh $1

project_dir=/data-ssd/tianwen/workers/worker${1}_Altun/Altun-core/bin
config_file=./systems/config_payload.json
top_k=3
reward_discount=0.99
mutate_env_fn=payload_mutation
fn_name=best_and_pre
timestamp=$(date +"%Y%m%d%H%M")
# Create experiment directory
saved_folder=/data-ssd/tianwen/workers/worker${1}_Altun/log_${fn_name}_env_${mutate_env_fn}_${timestamp}
checkpoint=/users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/payload/payload_202504080825/models/MADDPG_trained_agent_750.pt

# Clean up and prepare configuration
[ -f $project_dir/config_RL.json ] && rm $project_dir/config_RL.json
[ -f $project_dir/benchmark.log ] && rm $project_dir/benchmark.log
cp $project_dir/config_RL_default.json $project_dir/config_RL.json

# Run the MADDPG experiment
python online_maddpg.py \
    --config_file $config_file \
    --project_dir $project_dir \
    --batch_size 8 \
    --max_steps 1000 \
    --record_file $saved_folder/log.csv \
    --saved_folder $saved_folder \
    --reward_fn $fn_name \
    --reward_top_num $top_k \
    --reward_discount $reward_discount \
    --mutate_env_fn ${mutate_env_fn} \
    --saved_steps 250 \
    --inference \
    --checkpoint $checkpoint \
    --reward TPS










