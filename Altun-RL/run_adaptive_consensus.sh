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
config_file=./systems/config_default.json
top_k=3
reward_discount=0.8
TOTAL_STEPS=1000
mutate_env_fn=adaptive_consensus_mutation
fn_name=adaptive_consensus_reward
timestamp=$(date +"%Y%m%d%H%M")
# Create experiment directory
saved_folder=/data-ssd/tianwen/workers/worker${1}_Altun/log_${fn_name}_env_${mutate_env_fn}_${timestamp}
LOG_FILE=${saved_folder}/experiment.log
rm -rf $saved_folder
mkdir -p $saved_folder

# Delete history directories if they exist
[ -d $saved_folder/history_action ] && rm -rf $saved_folder/history_action
[ -d $saved_folder/performance_history ] && rm -rf $saved_folder/performance_history
[ -d $saved_folder/models ] && rm -rf $saved_folder/models

# Clean up and prepare configuration
[ -f $project_dir/config_RL.json ] && rm $project_dir/config_RL.json
[ -f $project_dir/benchmark.log ] && rm $project_dir/benchmark.log
cp $project_dir/config_RL_default.json $project_dir/config_RL.json
# cp $project_dir/config_bft_default.json $project_dir/config_RL.json

# Run the MADDPG experiment
python online_maddpg.py \
    --config_file $config_file \
    --project_dir $project_dir \
    --batch_size 8 \
    --max_steps ${TOTAL_STEPS} \
    --record_file $saved_folder/log.csv \
    --saved_folder $saved_folder \
    --reward_fn $fn_name \
    --reward_top_num $top_k \
    --reward_discount $reward_discount \
    --mutate_env_fn ${mutate_env_fn} \
    --saved_steps 200 \
    --reward TPS \
    --total_steps ${TOTAL_STEPS} 2>&1 | tee -a ${LOG_FILE}

# Check if the experiment completed successfully
if [ $? -eq 0 ]; then
    echo "Experiment completed successfully at $(date)" | tee -a ${LOG_FILE}
else
    echo "Experiment failed at $(date)" | tee -a ${LOG_FILE}
fi