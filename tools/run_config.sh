#!/bin/bash
set -x

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

JSON_FILE=${1:-"${SCRIPT_DIR}/config_RL.json"}

# Ensure the JSON file exists
if [ ! -f "$JSON_FILE" ]; then
    echo "Error: $JSON_FILE does not exist. Use default config file: config.json"
    exit 1
fi

# Read values from the JSON file
required_fields=(
    .algorithm
    .nodesize
    .clients
    .concurrency
    .ratelimit
    .batchsize
    .payloadsize
    .requests
    .writeratio
    .distribution
)

for field in "${required_fields[@]}"; do
    if ! jq -e "has(\"${field#.}\")" "$JSON_FILE" >/dev/null; then
        echo "Error: Required field '${field#.}' missing from JSON file"
        exit 1
    fi
done

# safely extract values
alg=$(jq -r '.algorithm' "$JSON_FILE")
replicas=$(jq '.nodesize' "$JSON_FILE")
clients=$(jq '.clients' "$JSON_FILE")
concurrency=$(jq '.concurrency' "$JSON_FILE")
nratelimit=$(jq '.ratelimit' "$JSON_FILE")
batchsize=$(jq '.batchsize' "$JSON_FILE")
payloadsize=$(jq '.payloadsize' "$JSON_FILE")
requests=$(jq -r '.requests' "$JSON_FILE")
writeratio=$(jq -r '.writeratio' "$JSON_FILE")
distribution=$(jq -r '.distribution' "$JSON_FILE")


echo "======================= start running ================"
rm -f ${SCRIPT_DIR}/benchmark.log

if [[ "${alg}" == "paxos" || "${alg}" == "epaxos" || "${alg}" == "vpaxos" || "${alg}" == "wpaxos" ]]; then
    echo "Running CFT algorithm: ${alg}"
    ${SCRIPT_DIR}/apply-cft.sh ${JSON_FILE} > ${SCRIPT_DIR}/cft-run.log
    # if ! ${SCRIPT_DIR}/apply-cft.sh ${JSON_FILE}; then
    #     echo "Error: apply-cft.sh script failed"
    #     exit 1
    # fi
elif [[ "$alg" == "chainedhotstuff" || "${alg}" == "simplehotstuff" || "${alg}" == "fasthotstuff" ]]; then
    echo "Running BFT algorithm: ${alg}"
    ${SCRIPT_DIR}/apply-bft.sh ${alg} ${concurrency} ${replicas} ${clients} ${batchsize} ${payloadsize} ${nratelimit} > ${SCRIPT_DIR}/bft-run.log
    # if ! ${SCRIPT_DIR}/apply-bft.sh ${alg} ${concurrency} ${replicas} ${clients} ${batchsize} ${payloadsize} ${nratelimit}; then
    #     echo "Error: apply_bft.sh script failed"
    #     exit 1
    # fi
elif [[ "$alg" == "mirbft" ]]; then
    leaders=${replicas}
    echo "Running BFT algorithm: ${alg} ${leaders}"
    ${SCRIPT_DIR}/../mir/deployment/apply-mir.sh ${replicas} ${clients} ${concurrency} ${nratelimit} ${payloadsize} ${batchsize} ${requests} ${leaders} > ${SCRIPT_DIR}/pbft-run.log
    # if ! ${SCRIPT_DIR}/../mir/deployment/apply-mir.sh ${replicas} ${clients} ${concurrency} ${nratelimit} ${payloadsize} ${batchsize} ${requests} ${leaders}; then
    #     echo "Error: apply_mir.sh script failed"
    #     exit 1
    # fi
elif [[ "$alg" == "pbft" ]]; then
    leaders=1
    echo "Running BFT algorithm: ${alg}"
    ${SCRIPT_DIR}/../mir/deployment/apply-mir.sh ${replicas} ${clients} ${concurrency} ${nratelimit} ${payloadsize} ${batchsize} ${requests} ${leaders} > ${SCRIPT_DIR}/mir-run.log
    # if ! ${SCRIPT_DIR}/../mir/deployment/apply-mir.sh ${replicas} ${clients} ${concurrency} ${nratelimit} ${payloadsize} ${batchsize} ${requests} ${leaders}; then
    #     echo "Error: apply_mir.sh script failed"
    #     exit 1
    # fi
else
  echo "Invalid Alg: ${alg}"
  exit 1
fi   


cat ${SCRIPT_DIR}benchmark.log 2>/dev/null || true
echo "ok: ${alg} apply config sucessfully. "