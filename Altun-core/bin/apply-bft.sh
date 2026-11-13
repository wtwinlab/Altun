#!/bin/bash
# apply-bft.sh — run one HotStuff-family BFT protocol (chainedhotstuff | fasthotstuff | simplehotstuff).
#
# Usage: apply-bft.sh <alg> <concurrency> <replicas> <clients> <batchsize> <payloadsize> <ratelimit> [leader-rotation]
#
# bft-server (the HotStuff orchestrator) deploys one worker per cluster host
# listed in env.sh, starts the replicas and clients, and collects the
# measurements; bft-window turns them into benchmark.log.
set -x

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

. ${SCRIPT_DIR}/env.sh

timestamp=$(date +%s)
alg=${1:-"chainedhotstuff"}
concurrency=${2:-100000}
replicas=${3:-4}
clients=${4:-1}
batchsize=${5:-1}
payloadsize=${6:-0}
ratelimit=${7:-100000}
leaderrotation=${8:-"round-robin"}
# concurrency * clients > 3 * batchsize


configs="--consensus ${alg} \
--max-concurrent ${concurrency}  \
--replicas ${replicas} \
--clients ${clients} \
--batch-size ${batchsize} \
--payload-size ${payloadsize} \
--leader-rotation ${leaderrotation} "
if [ -z ${ratelimit} ]; then
     configs="${configs}"
else
    configs="${configs} --rate-limit ${ratelimit}"
fi

if [ ${local} = true ]; then
    configs="${configs}"
else
    exepath="${SCRIPT_DIR}/bft-server"
    sshconfigfile="${SCRIPT_DIR}/ssh_config"
    hosts=""
    SERVER_WORKERS=$((replicas / NODES_PER_WORKER + (replicas % NODES_PER_WORKER == 0 ? 0 : 1)))
    CLIENT_WORKERS=$((clients / CLIENTS_PER_WORKER + (clients % CLIENTS_PER_WORKER == 0 ? 0 : 1)))
    HOST_NUM=$((SERVER_WORKERS + CLIENT_WORKERS))
    # ssh configuration handed to the orchestrator: one entry per cluster host
    : > "$sshconfigfile"
    for i in $(seq 1 $HOST_NUM); do
        pub=$(host_of $i)
        printf 'Host %s\n\tUser %s\n\tIdentityFile %s\n' "$pub" "$ssh_user" "$private_key_file" >> "$sshconfigfile"
        if [ $i -eq 1 ]; then
            hosts=$pub
        else
            hosts=${hosts}","$pub
        fi
    done

    if [ "${ALTUNWAN:-0}" = 1 ]; then
        # Docker/WAN: one worker per container. Replica id k -> slot r<j> of replica host i
        # (k = NODES_PER_WORKER*(i-1)+j, i.e. ids 1-4 on the first host, 5-8 on the second, as
        # for the CFT protocols); client id k -> slot c<m> of client host i. The ssh
        # configuration maps each container alias to its host.
        sshconfigfile="${SCRIPT_DIR}/ssh_config_altunwan"
        : > "$sshconfigfile"
        hosts=""; assign=""
        add_alias() { # alias host
            printf 'Host %s\n\tHostName %s\n\tUser %s\n\tIdentityFile %s\n' "$1" "$2" "$ssh_user" "$private_key_file" >> "$sshconfigfile"
            hosts="${hosts:+$hosts,}$1"
        }
        for k in $(seq 1 $replicas); do
            i=$(( (k - 1) / NODES_PER_WORKER + 1 )); j=$(( (k - 1) % NODES_PER_WORKER + 1 ))
            w=$(num_of $i)
            add_alias "altunwan-$w-r$j" "$(host_of $i)"; assign="${assign:+$assign,}altunwan-$w-r$j:r$k"
        done
        for k in $(seq 1 $clients); do
            i=$(( (k - 1) / CLIENTS_PER_WORKER + 1 )); m=$(( (k - 1) % CLIENTS_PER_WORKER + 1 ))
            w=$(num_of $((SERVER_WORKERS + i)))
            add_alias "altunwan-$w-c$m" "$(host_of $((SERVER_WORKERS + i)))"; assign="${assign:+$assign,}altunwan-$w-c$m:c$k"
        done
        # stage the binary once per physical host (bind-mounted into the containers)
        pids=()
        for i in $(seq 1 $HOST_NUM); do
            pub=$(host_of $i)
            # a worker left over from an earlier step would keep its fixed port: stop it first
            { ssh $ssh_options $ssh_user@$pub "pkill -9 -u \$(id -u) -x bft-server; mkdir -p ${ALTUNWAN_DIR}/bin; true" </dev/null
              rsync -a -e "ssh $ssh_options" "${exepath}" "$ssh_user@$pub:${ALTUNWAN_DIR}/bin/bft-server" </dev/null; } &
            pids+=($!)
        done
        for p in "${pids[@]}"; do wait $p; done
        export HS_DOCKER=1 HS_DOCKER_EXE=${ALTUNWAN_DIR}/bin/bft-server HS_DOCKER_TMP=${REMOTE_DIR} HS_DOCKER_ASSIGN="$assign" HS_DOCKER_IPS="${SCRIPT_DIR}/altunwan-ips.txt" HS_DOCKER_CPREFIX="${ALTUNWAN_CPREFIX}"
    fi
    echo "hosts: [$hosts]"
    configs="${configs} --hosts ${hosts}"
    configs="${configs} --exe ${exepath}"
    configs="${configs} --ssh-config ${sshconfigfile}"
fi

echo "configs: ${configs}"

# All paths are relative to SCRIPT_DIR: the script can be called from any
# working directory, and bft-server / bft-window live next to it.
rm -rf "${SCRIPT_DIR}/bft-performance.json"
rm -rf "${SCRIPT_DIR}/output"
rm -rf "${SCRIPT_DIR}/benchmark.log"


echo "ok: apply new config: ${alg}"
T_NEW_START=$(date +%s%3N)
cd "${SCRIPT_DIR}"
# Timed closed loop (same semantics as the CFT benchmark): every client keeps at
# most ${concurrency} requests outstanding, is capped at ${ratelimit} req/s, sends
# for HS_WINDOW_MS (default 10 s) and reports the requests completed (f+1 replies)
# inside that window and their mean latency. TPS = sum_i completed_i / window_i.
# HS_WINDOW_MS is forwarded to the remote workers (faultinject.EnvPrefix).
export HS_WINDOW_MS="${HS_WINDOW_MS:-10000}"
win_s=$(( (HS_WINDOW_MS + 999) / 1000 ))
# The orchestrator sleeps --duration after the last client has started, so every
# client's window has closed by then. The client timeout is longer than the
# window, so no request is abandoned (an abandoned request would free its slot
# and turn the closed loop into an open one).
./bft-server run ${configs} --client-timeout "${HS_CLIENT_TIMEOUT:-60s}" \
    --measurement-interval 5s --duration "$((win_s + 1))s" &> "${SCRIPT_DIR}/bft-run.log"
echo "ok: benchmark run (rc=$?)"

"${SCRIPT_DIR}/bft-window" -clients "${clients}" -details "${SCRIPT_DIR}/bft-performance.json" \
    "${SCRIPT_DIR}"/output/*/measurements.json > "${SCRIPT_DIR}/benchmark.log"
T_BENCH_END=$(date +%s%3N)
cat > "${SCRIPT_DIR}/switch_timing.json" <<EOF
{"t_drain_start":${T_NEW_START},"t_drain_end":${T_NEW_START},"t_bench_end":${T_BENCH_END},"protocol":"${alg}"}
EOF
cat "${SCRIPT_DIR}/benchmark.log"
