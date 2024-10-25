#!/usr/bin/env bash
#set -x

mv cft-client.*.log ./log/
./apply.sh paxos

mkdir -p log

#mv cft-client.*.log ./log/

./cft-client -id 1.1 -config config.json

TPS=$(grep "Throughput = " cft-client.*.log | cut -d = -f 2) 
LAT=$(grep "mean = " cft-client.*.log | cut -d = -f 2)
echo "{\"TPS\":${TPS}, \"latency\": ${LAT} }" > benchmark.log
echo 0 > RL_version
cat benchmark.log

./monitor.sh 
