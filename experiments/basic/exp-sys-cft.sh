#!/bin/bash
set -x

update_json() {
  local json_file="$1"
  local key="$2"
  local new_value="$3"

  python3 -c "
import json
import sys

json_file = '$json_file'
key = '$key'
new_value = '$new_value'

# Attempt to convert new_value to int or float
try:
    if '.' in new_value:
        new_value = float(new_value)
    else:
        new_value = int(new_value)
except ValueError:
    pass

with open(json_file, 'r') as f:
    data = json.load(f)

# Split keys for nested dictionaries
def update_nested(d, keys, value):
    keys = keys.split('.')
    for k in keys[:-1]:
        d = d.setdefault(k, {})
    d[keys[-1]] = value

update_nested(data, key, new_value)

with open(json_file, 'w') as f:
    json.dump(data, f, indent=4)
"
}


# cd `dirname ${BASH_SOURCE-$0}`
# CFT env parameters
nconsensus="paxos epaxso wpaxos vpaxos"
npolicy="majorit ema consecutive"
nREPLICAS="4 8 16 32 64"
nCLIENTS="1 10 100 500 1000 5000 10000"
nconcurrency="1 10 100 500 1000 5000 10000 100000"
nratelimit="10 100 500 1000 5000 10000 100000"
# nbatchsize="1 10 100 500 1000 5000 10000 "
# npayloadsize="0 128 256 512 1024 2048"
nthreshold="3 10 20 30 50 100" #delay/interval
nthrifty="false true"
nchan_buffer_size="256 512 1024 2048 4096 8192 10240"
nbuffer_size="256 512 1024 2048 4096 8192 10240"
nmultiversion="false true"
T="10" #  time
N="1" #  round
K="100 1000 5000 100000" #  key
W="0 0.2 0.4 0.6 0.8 1" # write ratio
Throttle="0" # ratelimit
# Concurrency="500"
nDistribution="uniform zipfian normal conflict exponential" # ！
LinearizabilityCheck="false"
nConflicts="0 30 50 70 100 " # ！
Min="0"
Mu="500"
Sigma="50"
Move="false"
Speed="10"
Zipfian_s="2"
Zipfian_v="1"
nLambda=(0.01 0.1 0.5 0.7 0.9) #exponential

CONFIG_FILE="config.exp.json"
configTemplate="config.json"
expFile="./benchmark/runcft.log"
mkdir -p ./benchmark

for CNS in ${nconsensus}; do
    echo "Experiments for ${CNS}" > ${expFile}
    
    echo "Experiment 1: threshold"
    for CTR in ${nthreshold}; do
        cp $configTemplate ${CONFIG_FILE}
        update_json ${CONFIG_FILE} "algorithm" ${CNS}
        update_json ${CONFIG_FILE} "threshold" ${CTR}
        cat ${CONFIG_FILE}
        ./run_config.sh ${CONFIG_FILE}
        echo "threshold-${CNS}-${CTR}" >> ${expFile}
        tail -n 1 ./benchmark.log >> ${expFile}
    done

    echo "Experiment 5: Ratelimiter Throttle="0""
    for CTR in $nratelimit; do
        cp $configTemplate ${CONFIG_FILE}
        update_json ${CONFIG_FILE} "algorithm" ${CNS}
        update_json ${CONFIG_FILE} "benchmark.Throttle" ${CTR}
        ./run_config.sh ${CONFIG_FILE}
        echo "Ratelimiter-${CNS}-${CTR}" >> ${expFile}
        tail -n 1 ./benchmark.log >> ${expFile}
    done



    echo "Experiment 2: Distribution"
    for CTD in ${nDistribution}; do
        cp $configTemplate ${CONFIG_FILE}
        update_json ${CONFIG_FILE} "algorithm" ${CNS}
        update_json ${CONFIG_FILE} "benchmark.Distribution" ${CTD}
        ./run_config.sh ${CONFIG_FILE}
        echo "Distribution-${CNS}-${CTD}" >> ${expFile}
        tail -n 1 ./benchmark.log >> ${expFile}
    done

    echo "Experiment 3: Conflicts"
    for CTC in ${nConflicts}; do
        cp $configTemplate ${CONFIG_FILE}
        update_json ${CONFIG_FILE} "algorithm" ${CNS}
        update_json ${CONFIG_FILE} "benchmark.Distribution" "conflicts"
        update_json ${CONFIG_FILE} "benchmark.Conflicts" ${CTC}
        ./run_config.sh ${CONFIG_FILE}
        echo "Conflicts-${CNS}-${CTC}" >> ${expFile}
        tail -n 1 ./benchmark.log >> ${expFile}
    done

    echo "Experiment 4: exponential"
    for CTE in ${nLambda}; do
        cp $configTemplate ${CONFIG_FILE}
        update_json ${CONFIG_FILE} "algorithm" ${CNS}
        update_json ${CONFIG_FILE} "benchmark.Distribution" "exponential"
        update_json ${CONFIG_FILE} "benchmark.exponential" ${CTE}
        ./run_config.sh ${CONFIG_FILE}
        echo "exponential-${CNS}-${CTE}" >> ${expFile}
        tail -n 1 ./benchmark.log >> ${expFile}
    done

   

done


# sed -i "s/CONSENSUS/${CNS}/" ${CONFIG_FILE}
# jq '.algorithm = "${CNS}"' ${CONFIG_FILE} > temp.json