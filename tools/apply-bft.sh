#!/bin/bash
set -x

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

. ${SCRIPT_DIR}/env.sh
BFT_DIR="${REMOTE_DIR}/bft"

timestamp=$(date +%s)
alg=${1:-"chainedhotstuff"}
concurrency=${2:-100000}
replicas=${3:-4}
clients=${4:-1} 
batchsize=${5:-1}
payloadsize=${6:-0}
ratelimit=${7:-100000}
leaderrotation=${8:-"round-robin"}
# concurrency * clients > 3 * batchsize


configs="--consensus ${alg} \
--max-concurrent ${concurrency}  \
--replicas ${replicas} \
--clients ${clients} \
--batch-size ${batchsize} \
--payload-size ${payloadsize} \
--leader-rotation ${leaderrotation} "
if [ -z ${ratelimit} ]; then
     configs="${configs}"
else
    configs="${configs} --rate-limit ${ratelimit}"
fi

if [ ${local} = true ]; then
    configs="${configs}"
else
    # For remote run if local=fase
    exepath="${SCRIPT_DIR}/bft-server"
    sshconfigfile="${SCRIPT_DIR}/ssh_config"
    hosts=""
    SERVER_WORKERS=$((replicas / NODES_PER_WORKER + (replicas % NODES_PER_WORKER == 0 ? 0 : 1)))
    CLIENT_WORKERS=$((clients / CLIENTS_PER_WORKER + (clients % CLIENTS_PER_WORKER == 0 ? 0 : 1)))
    HOST_NUM=$((SERVER_WORKERS + CLIENT_WORKERS))
    for i in $(seq 1 $HOST_NUM); do
        pub_ip=$(printf "%03d" $((START_NODE + i -1)))
        pub="worker-${pub_ip}"
        if [ $i -eq 1 ]; then
            hosts=$pub
        else
            hosts=${hosts}","$pub
        fi
    done
    
    echo "hosts: [$hosts]"
    configs="${configs} --hosts ${hosts}"
    configs="${configs} --exe ${exepath}"
    configs="${configs} --ssh-config ${sshconfigfile}"
fi

echo "configs: ${configs}"

rm -rf ./bft-performance.json
rm -rf ./output
rm -rf ./benchmark.log


echo "ok: apply new config: ${alg}"
./bft-server run ${configs} --measurement-interval 5s --duration 15s &> bft-run.log 

sleep 7
echo "ok: benchmark run"
pkill bft-client
for i in $(seq 1 $HOST_NUM); do
    pub_ip=$(printf "%03d" $((START_NODE + i -1)))
    pub="worker-${pub_ip}"
    ./bft-client ./output/${pub}/measurements.json & 
done  
# ./bft-client ./output/*/measurements.json
sleep 2
   
tail -n 1 ./bft-performance.json > benchmark.log
cat benchmark.log
pkill bft-client

# timeout 30 bash -c -- 'while [ ! -f ./bft-performance.json ]
# do
#   echo "not found, try calculate again"
#   ./bft-client ./output/local/measurements.json
#   sleep 2
# done'
# cat ./bft-performance.json
# tail -n 1 ./bft-performance.json >> ./benchmark/${alg}-${concurrency}-${replicas}-${clients}-${batchsize}-${payloadsize}-${leaderrotation}-${ratelimit}.log
# echo "${alg}-${concurrency}-${replicas}-${clients}-${batchsize}-${payloadsize}-${leaderrotation}-${ratelimit}" >> ./benchmark/exp.log
# tail -n 1 ./bft-performance.json >> ./benchmark/exp.log