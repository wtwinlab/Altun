#!/bin/bash
project_dir=/users/tianwen/go/src/github.com/wtwinlab/Altun-core/bin
config_cft_file=./systems/config_bft.json

top_k=3
reward_discount=0.99
# fn_name=only_consider_pre_1
# for fn_name in only_best_and_pre_log_wo_ratio
# do
fn_name=only_best_and_pre_log_wo_ratio

saved_folder=./log_${fn_name}_test_inference
rm -rf $saved_folder
[ -d $saved_folder ] | mkdir $saved_folder
# Delete history_action directory if it exists
[ -d $saved_folder/history_action ] && rm -rf $saved_folder/history_action

# Delete performance_history directory if it exists
[ -d $saved_folder/performance_history ] && rm -rf $saved_folder/performance_history


[ -d $saved_folder/models ] && rm -rf $saved_folder/models



# Run the configuration script
# Check if the file exists, and if it does, remove it
[ -f $project_dir/config_RL.json ] && rm $project_dir/config_RL.json
[ -f $project_dir/benchmark.log ] && rm $project_dir/benchmark.log
cp $project_dir/config.json $project_dir/config_RL.json
[ ! -d $saved_folder ] && mkdir $saved_folder


# Run the configuration script
# Check if the file exists, and if it does, remove it
[ -f $project_dir/config_RL.json ] && rm $project_dir/config_RL.json
[ -f $project_dir/benchmark.log ] && rm $project_dir/benchmark.log
cp $project_dir/config.json $project_dir/config_RL.json
[ ! -d $saved_folder ] && mkdir $saved_folder

python online_maddpg.py \
    --config_file $config_cft_file \
    --project_dir $project_dir \
    --batch_size 8 \
    --max_steps 12 \
    --inference \
    --checkpoint /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/log_only_best_and_pre_log_wo_ratio_maddpg_env_mutation_tps_073/models/MADDPG_trained_agent_20.pt \
    --record_file $saved_folder/log.csv \
    --mutate_env mutate_W \
    --saved_folder $saved_folder \
    --reward_fn $fn_name \
    --reward_top_num $top_k \
    --reward_discount $reward_discount \
    --saved_steps 20 \
    --reward TPS


