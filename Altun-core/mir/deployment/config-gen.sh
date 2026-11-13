#!/bin/bash -e

# set -x

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

. ${SCRIPT_DIR}/../../bin/env.sh

# Host naming and remote paths come from bin/env.sh:
#   WORKER_PREFIX  host name prefix, hosts are ${WORKER_PREFIX}<3-digit index> (default worker-)
#   REMOTE_DIR     per-host working directory that receives the mir<id> directories
#   REMOTE_BASE    parent of REMOTE_DIR, where stop.sh is placed (default: dirname of REMOTE_DIR)
WORKER_PREFIX=${WORKER_PREFIX:-worker-}
REMOTE_BASE=${REMOTE_BASE:-$(dirname "$REMOTE_DIR")}
. ${SCRIPT_DIR}/lib.sh

#   local=false # setting env.sh
NUM_NODES=${1:-4}
NUM_CLIENTS=${2:-1}
CONCURRENCY=${3:-1}
RATE_LIMIT=${4:-1000}
PAYLOAD_SIZE=${5:-500}
BATCH_SIZE_BYTES=${6:-2000000}
REQUESTS=${7:-10000}
NUM_LEADERS=${8:-1}
F=$(((NUM_NODES-1)/3))
thres=16
if [ $NUM_NODES -ge $thres ]; then
    watermark=$(( NUM_NODES*2 ))
    epoch=$(( NUM_NODES*16 ))
    timeout=$(( NUM_NODES*32*1000000 ))
else
    watermark=32
    epoch=256
    timeout=500000000
fi

SERVER_WORKERS=$((NUM_NODES / NODES_PER_WORKER + (NUM_NODES % NODES_PER_WORKER == 0 ? 0 : 1)))
CLIENT_WORKERS=$((NUM_CLIENTS / CLIENTS_PER_WORKER + (NUM_CLIENTS % CLIENTS_PER_WORKER == 0 ? 0 : 1)))
echo "Use SERVER_WORKERS size = $SERVER_WORKERS *$NODES_PER_WORKER"
echo "Use CLIENT_WORKER size = $CLIENT_WORKERS *$CLIENTS_PER_WORKER"
echo "Use NUM_LEADERS = $NUM_LEADERS"



echo "#################### Generating cluster-instance.info"
echo '' > ${SCRIPT_DIR}/cluster-instance.info
for i in $(seq 1 $SERVER_WORKERS); do
    WORKER_ID=$(printf "%03d" $((START_NODE + i - 1)))
    pub="${WORKER_PREFIX}${WORKER_ID}"
    for j in $(seq 1 $NODES_PER_WORKER); do
        id=$((NODES_PER_WORKER * (i - 1) + j))
        if (( id > NUM_NODES )); then
            break
        fi
        p="server${id}"
        # Container mode: certificates and addresses name the replica's container alias (bin/altunwan.sh)
        [ "${ALTUNWAN:-0}" = 1 ] && pub="altunwan-${WORKER_ID}-r${j}"
        echo "${p} ${pub} ${pub}" >> ${SCRIPT_DIR}/cluster-instance.info       
    done
done
for i in $(seq 1 $CLIENT_WORKERS); do
    WORKER_ID=$(printf "%03d" $((START_NODE + SERVER_WORKERS + i - 1)))
    pub="${WORKER_PREFIX}${WORKER_ID}"
    for j in $(seq 1 $CLIENTS_PER_WORKER); do
        id=$((CLIENTS_PER_WORKER * (i - 1) + j))
        if (( id > NUM_CLIENTS )); then
            break
        fi
        p="client${id}"
        echo "${p} ${pub} ${pub}" >> ${SCRIPT_DIR}/cluster-instance.info
    done
done
echo "#######${servers}, ${clients}########"
cat ${SCRIPT_DIR}/cluster-instance.info
servers=$(grep server ${SCRIPT_DIR}/cluster-instance.info | awk '{ print $1}')
clients=$(grep client ${SCRIPT_DIR}/cluster-instance.info | awk '{ print $1}')



echo "#################### Generating Certificates"
echo "Removing old configuration"
rm -rf ${SCRIPT_DIR}/temp
mkdir -p ${SCRIPT_DIR}/temp
cd ${SCRIPT_DIR}/temp
${SCRIPT_DIR}/generate-ca.sh -f
cd ..
for p in $servers; do
    pub=$(getIP $p)
    priv=$(getPrivIP $p)
    echo "$pub,$priv"
    cd ${SCRIPT_DIR}/temp
    # IP.3 = PRIV-IP
    cat ${SCRIPT_DIR}/config-file-templates/openssl-template.conf | sed "s/PUB-IP/$pub/ ; s/PRIV-IP/$priv/"> ${SCRIPT_DIR}/temp/openssl.conf
    genCert $p
    cd ..
done


echo "#################### Generating address:"
address=""
client_config_address=""
for i in $(seq 1 $SERVER_WORKERS); do
    WORKER_ID=$(printf "%03d" $((START_NODE + i - 1)))
    pub="${WORKER_PREFIX}${WORKER_ID}"
    port=38001
    clientport=38003
    for j in $(seq 1 $NODES_PER_WORKER); do
        id=$((NODES_PER_WORKER * (i - 1) + j))
        if (( id > NUM_NODES )); then
            break
        fi
        
        (( port += 10 * ((id - 1) % NODES_PER_WORKER)))
        [ "${ALTUNWAN:-0}" = 1 ] && pub="altunwan-${WORKER_ID}-r${j}"
        address+="    - \"$pub:$port\"\\n"
        client_config_address+="    - \"$pub:$port\"\\n"
    done
done
echo "Server config address: "
printf "%b" "$address"
echo "Client config address: "
printf "%b" "$client_config_address"


echo "#################### Generating Server configuration files:"

for i in $(seq 1 $SERVER_WORKERS); do
    WORKER_ID=$(printf "%03d" $((START_NODE + i - 1)))
    pub="${WORKER_PREFIX}${WORKER_ID}"
    port=38001
    for j in $(seq 1 $NODES_PER_WORKER); do
        id=$((NODES_PER_WORKER * (i - 1) + j))
        if (( id > NUM_NODES )); then
            break
        fi
        p="server${id}"
        sid=$((id - 1))
        (( port += 10 * ((id - 1) % NODES_PER_WORKER)))
        certpath="${REMOTE_DIR}/mir${id}"
        cat ${SCRIPT_DIR}/config-file-templates/server-config.yml | sed "s|SERVER_ID|$sid|g; s|CERT_PATH|$certpath|g; s|LISTEN_ENDPOINT|0.0.0.0:$port|g; s|SERVER_HOSTNAME|$p|g; s|NUM_NODES|$NUM_NODES|g; s|NUM_LEADERS|$NUM_LEADERS|g; s|NUM_FAULTS|$F|g; s|BATCH_SIZE_BYTES|$BATCH_SIZE_BYTES|g; s|EPOCH|$epoch|g; s|WATERMARK|$watermark|g; s|BATCHTIMEOUT|$timeout|g; s|SERVER_LOG_LEVEL|${MIR_SERVER_LOG:-critical}|g"  > ${SCRIPT_DIR}/temp/config_$p.yml
        
        echo "  certFiles:" >> ${SCRIPT_DIR}/temp/config_$p.yml
        for s in $servers; do
            echo "    - \"$certpath/config/certs/ecdsa/$s.pem\"" >> ${SCRIPT_DIR}/temp/config_$p.yml
        done
        echo "  addresses:" >> ${SCRIPT_DIR}/temp/config_$p.yml
        printf "%b" "$address" >> ${SCRIPT_DIR}/temp/config_$p.yml
    done
done

echo "#################### Generating Client configuration files:"

for i in $(seq 1 $CLIENT_WORKERS); do
    WORKER_ID=$(printf "%03d" $((START_NODE + SERVER_WORKERS + i - 1)))
    pub="${WORKER_PREFIX}${WORKER_ID}"
    
    for j in $(seq 1 $CLIENTS_PER_WORKER); do
        id=$((CLIENTS_PER_WORKER * (i - 1) + j))
        if (( id > NUM_CLIENTS )); then
            break
        fi
        p="client${id}"
        cid=$((id - 1))
        capath="${REMOTE_DIR}/mir${id}/config/certs/ecdsa/ca.pem"
        cat ${SCRIPT_DIR}/config-file-templates/client-config.yml | sed "s|CLIENT_ID|$cid|g; \
        s|CA_PATH|$capath|g; \
        s|NUM_NODES|$NUM_NODES|g; \
        s|NUM_LEADERS|$NUM_LEADERS|g; \
        s|NUM_FAULTS|$F|g; \
        s|NUM_CLIENTS|$NUM_CLIENTS|g; \
        s|CONCURRENCY|$CONCURRENCY|g; \
        s|RATE_LIMIT|$RATE_LIMIT|g; \
        s|PAYLOAD_SIZE|$PAYLOAD_SIZE|g; \
        s|BATCH_SIZE_BYTES|$BATCH_SIZE_BYTES|g; \
        s|REQUESTS|$REQUESTS|g; \
        s|CLIENT_RUN_TIME_MS|${MIR_WINDOW_MS:-10000}|g; \
        s|CLIENT_LOG_LEVEL|${MIR_CLIENT_LOG:-error}|g; \
        s|MIR_RECEIVERS_N|${MIR_RECEIVERS:-$NUM_NODES}|g; \
        s|MIR_DEDICATED_B|${MIR_DEDICATED:-false}|g; \
        s|EPOCH|$epoch|g" \
        "${SCRIPT_DIR}/config-file-templates/client-config.yml" \
        > "${SCRIPT_DIR}/temp/config_$p.yml"

        echo "  addresses:" >> ${SCRIPT_DIR}/temp/config_$p.yml
        printf "%b" "${client_config_address}" >> ${SCRIPT_DIR}/temp/config_$p.yml
    done
done

# Stage every host's files locally, then one ssh + one scp per host, all hosts
# in parallel.
echo "#################### Staging and copying files (one transfer per host, in parallel):"
STAGE=${SCRIPT_DIR}/temp/stage
mkdir -p $STAGE
stage_proc() { # kind id hostdir
    local kind=$1 id=$2 d=$3/mir$2
    mkdir -p $d/bin $d/config/certs/ecdsa $d/config/serverconfig $d/config/clientconfig
    cp ${SCRIPT_DIR}/temp/*.pem $d/config/certs/ecdsa/
    if [ $kind = server ]; then
        cp ${SCRIPT_DIR}/temp/server$id.key $d/config/certs/ecdsa/
        cp ${SCRIPT_DIR}/temp/config_server$id.yml $d/config/serverconfig/config.yml
        cp ${SCRIPT_DIR}/../bin/mir-server $d/bin/
        cp ${SCRIPT_DIR}/run-server.sh $d/
    else
        cp ${SCRIPT_DIR}/temp/config_client$id.yml $d/config/clientconfig/config.yml
        cp ${SCRIPT_DIR}/../bin/mir-client $d/bin/
        cp ${SCRIPT_DIR}/run-client.sh $d/
    fi
}
HOSTS=()
for i in $(seq 1 $SERVER_WORKERS); do
    pub="${WORKER_PREFIX}$(printf "%03d" $((START_NODE + i - 1)))"; HOSTS+=($pub); mkdir -p $STAGE/$pub
    for j in $(seq 1 $NODES_PER_WORKER); do
        id=$((NODES_PER_WORKER * (i - 1) + j)); (( id > NUM_NODES )) && break
        echo "server${id} on ${pub}"; stage_proc server $id $STAGE/$pub
    done
done
for i in $(seq 1 $CLIENT_WORKERS); do
    pub="${WORKER_PREFIX}$(printf "%03d" $((START_NODE + SERVER_WORKERS + i - 1)))"; HOSTS+=($pub); mkdir -p $STAGE/$pub
    for j in $(seq 1 $CLIENTS_PER_WORKER); do
        id=$((CLIENTS_PER_WORKER * (i - 1) + j)); (( id > NUM_CLIENTS )) && break
        echo "client${id} on ${pub}"; stage_proc client $id $STAGE/$pub
    done
done
pids=()
for pub in "${HOSTS[@]}"; do
    (
    # clear the node's mir work dir and stop old processes
    ssh $ssh_user@$pub $ssh_options "rm -rf ${REMOTE_DIR}/* ; mkdir -p ${REMOTE_DIR}" </dev/null
    scp -q $ssh_options ${SCRIPT_DIR}/stop.sh $ssh_user@$pub:${REMOTE_BASE}/ </dev/null
    ssh $ssh_user@$pub $ssh_options "cd ${REMOTE_BASE}/ && source stop.sh" </dev/null
    scp -q -r $ssh_options $STAGE/$pub/mir* $ssh_user@$pub:${REMOTE_DIR}/ </dev/null
    ) &
    pids+=($!)
done
for p in "${pids[@]}"; do wait $p; done
echo "#################### Copy done"
