#!/usr/bin/env bash
# set -x
ppkill () {
        pids=( $(pgrep -f $1) )
        for pid in "${pids[@]}"; do
                if [[ $pid != $$ ]]; then
                        kill "$pid"
                fi
        done
}


SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
DEFAULT_CONFIG_FILE="${SCRIPT_DIR}/template/cft-config-template.json"
CONFIG_FILE="${SCRIPT_DIR}/config_cft.json"

JSON_FILE=${1:-"${SCRIPT_DIR}/config_RL_default.json"}
alg=$(jq -r '.algorithm' "$JSON_FILE")
replicas=$(jq '.nodesize' "$JSON_FILE")
clients=$(jq '.clients' "$JSON_FILE")
concurrency=$(jq '.concurrency' "$JSON_FILE")
ratelimit=$(jq '.ratelimit' "$JSON_FILE")
batchsize=$(jq '.batchsize' "$JSON_FILE")
payloadsize=$(jq '.payloadsize' "$JSON_FILE")
requests=$(jq -r '.requests' "$JSON_FILE")
writeratio=$(jq -r '.writeratio' "$JSON_FILE")
distribution=$(jq -r '.distribution' "$JSON_FILE")
buffersize=$((batchsize*payloadsize))
# Import config update library
. ${SCRIPT_DIR}/env.sh
if [ "$alg" = "vpaxos" ] && [ "$replicas" = 4 ]; then
    NODES_PER_WORKER=2
fi
# START_NODE=94
# START_NODE="127.0.0.1"
. ${SCRIPT_DIR}/update_cft_config.sh

cp ${DEFAULT_CONFIG_FILE} ${CONFIG_FILE}
echo "algorithm: ${alg}"
echo "requests: ${requests}"
update_config_value "${CONFIG_FILE}" "algorithm" "${alg}"
update_config_value "${CONFIG_FILE}" "N" "${requests}"
update_config_value "${CONFIG_FILE}" "W" "${writeratio}"
update_config_value "${CONFIG_FILE}" "Concurrency" ${concurrency}
update_config_value "${CONFIG_FILE}" "Distribution" ${distribution}
update_config_value "${CONFIG_FILE}" "Throttle" ${ratelimit}
update_config_value "${CONFIG_FILE}" "buffer_size" ${buffersize}
update_config_value "${CONFIG_FILE}" "chan_buffer_size" ${buffersize}
# Read nREPLICAS from the config file
echo "replicas: ${replicas} ${CONFIG_FILE}"

update_node_address "${CONFIG_FILE}" ${replicas} ${NODES_PER_WORKER} ${START_NODE}


ppkill cft-server
ppkill cft-client
rm -f ${SCRIPT_DIR}/cft-server.*.log
rm -f ${SCRIPT_DIR}/cft-client.*.log
rm -f ${SCRIPT_DIR}/benchmark.log
rm -f ${SCRIPT_DIR}/history.csv
rm -f ${SCRIPT_DIR}/latency

echo $CONFIG_FILE


SERVER_WORKERS=$((replicas / NODES_PER_WORKER + (replicas % NODES_PER_WORKER == 0 ? 0 : 1)))

echo "ok: new config updated: ${alg}"
echo "Start all server nodes: $SERVER_WORKERS*$NODES_PER_WORKER"
echo "Copying server configuration files"
# set -x
for i in $(seq 1 $SERVER_WORKERS); do
    pub_ip=$(printf "%03d" $((START_NODE + i - 1)))
    pub="worker-${pub_ip}"
    scp $ssh_options ${SCRIPT_DIR}/stop.sh $ssh_user@$pub:/data-ssd/tianwen/ >/dev/null
    ssh $ssh_user@$pub $ssh_options "cd /data-ssd/tianwen/ && source stop.sh"
    ssh $ssh_user@$pub $ssh_options "rm -rf /data-ssd/tianwen/go/* && mkdir -p /data-ssd/tianwen/go/cft/ "
    scp $ssh_options ${SCRIPT_DIR}/config_cft.json $ssh_user@$pub:/data-ssd/tianwen/go/cft/config_cft.json >/dev/null 
    scp $ssh_options ${SCRIPT_DIR}/cft-server $ssh_user@$pub:/data-ssd/tianwen/go/cft/ >/dev/null 
done

echo "Start servers with configuration"
for i in $(seq 1 $SERVER_WORKERS); do
    pub_ip=$(printf "%03d" $((START_NODE + i - 1)))
    pub="worker-${pub_ip}"
    for j in $(seq 1 $NODES_PER_WORKER); do
        id=$(( NODES_PER_WORKER * (i - 1) + j ))
        if (( id > replicas )); then
            break
        fi
        ssh $ssh_user@$pub $ssh_options "cd /data-ssd/tianwen/go/cft/ && nohup ./cft-server -id $i.$j -algorithm ${alg} -config config_cft.json > server$j.log 2>&1 & " &
    done
done

sleep 6

# clients=6 # Test!
echo "Start clients with configuration: $clients"
CLIENT_WORKERS=$((clients / CLIENTS_PER_WORKER + (clients % CLIENTS_PER_WORKER == 0 ? 0 : 1)))
echo "Use CLIENT_WORKER size = $CLIENT_WORKERS*$CLIENTS_PER_WORKER in config file"

for i in $(seq 1 $CLIENT_WORKERS); do
    pub_ip=$(printf "%03d" $((START_NODE + SERVER_WORKERS + i - 1)))
    pub="worker-${pub_ip}"
    scp $ssh_options ${SCRIPT_DIR}/stop.sh $ssh_user@$pub:/data-ssd/tianwen/ >/dev/null 
    ssh $ssh_user@$pub $ssh_options "cd /data-ssd/tianwen/ && source stop.sh"
    ssh $ssh_user@$pub $ssh_options "rm -rf /data-ssd/tianwen/go/* && mkdir -p /data-ssd/tianwen/go/cft/"
    scp $ssh_options ${SCRIPT_DIR}/config_cft.json $ssh_user@$pub:/data-ssd/tianwen/go/cft/config_cft.json >/dev/null 
    scp $ssh_options ${SCRIPT_DIR}/cft-client $ssh_user@$pub:/data-ssd/tianwen/go/cft/ >/dev/null 
done

for i in $(seq 1 $CLIENT_WORKERS); do
    pub_ip=$(printf "%03d" $((START_NODE + SERVER_WORKERS + i - 1)))
    pub="worker-${pub_ip}"
    for j in $(seq 1 $CLIENTS_PER_WORKER); do
        id=$(( CLIENTS_PER_WORKER * (i - 1) + j ))
        if (( id > clients )); then
            break
        fi
        ssh $ssh_user@$pub $ssh_options "cd /data-ssd/tianwen/go/ && cp -r cft cft$j"
        ssh $ssh_user@$pub $ssh_options "cd /data-ssd/tianwen/go/cft$j/ && nohup ./cft-client -id 1.1 -algorithm ${alg} -config config_cft.json > client$id.log 2>&1 & " &
    done
done

sleep 20
# sleep ${SERVER_WORKERS}

echo "benchmarking... $CLIENT_WORKERS"
for i in $(seq 1 $CLIENT_WORKERS); do
    pub_ip=$(printf "%03d" $((START_NODE + SERVER_WORKERS + i - 1)))
    pub="worker-${pub_ip}"
    while ssh $ssh_options $ssh_user@$pub "pgrep -f cft-client" >/dev/null 2>&1; do
        sleep 2
        echo "Waiting for cft-client process to finish on $pub..."
    done
    if ssh $ssh_options $ssh_user@$pub "ls /data-ssd/tianwen/go/cft*/cft-client.*.log" >/dev/null 2>&1; then
        scp $ssh_options $ssh_user@$pub:/data-ssd/tianwen/go/cft*/cft-client.*.log ${SCRIPT_DIR}/ 
    else
        echo "Warning: No cft-client.*.log files found on remote server: $pub"
    fi
done

# set +x
echo "ok: benchmark done"

TPS=$(grep "Throughput = " cft-client.*.log 2>/dev/null | cut -d= -f2 | paste -sd+ | bc) || TPS=0
LAT=$(grep "mean = " cft-client.*.log 2>/dev/null | cut -d= -f2 | awk '{sum += $1; count++} END {print (count > 0) ? sum/count : 0}')
echo "{\"TPS\":${TPS}, \"latency\": ${LAT} }" > benchmark.log

cat benchmark.log

# rm -f ${SCRIPT_DIR}/cft-server.*.log
# rm -f ${SCRIPT_DIR}/cft-client*.log
# rm history.csv
# rm latency

# {"TPS": 6018.429342, "latency":  4014.059021 }
#  2500, 1500
# {"TPS": 6991.046476, "latency":  3261.838016 3685.999743 2699.505987 }
# {"TPS":6601.240207, "latency": 22972.885535 }
# {"TPS":3026.223896, "latency": 15015.321069 }







