#!/bin/bash
#set -x
# File to be monitored
FILE_TO_MONITOR=${1:-"config_RL.json"}
# Interval between checks (in seconds)
CHECK_INTERVAL=3
version=0
# Add additional actions here, such as sending an alert or logging
alg=$(grep algorithm ${FILE_TO_MONITOR} |  cut -d \" -f 4)
# Check if alg equals to cft
if [[ "${alg}" == "paxos" || "${alg}" == "epaxos" || "${alg}" == "vpaxos" || "${alg}" == "wpaxos" ]]; then
  echo "Alg is valid: ${alg}"
else
  echo "Invalid Alg: ${alg}"
  exit 1
fi


echo "ok: new config generated ${alg}"
echo "ok: new config applied chain"
echo "=======================start running================"


./apply.sh ${alg}
echo "ok: cft-server apply config"
sleep 3
./cft-client -id 1.1 -config ${FILE_TO_MONITOR}
echo "ok: benchmark done"

TPS=$(grep "Throughput = " cft-client.*.log | cut -d = -f 2)
LAT=$(grep "mean = " cft-client.*.log | cut -d = -f 2)
echo "{\"TPS\":${TPS}, \"latency\": ${LAT} }" > benchmark.log
#echo ((++version)) > RL_version
cat benchmark.log
pkill cft-server
pkill cft-client


