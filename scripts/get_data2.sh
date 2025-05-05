
function getdata() {
    wnum=$1
    reward=$2
    expfn=$3
    tfolder=/users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/experiments/$4/log_${reward}_env_mutation_${expfn}_${item}_w${wnum}
    item=$5
    rsync -av  worker-0${wnum}:/data-ssd/tianwen/workers/worker0${wnum}_Altun/log_${reward}_env_mutation_${expfn}_${item}_wo_negr/ ${tfolder}
    #ssh worker-0${wnum} "rsync -av --exclude='*.pt'  /data-ssd/tianwen/workers/worker0${wnum}_Altun/log_${reward}_env_mutation_${expfn}_${item}_wo_negr/ ${tfolder}"
    #mv /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/experiments/${tfolder}/log_${reward}_env_mutation_${expfn}_latency /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/experiments/cft/log_${reward}_env_mutation_${expfn}_latency_w${wnum}
}

local_dir="/users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/results"

# writeratio concurrency ratelimit node_size clients random
rsync -av worker-109:/data-ssd/tianwen/workers/worker109_Altun/log_best_and_pre_env_concurrency_mutation_202504261042 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/concurrency/
rsync -av worker-101:/data-ssd/tianwen/workers/worker101_Altun/log_best_and_pre_env_payload_mutation_202504191450 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/payload/
rsync -av worker-100:/data-ssd/tianwen/workers/worker100_Altun/log_best_and_pre_env_batchsize_mutation_202504271723 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/batch/
rsync -av worker-095:/data-ssd/tianwen/workers/worker95_Altun/log_best_and_pre_env_ratelimit_mutation_202504292218 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/ratelimit/
rsync -av worker-103:/data-ssd/tianwen/workers/worker103_Altun/log_best_and_pre_env_ratelimit_mutation_202504292209 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/ratelimit/


rsync -av worker-110:/data-ssd/tianwen/workers/worker110_Altun/log_best_and_pre_1_env_nodesize_mutation_202504111002 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/nodesize/
rsync -av worker-102:/data-ssd/tianwen/workers/worker102_Altun/log_best_and_pre_1_env_clients_mutation_202504091324 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/clients/

# selecction
rsync -av worker-092:/data-ssd/tianwen/workers/worker92_Altun/log_adaptive_consensus_reward_env_adaptive_consensus_mutation_202504201327 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/selection/
# 50
rsync -av worker-100:/data-ssd/tianwen/workers/worker100_Altun/log_adaptive_consensus_reward_env_adaptive_consensus_mutation_202504202041 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/selection/
# 100
rsync -av worker-099:/data-ssd/tianwen/workers/worker99_Altun/log_adaptive_consensus_reward_env_adaptive_consensus_mutation_202504202057 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/selection/
# 150
rsync -av worker-100:/data-ssd/tianwen/workers/worker100_Altun/log_adaptive_consensus_reward_env_adaptive_consensus_mutation_202504211349  /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/selection/
# 400
rsync -av worker-099:/data-ssd/tianwen/workers/worker99_Altun/log_adaptive_consensus_reward_env_adaptive_consensus_mutation_202504251739 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/selection/


# baseline
rsync -av worker-116:/data-ssd/tianwen/workers/worker116_Altun/bftbrain_random_mutation_202504151435 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/bftbrain/
rsync -av worker-098:/data-ssd/tianwen/workers/worker98_Altun/bftbrain_adaptive_selecton_mutation_202504250638 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/bftbrain/
rsync -av worker-098:/data-ssd/tianwen/workers/worker98_Altun/bftbrain_adaptive_selecton_mutation_202504262040 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/bftbrain/
rsync -av worker-098:/data-ssd/tianwen/workers/worker98_Altun/bftbrain_adaptive_selecton_mutation_202504271906 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/bftbrain/
rsync -av worker-097:/data-ssd/tianwen/workers/worker97_Altun/bftbrain_ratelimit_mutation_202504292214 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/bftbrain/
rsync -av --exclude='performance_history/' --exclude='models/'  worker-094:/data-ssd/tianwen/workers/worker94_Altun/bftbrain_ratelimit_mutation_202504301802 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/bftbrain/
rsync -av --exclude='performance_history/' --exclude='models/'  worker-096:/data-ssd/tianwen/workers/worker96_Altun/bftbrain_ratelimit_mutation_202504301801 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/bftbrain/

rsync -av --exclude='performance_history/' --exclude='models/' worker-093:/data-ssd/tianwen/workers/worker93_Altun/bftbrain_ratelimit_mutation_202504302031  /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/bftbrain/
rsync -av --exclude='performance_history/' --exclude='models/' worker-092:/data-ssd/tianwen/workers/worker92_Altun/bftbrain_ratelimit_mutation_202504302037 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/bftbrain/
rsync -av --exclude='performance_history/' --exclude='models/' worker-091:/data-ssd/tianwen/workers/worker91_Altun/bftbrain_ratelimit_mutation_202505010716 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/bftbrain/


# random
rsync -av worker-099:/data-ssd/tianwen/workers/worker99_Altun/log_best_and_pre_1_env_random_small_mutation_202504061524 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/random/
rsync -av worker-099:/data-ssd/tianwen/workers/worker99_Altun/log_best_and_pre_env_random_mutation_202504170250 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/random/



# reward
rsync -av worker-098:/data-ssd/tianwen/workers/worker98_Altun/log_best_and_pre_env_ratelimit_mutation_202504140838 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/reward/
rsync -av worker-097:/data-ssd/tianwen/workers/worker97_Altun/log_dynamic_reward_function_env_ratelimit_mutation_202504140846 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/reward/
rsync -av worker-096:/data-ssd/tianwen/workers/worker96_Altun/log_adaptive_reward_scaling_env_ratelimit_mutation_202504140848 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/reward/
rsync -av worker-095:/data-ssd/tianwen/workers/worker95_Altun/log_env_aware_reward_env_ratelimit_mutation_202504140857 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/reward/
rsync -av worker-094:/data-ssd/tianwen/workers/worker94_Altun/log_progressive_reward_shaping_env_ratelimit_mutation_202504140907 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/reward/
rsync -av worker-093:/data-ssd/tianwen/workers/worker93_Altun/log_multi_objective_reward_env_ratelimit_mutation_202504140910 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/reward/


rsync -av --exclude='performance_history/' --exclude='models/' worker-095:/data-ssd/tianwen/workers/worker95_Altun/log_best_and_pre_env_ratelimit_mutation_202505010618 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/reward/
rsync -av --exclude='performance_history/' --exclude='models/' worker-103:/data-ssd/tianwen/workers/worker103_Altun/log_best_and_pre_env_ratelimit_mutation_202505010616 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/reward/

rsync -av --exclude='performance_history/' --exclude='models/' worker-109:/data-ssd/tianwen/workers/worker109_Altun/log_best_and_pre_env_ratelimit_mutation_202505010612 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/reward/

rsync -av --exclude='performance_history/' --exclude='models/' worker-101:/data-ssd/tianwen/workers/worker101_Altun/log_best_and_pre_env_payload_mutation_202505011548 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/resultsv2/inference_payloads

# rsync -av worker-102:/data-ssd/tianwen/workers/worker102_Altun/log_best_and_pre_1_env_clients_mutation_202504051312 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/results/clients/
# rsync -av worker-111:/data-ssd/tianwen/workers/worker111_Altun/log_best_and_pre_1_env_writeratio_mutation_202503311303 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/results/writeratio/
# rsync -av worker-109:/data-ssd/tianwen/workers/worker109_Altun/log_best_and_pre_1_env_concurrency_mutation_202503311422 /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/results/concurrency/

# rsync -av worker-111:/data-ssd/tianwen/workers/worker111_Altun/log_only_best_and_pre_1_wo_ratio_env_writeratio_mutation_experiment /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/results/writeratio/
# rsync -av worker-109:/data-ssd/tianwen/workers/worker109_Altun/log_only_best_and_pre_1_wo_ratio_env_concurrency_mutation_tps_replica_experiment /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/results/concurrency/
# rsync -av worker-110:/data-ssd/tianwen/workers/worker110_Altun/log_only_best_and_pre_1_wo_ratio_env_ratelimit_mutation_experiment /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/results/ratelimit/

# rsync -av worker-110:/data-ssd/tianwen/workers/worker111_Altun/log_only_best_and_pre_1_wo_ratio_env_writeratio_mutation_experiment /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/results/writeratio/
# rsync -av worker-110:/data-ssd/tianwen/workers/worker110_Altun/log_only_best_and_pre_1_wo_ratio_env_replica_random_mutation_tps_replica_experiment /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/results/node_size/
# rsync -av worker-110:/data-ssd/tianwen/workers/worker110_Altun/log_random_failed_bft /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/results/node_size/
# rsync -av worker-110:/data-ssd/tianwen/workers/worker110_Altun/log_random_failed_concurrency /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/results/node_size/

# rsync -av worker-116:/data-ssd/tianwen/workers/worker0116_Altun/log_only_best_and_pre_1_wo_ratio_env_replica_mutation_tps_replica_experiment /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/results/node_size
#  rsync -av /mnt/data1/tianwen/experiments/ worker-080:/data-ssd/tianwen/experiments/
# mkdir -p /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/experiments/w_exp_two_agents
# mkdir -p /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/experiments/cft_two_agents
# getdata 86 only_best_and_pre_1_wo_ratio cft_bft_exp_step350 cft_two_agents latency

# getdata 76 only_best_and_pre_1_wo_ratio cft_bft_exp_random cft_two_agents latency

# getdata 87 only_best_and_pre_1_wo_ratio cft_bft_exp_step20 cft_two_agents latency

# getdata 70 only_best_and_pre_1_wo_ratio cft_bft_exp_step350 cft_two_agents tps
# getdata 71 only_best_and_pre_1_wo_ratio cft_bft_exp_random cft_two_agents tps
# getdata 75 only_best_and_pre_1_wo_ratio cft_bft_exp_step20 cft_two_agents tps



# getdata 77 only_best_and_pre_1_wo_ratio loop_w w_exp_two_agents latency

# getdata 78 only_best_and_pre_1_wo_ratio loop_w w_exp_two_agents latency

# getdata 78 only_best_and_pre_1_wo_ratio loop_w w_exp_two_agents latency


# getdata 83 only_best_and_pre_1_wo_ratio loop_w_2 w_exp_two_agents latency

# getdata 84 only_best_and_pre_1_wo_ratio loop_w_2 w_exp_two_agents latency

# getdata 85 only_best_and_pre_1_wo_ratio loop_w_2 w_exp_two_agents latency


# getdata 72 only_best_and_pre_1_wo_ratio loop_w w_exp_two_agents tps

# getdata 73 only_best_and_pre_1_wo_ratio loop_w w_exp_two_agents tps

# getdata 74 only_best_and_pre_1_wo_ratio loop_w w_exp_two_agents tps

# getdata 80 only_best_and_pre_1_wo_ratio loop_w_2 w_exp_two_agents tps

# getdata 81 only_best_and_pre_1_wo_ratio loop_w_2 w_exp_two_agents tps
# getdata 82 only_best_and_pre_1_wo_ratio loop_w_2 w_exp_two_agents tps

# mkdir -p /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/experiments/random_change
# getdata 88 only_best_and_pre_1_wo_ratio mutate_env random_change latency
# getdata 89 only_best_and_pre_1_wo_ratio mutate_env random_change tps


# rsync -av  worker-065:/data-ssd/tianwen/workers/worker065_Altun/inference_log_only_best_and_pre_1_wo_ratio_env_mutation_loop_w_3_tps_w_random/ experiments/inference_w/inference_log_only_best_and_pre_1_wo_ratio_env_mutation_loop_w_3_tps_w_random_65/

# rsync -av  worker-064:/data-ssd/tianwen/workers/worker064_Altun/inference_log_only_best_and_pre_1_wo_ratio_env_mutation_loop_w_3_latency_w_random/ experiments/inference_w/inference_log_only_best_and_pre_1_wo_ratio_env_mutation_loop_w_3_latency_w_random_64/