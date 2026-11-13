#!/bin/bash 

set -x

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
echo "Killing previously running servers and clients"
pkill mir-server
pkill mir-client
rm -rf status
NUM_NODES=${1:-4}
NUM_CLIENTS=${2:-1}

echo "Start nodes"
for i in $(seq 1 $NUM_NODES); do
    # servers+="server$i "
    nohup ${SCRIPT_DIR}/../bin/mir-server ${SCRIPT_DIR}/config/serverconfig/config_server$i.yml server$i > ${SCRIPT_DIR}/server-$i.out 2>&1 &
done
for i in $(seq 1 $NUM_CLIENTS); do
    # clients+="client$i "
    nohup ${SCRIPT_DIR}/../bin/mir-client ${SCRIPT_DIR}/config/clientconfig/config_client$i.yml client$i > ${SCRIPT_DIR}/client-$i.out 2>&1 &
done

python3 ${SCRIPT_DIR}/../tools/perf-eval.py $NUM_NODES $NUM_CLIENTS ${SCRIPT_DIR}/server*.out ${SCRIPT_DIR}/client*.trc  > ${SCRIPT_DIR}/mir-exp.out
cat ${SCRIPT_DIR}/mir-exp.out

# 4 nodes
# nohup ./bin/server ./sampleconfig/serverconfig/config1.yml server1 > ./server-1.out 2>&1 &
# nohup ./bin/server ./sampleconfig/serverconfig/config2.yml server2 > ./server-2.out 2>&1 &
# nohup ./bin/server ./sampleconfig/serverconfig/config3.yml server3 > ./server-3.out 2>&1 &
# nohup ./bin/server ./sampleconfig/serverconfig/config4.yml server4 > ./server-4.out 2>&1 &
# # 1 client
# ./bin/client ./sampleconfig/clientconfig/4peer-config.yml client > ./client.log 2>&1 &
# #./client ../sampleconfig/clientconfig/4peer-config.yml client 

# # calculate
# python3 tools/perf-eval.py 4 1 ./server*.out ./client*.trc




# ./server ../sampleconfig/serverconfig/config1.yml server1 2>&1 | tee server-1.out &
