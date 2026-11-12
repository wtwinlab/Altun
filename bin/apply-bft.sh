#!/bin/bash
# set -x

timestamp=$(date +%s)
dir=$(dirname "${BASH_SOURCE[0]}")
alg=${1:-"chainedhotstuff"}
concurrency=${2:-1}
replicas=${3:-4}
clients=${4:-1} 
batchsize=${5:-1}
payloadsize=${6:0}
leaderrotation=${7:-"round-robin"}
ratelimit=${8}

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

rm -rf ./bft-performance.json
rm -rf ./output
rm -rf ./benchmark.log
# echo "$PWD"

echo "ok: new config generated ${alg}"
./bft run ${configs} --measurement-interval 10s --duration 15s &> bft-run.log 
echo "ok: benchmark done"
sleep 7
./bft-cli ./output/local/measurements.json
sleep 2

tail -n 1 ./bft-performance.json > benchmark.log
# timeout 30 bash -c -- 'while [ ! -f ./bft-performance.json ]
# do
#   echo "not found, try calculate again"
#   ./bft-cli ./output/local/measurements.json
#   sleep 2
# done'
# cat ./bft-performance.json
# tail -n 1 ./bft-performance.json >> ./benchmark/${alg}-${concurrency}-${replicas}-${clients}-${batchsize}-${payloadsize}-${leaderrotation}-${ratelimit}.log
echo "${alg}-${concurrency}-${replicas}-${clients}-${batchsize}-${payloadsize}-${leaderrotation}-${ratelimit}" >> ./benchmark/exp.log
tail -n 1 ./bft-performance.json >> ./benchmark/exp.log