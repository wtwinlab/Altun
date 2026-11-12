#!/bin/bash
# set -x

dir=$(dirname "${BASH_SOURCE[0]}")
DEFAULT_CONFIG_FILE="config_RL.json"
defaultAlg=$(grep algorithm ${DEFAULT_CONFIG_FILE} |  cut -d \" -f 4)
defaultPol=$(grep -o '"policy": "[^"]*"' ${DEFAULT_CONFIG_FILE} | sed 's/.*: "//;s/"//g')
defaultCon=$(grep -o '"Concurrency": [0-9]*' ${DEFAULT_CONFIG_FILE} | sed 's/[^0-9]*//g')
if [ -z "${defaultAlg}" ]; then
    defaultAlg=chainedhotstuff
fi
if [ -z "${defaultCon}" ]; then
    defaultCon=500
fi
alg=${1:-${defaultAlg}}
concurrency=${2:-${defaultCon}}
policy=${3:-${defaultPol}}
batchsize=${4:-1000}

rm -rf ./bft-performance.json
rm -rf ./output
rm -rf ./benchmark.log

# Check if alg equals to bft
if [[ "${alg}" == "chainedhotstuff" || "${alg}" == "simplehotstuff" || "${alg}" == "fasthotstuff" ]]; then
  echo "Alg is valid: ${alg}"
else
  echo "Invalid alg: ${alg}"
  exit 1
fi
# Check if policy equals to "fixed" or "round-robin"
if [[ "${policy}" == "fixed" || "${policy}" == "round-robin" ]]; then
  echo "Policy is valid: ${policy}"
else
  echo "Invalid policy: ${policy}, use default 'fixed'"
  policy="fixed"
fi

echo "ok: new config generated ${alg}"

./bft-server run  --consensus ${alg} --max-concurrent ${concurrency} --measurement-interval 10s --duration 15s --replicas 4 --clients 4 --batch-size ${batchsize} --payload-size 128 --leader-rotation ${policy} &> bft-run.log 

echo "ok: benchmark done"

sleep 7
./bft-client ./output/local/measurements.json
sleep 1
timeout 30 bash -c -- 'while [ ! -f ./bft-performance.json ]
do
  ./bft-client  ./output/local/measurements.json
  sleep 1
done'
#cat ./bft-performance.json
tail -n 1 ./bft-performance.json > benchmark.log
