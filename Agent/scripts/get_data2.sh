
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
#  rsync -av /mnt/data1/tianwen/experiments/ worker-080:/data-ssd/tianwen/experiments/
mkdir -p /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/experiments/w_exp_two_agents
mkdir -p /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/experiments/cft_two_agents
getdata 86 only_best_and_pre_1_wo_ratio cft_bft_exp_step350 cft_two_agents latency

getdata 76 only_best_and_pre_1_wo_ratio cft_bft_exp_random cft_two_agents latency

getdata 87 only_best_and_pre_1_wo_ratio cft_bft_exp_step20 cft_two_agents latency

getdata 70 only_best_and_pre_1_wo_ratio cft_bft_exp_step350 cft_two_agents tps
getdata 71 only_best_and_pre_1_wo_ratio cft_bft_exp_random cft_two_agents tps
getdata 75 only_best_and_pre_1_wo_ratio cft_bft_exp_step20 cft_two_agents tps



getdata 77 only_best_and_pre_1_wo_ratio loop_w w_exp_two_agents latency

getdata 78 only_best_and_pre_1_wo_ratio loop_w w_exp_two_agents latency

getdata 78 only_best_and_pre_1_wo_ratio loop_w w_exp_two_agents latency


getdata 83 only_best_and_pre_1_wo_ratio loop_w_2 w_exp_two_agents latency

getdata 84 only_best_and_pre_1_wo_ratio loop_w_2 w_exp_two_agents latency

getdata 85 only_best_and_pre_1_wo_ratio loop_w_2 w_exp_two_agents latency


getdata 72 only_best_and_pre_1_wo_ratio loop_w w_exp_two_agents tps

getdata 73 only_best_and_pre_1_wo_ratio loop_w w_exp_two_agents tps

getdata 74 only_best_and_pre_1_wo_ratio loop_w w_exp_two_agents tps

getdata 80 only_best_and_pre_1_wo_ratio loop_w_2 w_exp_two_agents tps

getdata 81 only_best_and_pre_1_wo_ratio loop_w_2 w_exp_two_agents tps
getdata 82 only_best_and_pre_1_wo_ratio loop_w_2 w_exp_two_agents tps

mkdir -p /users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/experiments/random_change
getdata 88 only_best_and_pre_1_wo_ratio mutate_env random_change latency
getdata 89 only_best_and_pre_1_wo_ratio mutate_env random_change tps


rsync -av  worker-065:/data-ssd/tianwen/workers/worker065_Altun/inference_log_only_best_and_pre_1_wo_ratio_env_mutation_loop_w_3_tps_w_random/ experiments/inference_w/inference_log_only_best_and_pre_1_wo_ratio_env_mutation_loop_w_3_tps_w_random_65/

rsync -av  worker-064:/data-ssd/tianwen/workers/worker064_Altun/inference_log_only_best_and_pre_1_wo_ratio_env_mutation_loop_w_3_latency_w_random/ experiments/inference_w/inference_log_only_best_and_pre_1_wo_ratio_env_mutation_loop_w_3_latency_w_random_64/