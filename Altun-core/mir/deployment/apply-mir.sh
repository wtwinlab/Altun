#!/bin/bash -e
set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

NUM_NODES=${1:-4}
NUM_CLIENTS=${2:-1}
CONCURRENCY=${3:-4}
RATE_LIMIT=${4:-1000}
PAYLOAD_SIZE=${5:-500}
BATCH_SIZE=${6:-100}
REQUESTS=${7:-10000}
NUM_LEADERS=${8:-1}
BATCH_SIZE_BYTES=$((BATCH_SIZE*PAYLOAD_SIZE))
# payload > batchSizeBytes
date
#  local
# echo "Algorithm: mir"
# ${SCRIPT_DIR}/config-gen.sh -l ${NUM_NODES} ${NUM_CLIENTS} ${CONCURRENCY} ${RATE_LIMIT} ${PAYLOAD_SIZE} ${BATCH_SIZE_BYTES}
# echo "Run benchmark"
# ${SCRIPT_DIR}/run-local.sh ${NUM_NODES} ${NUM_CLIENTS}

echo "Algorithm: mir"
${SCRIPT_DIR}/config-gen.sh ${NUM_NODES} ${NUM_CLIENTS} ${CONCURRENCY} ${RATE_LIMIT} ${PAYLOAD_SIZE} ${BATCH_SIZE_BYTES} ${REQUESTS} ${NUM_LEADERS}
echo "Run benchmark"
${SCRIPT_DIR}/run.sh #&> ${SCRIPT_DIR}/mir-run.log 

# set -x
result=$(cat "${SCRIPT_DIR}/mir-exp.out")
if ! TPS=$(echo "$result" | grep -oP 'Throughput: \K[\d.]+'); then
    TPS=0
fi

if ! LAT=$(echo "$result" | grep -oP 'Average end to end latency: \K-?[0-9.]+'); then
    LAT=0
fi

# Write to benchmark log 
printf '{"TPS":%s, "latency": %s}\n' "$TPS" "$LAT" > "${SCRIPT_DIR}/../../bin/benchmark.log"
date
cat ${SCRIPT_DIR}/../../bin/benchmark.log