#!/bin/bash -e

# set -x

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

getIP() {
    if [ "$local" = "true" ]; then
        echo 127.0.0.1
    else
	    grep -w $1 ${SCRIPT_DIR}/cluster-instance.info | awk '{ print $2}'
	fi
}

getPrivIP(){
    if [ "$local" = "true" ]; then
        echo 127.0.0.1
    else
	    grep -w $1 ${SCRIPT_DIR}/cluster-instance.info | awk '{ print $3}'
	fi
}

genCert(){
    openssl ecparam -name secp521r1 -genkey -param_enc named_curve -out $1.key
    openssl ecparam -genkey -name prime256v1 -out $1.key
    openssl req -new -SHA384 -key $1.key -nodes -out $1.csr -config openssl.conf
    openssl x509 -req -SHA384 -days 3650 -in $1.csr -CA ca.pem -CAkey ca.key -CAcreateserial -out $1.pem -extfile openssl.conf -extensions req_ext -CAcreateserial
}


. ${SCRIPT_DIR}/../../bin/env.sh

# Host naming and remote paths come from bin/env.sh:
#   WORKER_PREFIX  host name prefix, hosts are ${WORKER_PREFIX}<3-digit index> (default worker-)
#   REMOTE_DIR     per-host working directory that receives the mir<id> directories
#   REMOTE_BASE    parent of REMOTE_DIR, where stop.sh is placed (default: dirname of REMOTE_DIR)
WORKER_PREFIX=${WORKER_PREFIX:-worker-}
REMOTE_BASE=${REMOTE_BASE:-$(dirname "$REMOTE_DIR")}

if [ "${local}" = true ]; then
  NUM_NODES=${1:-4}
  NUM_CLIENTS=${2:-1}
  CONCURRENCY=${3:-1}
  RATE_LIMIT=${4:-1000}
  PAYLOAD_SIZE=${5:-500}
  BATCH_SIZE=${6:-1}
  
  N=$NUM_NODES
  F=$(((N-1)/3))
  C=$NUM_CLIENTS
  servers=""
  clients=""
  for i in $(seq 1 $N); do
    servers+="server$i "
  done
  for i in $(seq 1 $C); do
      clients+="client$i "
  done
else
#   local=false # setting env.sh
  NUM_NODES=${1:-4}
  NUM_CLIENTS=${2:-1}
  CONCURRENCY=${3:-1}
  RATE_LIMIT=${4:-1000}
  PAYLOAD_SIZE=${5:-500}
  BATCH_SIZE_BYTES=${6:-2000000}
  echo '' > ${SCRIPT_DIR}/cluster-instance.info

  SERVER_WORKERS=$((NUM_NODES / NODES_PER_WORKER + (NUM_NODES % NODES_PER_WORKER == 0 ? 0 : 1)))
  CLIENT_WORKERS=$((NUM_CLIENTS / NODES_PER_WORKER + (NUM_CLIENTS % NODES_PER_WORKER == 0 ? 0 : 1)))
  echo "Use SERVER_WORKERS size: $SERVER_WORKERS*$NODES_PER_WORKER"
  echo "Use CLIENT_WORKER size = $CLIENT_WORKERS*$NODES_PER_WORKER"

  for i in $(seq 1 $SERVER_WORKERS); do
    WORKER_ID=$(printf "%03d" $((START_NODE + i - 1)))
    pub="${WORKER_PREFIX}${WORKER_ID}"
    for j in $(seq 1 $NODES_PER_WORKER); do
        id=$(( NODES_PER_WORKER * (i - 1) + j ))
        if (( id > NUM_NODES )); then
            break
        fi
        echo "server${id} ${pub} ${pub}" >> ${SCRIPT_DIR}/cluster-instance.info
    done
  done

  for i in $(seq 1 $CLIENT_WORKERS); do
    WORKER_ID=$(printf "%03d" $((START_NODE + SERVER_WORKERS + i - 1)))
    pub="${WORKER_PREFIX}${WORKER_ID}"
    for j in $(seq 1 $NODES_PER_WORKER); do
        id=$(( NODES_PER_WORKER * (i - 1) + j ))
        if (( id > NUM_CLIENTS )); then
            break
        fi
        echo "client${id} ${pub} ${pub}" >> ${SCRIPT_DIR}/cluster-instance.info
    done
  done


  servers=$(grep server ${SCRIPT_DIR}/cluster-instance.info | awk '{ print $1}')
  clients=$(grep client ${SCRIPT_DIR}/cluster-instance.info | awk '{ print $1}')
  N=$(grep -c server ${SCRIPT_DIR}/cluster-instance.info)
  F=$(((N-1)/3))
  C=$(grep -c client ${SCRIPT_DIR}/cluster-instance.info)
fi

echo "#######${servers}, ${clients}########"
cat ${SCRIPT_DIR}/cluster-instance.info
config_only=false

echo "Removing old configuration"
rm -rf ${SCRIPT_DIR}/temp
mkdir -p ${SCRIPT_DIR}/temp

if [ "$local" = "true" ]; then
  if [ "$config_only" = "false" ]; then
    rm -rf ${SCRIPT_DIR}/config/certs/ecdsa/*
  fi
  rm -rf ${SCRIPT_DIR}/config/serverconfig/*
  rm -rf ${SCRIPT_DIR}/config/clientconfig/*
  mkdir -p ${SCRIPT_DIR}/config/certs/ecdsa/
  mkdir -p ${SCRIPT_DIR}/config/serverconfig/
  mkdir -p ${SCRIPT_DIR}/config/clientconfig/
else
  for p in $servers $clients; do
      pub=$(getIP $p)
    #   if [ "$config_only" = "false" ]; then
    #     ssh $ssh_user@$pub $ssh_options "rm -rf ${REMOTE_DIR}/mir/config/certs/ecdsa/*"
    #   fi
    #   ssh $ssh_user@$pub $ssh_options "rm -rf ${REMOTE_DIR}/mir/config/serverconfig/*"
    #   ssh $ssh_user@$pub $ssh_options "rm -rf ${REMOTE_DIR}/mir/config/clientconfig/*"
      ssh $ssh_user@$pub $ssh_options "rm -rf ${REMOTE_DIR}/mir/*"
      ssh $ssh_user@$pub $ssh_options "mkdir -p ${REMOTE_DIR}/mir/bin/"
      ssh $ssh_user@$pub $ssh_options "mkdir -p ${REMOTE_DIR}/mir/config/certs/ecdsa/"
      ssh $ssh_user@$pub $ssh_options "mkdir -p ${REMOTE_DIR}/mir/config/serverconfig/"
      ssh $ssh_user@$pub $ssh_options "mkdir -p ${REMOTE_DIR}/mir/config/clientconfig/"
  done
fi

if [ "$config_only" = "false" ]; then
    cd ${SCRIPT_DIR}/temp
    ${SCRIPT_DIR}/generate-ca.sh -f
    cd ..

    echo "Generating Certificates"
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

    echo "Copying Certificates"
    if [ "$local" = "true" ]; then
        cp ${SCRIPT_DIR}/temp/* ${SCRIPT_DIR}/config/certs/ecdsa/ 
    else
        for p in $servers $clients; do
            pub=$(getIP $p)
            scp $ssh_options ${SCRIPT_DIR}/temp/*.pem $ssh_user@$pub:${REMOTE_DIR}/mir/config/certs/ecdsa/
        done
        for p in $servers; do
            pub=$(getIP $p)
            scp $ssh_options ${SCRIPT_DIR}/temp/$p.key $ssh_user@$pub:${REMOTE_DIR}/mir/config/certs/ecdsa/
        done
    fi
fi

echo "#################### Generating configuration files:"
port=38001
id=0
thres=16
if [ $N -ge $thres ]; then
    watermark=$(( N*2 ))
    epoch=$(( N*16 ))
    timeout=$(( N*32*1000000 ))
else
    watermark=32
    epoch=256
    timeout=500000000
fi

certpath=""
if [ "$local" = "true" ]; then
    certpath="${SCRIPT_DIR}"
    for p in $servers; do
        ip=$(getPrivIP $p)
        cat ${SCRIPT_DIR}/config-file-templates/server-config.yml | sed "s|SERVER_ID|$id|g; s|CERT_PATH|$certpath|g; s|LISTEN_ENDPOINT|0.0.0.0:$port|g; s|SERVER_HOSTNAME|$p|g; s|NUM_NODES|$N|g; s|NUM_FAULTS|$F|g; s|BATCH_SIZE_BYTES|$BATCH_SIZE_BYTES|g; s|EPOCH|$epoch|g; s|WATERMARK|$watermark|g; s|BATCHTIMEOUT|$timeout|g"  > ${SCRIPT_DIR}/temp/config_$p.yml
        (( id += 1 ))
        (( port += 10 ))
    done
else
    certpath="${REMOTE_DIR}/mir"
    for p in $servers; do
        ip=$(getPrivIP $p)
        cat ${SCRIPT_DIR}/config-file-templates/server-config.yml | sed "s|SERVER_ID|$id|g; s|CERT_PATH|$certpath|g; s|LISTEN_ENDPOINT|0.0.0.0:$port|g; s|SERVER_HOSTNAME|$p|g; s|NUM_NODES|$N|g; s|NUM_FAULTS|$F|g; s|BATCH_SIZE_BYTES|$BATCH_SIZE_BYTES|g; s|EPOCH|$epoch|g; s|WATERMARK|$watermark|g; s|BATCHTIMEOUT|$timeout|g"  > ${SCRIPT_DIR}/temp/config_$p.yml
        (( port += (id % NODES_PER_WORKER)))
        (( id += 1 ))
    done
fi

capath=""
if [ "$local" = "true" ]; then
    capath="${SCRIPT_DIR}/config/certs/ecdsa/ca.pem"
else
    capath="${REMOTE_DIR}/mir/config/certs/ecdsa/ca.pem"
fi

id=0
for p in $clients; do
    cat ${SCRIPT_DIR}/config-file-templates/client-config.yml | sed "s|CLIENT_ID|$id|g; \
     s|CA_PATH|$capath|g; \
     s|NUM_NODES|$N|g; \
     s|NUM_FAULTS|$F|g; \
     s|NUM_CLIENTS|$C|g; \
     s|CONCURRENCY|$CONCURRENCY|g; \
     s|RATE_LIMIT|$RATE_LIMIT|g; \
     s|PAYLOAD_SIZE|$PAYLOAD_SIZE|g; \
     s|BATCH_SIZE_BYTES|$BATCH_SIZE_BYTES|g; \
     s|EPOCH|$epoch|g" \
     "${SCRIPT_DIR}/config-file-templates/client-config.yml" \
     > "${SCRIPT_DIR}/temp/config_$p.yml"
    #  sed "s/CLIENT_ID/$id/ ; s/CA_PATH/$capath/ ; s/NUM_NODES/$N/ ; s/NUM_FAULTS/$F/ ; s/NUM_CLIENTS/$C/ ; s/CONCURRENCY/$CONCURRENCY/ ; s/RATE_LIMIT/$RATE_LIMIT/ ; s/PAYLOAD_SIZE/$PAYLOAD_SIZE/ ; s/BATCH_SIZE_BYTES/$BATCH_SIZE_BYTES/ ; s/EPOCH/$epoch/"> ${SCRIPT_DIR}/temp/config_$p.yml
    (( id += 1 ))
done

for p in $servers; do
    echo "  certFiles:" >> ${SCRIPT_DIR}/temp/config_$p.yml
    for s in $servers; do
        echo "    - \"$certpath/config/certs/ecdsa/$s.pem\"" >> ${SCRIPT_DIR}/temp/config_$p.yml
    done
    echo "  addresses:" >> ${SCRIPT_DIR}/temp/config_$p.yml
    port=38001
    for s in $servers; do
        ip=$(getPrivIP $s)
        if [ "$local" = "true" ]; then
            ip="127.0.0.1"
        fi
        echo "    - \"$ip:$port\"" >> ${SCRIPT_DIR}/temp/config_$p.yml
        if [ "$local" = "true" ]; then
            (( port += 10 ))
        fi
    done
done

for p in $clients; do
    echo "  addresses:" >> ${SCRIPT_DIR}/temp/config_$p.yml
    port=38001
    (( port += 2 ))
    for s in $servers; do
        ip=$(getIP $s)
        if [ "$local" = "true" ]; then
            ip="127.0.0.1"
        fi
        echo "    - \"$ip:$port\"" >> ${SCRIPT_DIR}/temp/config_$p.yml
        if [ "$local" = "true" ]; then
            (( port += 10 ))
        fi
    done
done

echo "Copying configuration files"
if [ "$local" = "true" ]; then
    for p in $servers; do
        echo ""
        cp ${SCRIPT_DIR}/temp/config_$p.yml ${SCRIPT_DIR}/config/serverconfig/
    done
    for p in $clients; do
       cp ${SCRIPT_DIR}/temp/config_$p.yml ${SCRIPT_DIR}/config/clientconfig/
    done
else
    for p in $servers; do
       pub=$(getIP $p)
       scp $ssh_options ${SCRIPT_DIR}/temp/config_$p.yml $ssh_user@$pub:${REMOTE_DIR}/mir/config/serverconfig/config.yml
    done
    for p in $clients; do
       pub=$(getIP $p)
       scp $ssh_options ${SCRIPT_DIR}/temp/config_$p.yml $ssh_user@$pub:${REMOTE_DIR}/mir/config/clientconfig/config.yml
    done
fi

# rm -rf ${SCRIPT_DIR}/temp