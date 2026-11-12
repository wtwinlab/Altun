#!/bin/bash
set -x



JSON_FILE="config_RL.json"

# Ensure the JSON file exists
if [ ! -f "$JSON_FILE" ]; then
    echo "Error: $JSON_FILE does not exist."
    exit 1
fi

# Read values from the JSON file
alg=$(jq -r '.algorithm' "$JSON_FILE")
replicas=$(jq '.bft_config.nREPLICAS' "$JSON_FILE")
clients=$(jq '.bft_config.nCLIENTS' "$JSON_FILE")
concurrency=$(jq '.bft_config.nconcurrency' "$JSON_FILE")
nratelimit=$(jq '.bft_config.nratelimit' "$JSON_FILE")
batchsize=$(jq '.bft_config.nbatchsize' "$JSON_FILE")
payloadsize=$(jq '.bft_config.npayloadsize' "$JSON_FILE")
leaderrotation=$(jq -r '.bft_config.nleaderrotation' "$JSON_FILE")

 ./apply-bft.sh ${alg} ${concurrency} ${replicas} ${clients} ${batchsize} ${payloadsize} ${leaderrotation} ${nratelimit}
    