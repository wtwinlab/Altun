#!/usr/bin/env bash
# apply-cft.sh — deploy and benchmark one CFT protocol (paxos | epaxos | vpaxos | wpaxos).
#
# Usage: apply-cft.sh [benchmark.json]      (default: config_RL_default.json)
#
# The benchmark description (algorithm, nodesize, clients, concurrency, ratelimit,
# batchsize, payloadsize, requests, writeratio, distribution) is turned into a
# replica configuration (config_cft.json), the replica and client binaries are
# copied to the cluster hosts named in env.sh, the benchmark runs for its window
# and the aggregated throughput and latency are written to benchmark.log.
#
# Optional environment:
#   FORENSICS_MODE   off | log | full   forensics log collection (full also runs forensic-verify)
#   FORENSICS_LOG    path of the forensics log (default: a file under FORENSICS_DIR)
#   INJECT_EQUIVOCATION, SLOW_PROPOSAL_MS, SLOW_PROPOSAL_IDS, ABSENT_IDS
#                    fault injection, forwarded to every replica (see consensus/cft/fault.go)
#   WAN_CLIENT_REGION, WAN_KEYS, WAN_CONFLICTS
#                    client placement and key access pattern of the Docker/WAN emulation
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

. ${SCRIPT_DIR}/env.sh

# -----------------------------------------------------------------------
# Forensics log collection
#   off   - disabled
#   log   - every replica appends to FORENSICS_LOG; the verifier is not run
#   full  - log, then run forensic-verify after the benchmark
# FORENSICS_LOG must be writable from every cluster host.
# -----------------------------------------------------------------------
FORENSICS_MODE="${FORENSICS_MODE:-off}"
if [ -z "${FORENSICS_LOG:-}" ]; then
    FORENSICS_LOG="${FORENSICS_DIR}/forensics_$(hostname)_$$.jsonl"
fi
if [ "$FORENSICS_MODE" != "off" ]; then
    mkdir -p "$(dirname "$FORENSICS_LOG")" 2>/dev/null || true
    rm -f "$FORENSICS_LOG"
    touch "$FORENSICS_LOG" 2>/dev/null || {
        echo "warn: cannot write to $FORENSICS_LOG, forcing FORENSICS_MODE=off" >&2
        FORENSICS_MODE=off
        FORENSICS_LOG=""
    }
else
    # An empty FORENSICS_LOG disables the auditor in every replica.
    FORENSICS_LOG=""
fi

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
if [ "$alg" = "vpaxos" ] && [ "$replicas" = 4 ]; then
    NODES_PER_WORKER=2
fi
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
echo "replicas: ${replicas} ${CONFIG_FILE}"

update_node_address "${CONFIG_FILE}" ${replicas} ${NODES_PER_WORKER} ${START_NODE}

RDIR="${REMOTE_DIR:?}"

# ── Phase 1: stop the previous instance ─────────────────────────────
T_SWITCH_START=$(date +%s%3N)
ppkill cft-server
ppkill cft-client
T_KILL_DONE=$(date +%s%3N)

rm -f ${SCRIPT_DIR}/cft-server.*.log
rm -f ${SCRIPT_DIR}/cft-client.*.log
rm -f ${SCRIPT_DIR}/benchmark.log
rm -f ${SCRIPT_DIR}/history.csv
rm -f ${SCRIPT_DIR}/latency
T_DRAIN_START=${T_SWITCH_START}


echo $CONFIG_FILE


SERVER_WORKERS=$((replicas / NODES_PER_WORKER + (replicas % NODES_PER_WORKER == 0 ? 0 : 1)))

# ── Phase 2: copy configuration and binary to the replica hosts ─────
echo "ok: new config updated: ${alg}"
echo "Start all server nodes: $SERVER_WORKERS*$NODES_PER_WORKER"
echo "Copying server configuration files"
for i in $(seq 1 $SERVER_WORKERS); do
    pub=$(host_of $i)
    ssh $ssh_user@$pub $ssh_options "bash -s" < ${SCRIPT_DIR}/stop.sh
    ssh $ssh_user@$pub $ssh_options "mkdir -p ${RDIR} && rm -rf ${RDIR}/* && mkdir -p ${RDIR}/cft/ "
    scp $ssh_options ${SCRIPT_DIR}/config_cft.json $ssh_user@$pub:${RDIR}/cft/config_cft.json >/dev/null
    scp $ssh_options ${SCRIPT_DIR}/cft-server $ssh_user@$pub:${RDIR}/cft/ >/dev/null
done
T_DEPLOY_DONE=$(date +%s%3N)

# ── Phase 3: start the replicas ─────────────────────────────────────
echo "Start servers with configuration"
FAULT_ENV="FORENSICS_LOG='${FORENSICS_LOG}' INJECT_EQUIVOCATION='${INJECT_EQUIVOCATION:-}' AUDITOR_ENABLED='${AUDITOR_ENABLED:-}' BENV_PATH='${BENV_PATH:-}' SLOW_PROPOSAL_MS='${SLOW_PROPOSAL_MS:-}' SLOW_PROPOSAL_IDS='${SLOW_PROPOSAL_IDS:-}' ABSENT_IDS='${ABSENT_IDS:-}'"
for i in $(seq 1 $SERVER_WORKERS); do
    pub=$(host_of $i)
    for j in $(seq 1 $NODES_PER_WORKER); do
        id=$(( NODES_PER_WORKER * (i - 1) + j ))
        if (( id > replicas )); then
            break
        fi
        if [ "${ALTUNWAN:-0}" = 1 ]; then
            # Docker/WAN: the replica runs in its own container ${ALTUNWAN_CPREFIX}r$j (same files, bind-mounted)
            ssh $ssh_user@$pub $ssh_options "docker exec -d -w ${RDIR}/cft ${ALTUNWAN_CPREFIX}r$j sh -c \"${FAULT_ENV} exec ./cft-server -id $i.$j -algorithm ${alg} -config config_cft.json > server$j.log 2>&1\"" &
        else
        ssh $ssh_user@$pub $ssh_options "cd ${RDIR}/cft/ && ${FAULT_ENV} nohup ./cft-server -id $i.$j -algorithm ${alg} -config config_cft.json > server$j.log 2>&1 & " &
        fi
    done
done

# Wait until every replica's TCP port is listening on its host (ports come
# from config_cft.json), at most 30 s. The launching ssh sessions stay open
# as long as the remote server runs, so they are not waited on.
for i in $(seq 1 $SERVER_WORKERS); do
    pub=$(host_of $i)
    ports=$(jq -r ".address | to_entries[] | select(.key | startswith(\"$i.\")) | .value" ${CONFIG_FILE} | sed 's/.*://' | tr '\n' ' ')
    for t in $(seq 1 30); do
        if [ "${ALTUNWAN:-0}" = 1 ]; then
            # Docker/WAN: ask each container; the host's ss would see docker-proxy's published ports
            listening=$(ssh $ssh_options $ssh_user@$pub "for c in \$(docker ps --format '{{.Names}}' --filter name=^${ALTUNWAN_CPREFIX}r); do docker exec \$c ss -ltnH; done 2>/dev/null | awk '{print \$4}' | sed 's/.*://'")
        else
        listening=$(ssh $ssh_options $ssh_user@$pub "ss -ltnH 2>/dev/null | awk '{print \$4}' | sed 's/.*://'")
        fi
        up=1
        for port in $ports; do
            echo "$listening" | grep -qx "$port" || { up=0; break; }
        done
        [ $up = 1 ] && break
        sleep 1
    done
done
T_SERVERS_READY=$(date +%s%3N)

# ── Phase 4: start the clients ──────────────────────────────────────
echo "Start clients with configuration: $clients"
CLIENT_WORKERS=$((clients / CLIENTS_PER_WORKER + (clients % CLIENTS_PER_WORKER == 0 ? 0 : 1)))
echo "Use CLIENT_WORKER size = $CLIENT_WORKERS*$CLIENTS_PER_WORKER in config file"

for i in $(seq 1 $CLIENT_WORKERS); do
    pub=$(host_of $((SERVER_WORKERS + i)))
    ssh $ssh_user@$pub $ssh_options "bash -s" < ${SCRIPT_DIR}/stop.sh
    ssh $ssh_user@$pub $ssh_options "mkdir -p ${RDIR} && rm -rf ${RDIR}/* && mkdir -p ${RDIR}/cft/"
    scp $ssh_options ${SCRIPT_DIR}/config_cft.json $ssh_user@$pub:${RDIR}/cft/config_cft.json >/dev/null
    scp $ssh_options ${SCRIPT_DIR}/cft-client $ssh_user@$pub:${RDIR}/cft/ >/dev/null
done

# Docker/WAN network settings of this step (unset = split clients, one shared key space):
#   WAN_CLIENT_REGION=split|A|B  region of the client slots (altunwan.sh netem): split = slots 1..CLIENTS_PER_WORKER/2
#                                in A, the rest in B; A / B = every client in that region. Clients target replicas of
#                                their own region (round robin over the region's non-absent replicas); paxos clients
#                                target the leader 1.1 (region A) in every mode.
#   WAN_KEYS=global|local|conflict  key access pattern of the benchmark (config "benchmark"): global = uniform over
#                                one shared key space [0, K); local = uniform over a key range of the client's own region
#                                (A: [0, K), B: [K, 2K)); conflict = distribution "conflict" over the region's range with
#                                WAN_CONFLICTS % (default 50) of the requests on the one shared key 0.
WAN_CREG=${WAN_CLIENT_REGION:-split}
WAN_KEYMODE=${WAN_KEYS:-global}
client_region() { # client slot j -> A | B
    if [ "$WAN_CREG" = A ] || [ "$WAN_CREG" = B ]; then echo $WAN_CREG
    elif [ "$1" -le $((CLIENTS_PER_WORKER / 2)) ]; then echo A; else echo B; fi
}
if [ "$WAN_KEYMODE" = local ] || [ "$WAN_KEYMODE" = conflict ]; then
    KEYSPACE=$(jq '.benchmark.K' ${CONFIG_FILE})
    for R in A B; do
        kmin=0; [ $R = B ] && kmin=$KEYSPACE
        if [ "$WAN_KEYMODE" = conflict ]; then
            jq --argjson m $kmin --argjson c ${WAN_CONFLICTS:-50} '.benchmark.Min = $m | .benchmark.Distribution = "conflict" | .benchmark.Conflicts = $c' ${CONFIG_FILE} > ${SCRIPT_DIR}/config_cft_keys$R.json
        else
            jq --argjson m $kmin '.benchmark.Min = $m | .benchmark.Distribution = "uniform"' ${CONFIG_FILE} > ${SCRIPT_DIR}/config_cft_keys$R.json
        fi
    done
    for i in $(seq 1 $CLIENT_WORKERS); do
        pub=$(host_of $((SERVER_WORKERS + i)))
        scp $ssh_options ${SCRIPT_DIR}/config_cft_keysA.json ${SCRIPT_DIR}/config_cft_keysB.json $ssh_user@$pub:${RDIR}/cft/ >/dev/null
    done
    echo "keys: $WAN_KEYMODE (K=$KEYSPACE per region${WAN_CONFLICTS:+, conflicts $WAN_CONFLICTS %})"
elif [ "$WAN_KEYMODE" != global ]; then
    echo "Error: WAN_KEYS=$WAN_KEYMODE (global|local|conflict)"; exit 1
fi

CLIENT_TARGETS=$(jq -r '.address | keys[]' ${CONFIG_FILE} | sort -V | grep -vxF -f <(echo "${ABSENT_IDS:-}" | tr ',' '\n' | sed '/^$/d'; echo "__none__"))
# Paxos is single-leader and a non-leader that receives a request starts a new
# ballot, so spreading its clients would make replicas compete for leadership:
# all Paxos clients target the leader 1.1.
[ "${alg}" = "paxos" ] && CLIENT_TARGETS="1.1"
# Replica zones of each region (as altunwan.sh netem: zones 1..ceil(SW/2) = A, the rest B)
WAN_HALF=$(( (SERVER_WORKERS + 1) / 2 ))
region_targets() { # A | B -> the non-absent targets in that region (all targets if none)
    local t=$(echo "$CLIENT_TARGETS" | awk -F. -v h=$WAN_HALF -v r=$1 '(r == "A" && $1 <= h) || (r == "B" && $1 > h)')
    [ -n "$t" ] && echo "$t" || echo "$CLIENT_TARGETS"
}
if [ "$WAN_CREG" != split ] && [ "$WAN_CREG" != A ] && [ "$WAN_CREG" != B ]; then
    echo "Error: WAN_CLIENT_REGION=$WAN_CREG (split|A|B)"; exit 1
elif [ "$WAN_CREG" != split ] && [ "${alg}" != "paxos" ]; then
    CLIENT_TARGETS=$(region_targets $WAN_CREG)
fi
CLIENT_TARGET_N=$(echo "$CLIENT_TARGETS" | wc -l)
echo "client targets: $(echo $CLIENT_TARGETS)"
for i in $(seq 1 $CLIENT_WORKERS); do
    pub=$(host_of $((SERVER_WORKERS + i)))
    for j in $(seq 1 $CLIENTS_PER_WORKER); do
        id=$(( CLIENTS_PER_WORKER * (i - 1) + j ))
        if (( id > clients )); then
            break
        fi
        if [ "$WAN_KEYMODE" = global ]; then
        ssh $ssh_user@$pub $ssh_options "cd ${RDIR}/ && cp -r cft cft$j"
        else
        ssh $ssh_user@$pub $ssh_options "cd ${RDIR}/ && cp -r cft cft$j && cp cft$j/config_cft_keys$(client_region $j).json cft$j/config_cft.json"
        fi
        # CFT_PAYLOAD_SIZE makes the client write values of `payloadsize` bytes.
        # Clients are spread round-robin over the replicas that are not absent.
        target=$(echo "$CLIENT_TARGETS" | sed -n "$(( (id - 1) % CLIENT_TARGET_N + 1 ))p")
        if [ "${ALTUNWAN:-0}" = 1 ]; then
            # Docker/WAN: client slot j of this host runs in container ${ALTUNWAN_CPREFIX}c$j
            ssh $ssh_user@$pub $ssh_options "docker exec -d -w ${RDIR}/cft$j ${ALTUNWAN_CPREFIX}c$j sh -c \"CFT_PAYLOAD_SIZE='${payloadsize}' exec ./cft-client -id ${target} -algorithm ${alg} -config config_cft.json > client$id.log 2>&1\"" &
        else
        ssh $ssh_user@$pub $ssh_options "cd ${RDIR}/cft$j/ && CFT_PAYLOAD_SIZE='${payloadsize}' nohup ./cft-client -id ${target} -algorithm ${alg} -config config_cft.json > client$id.log 2>&1 & " &
        fi
    done
done

# Wait until the clients are running on each client host (at most 20 s).
for i in $(seq 1 $CLIENT_WORKERS); do
    pub=$(host_of $((SERVER_WORKERS + i)))
    want=$(( clients - CLIENTS_PER_WORKER * (i - 1) )); [ $want -gt $CLIENTS_PER_WORKER ] && want=$CLIENTS_PER_WORKER
    for t in $(seq 1 20); do
        running=$(ssh $ssh_options $ssh_user@$pub "pgrep -c -x cft-client || true" 2>/dev/null | tail -1); running=${running:-0}
        [ "${running:-0}" -ge "$want" ] && break
        # a short client run may already have finished: its log exists
        ssh $ssh_options $ssh_user@$pub "ls ${RDIR}/cft*/cft-client.*.log" >/dev/null 2>&1 && break
        sleep 1
    done
done
T_CLIENTS_READY=$(date +%s%3N)

# ── Phase 5: wait for the benchmark to complete ─────────────────────
echo "benchmarking... $CLIENT_WORKERS"
for i in $(seq 1 $CLIENT_WORKERS); do
    pub=$(host_of $((SERVER_WORKERS + i)))
    while ssh $ssh_options $ssh_user@$pub "pgrep -x cft-client" >/dev/null 2>&1; do
        sleep 1
        echo "Waiting for cft-client process to finish on $pub..."
    done
    if [ "${ALTUNWAN:-0}" = 1 ] && ssh $ssh_options $ssh_user@$pub "ls ${RDIR}/cft*/cft-client.*.log" >/dev/null 2>&1; then
        # Docker/WAN: each client has its own pid namespace, so the pid-named logs
        # (cft-client.<pid>.log) of two clients can collide: fetch them under per-directory names.
        ssh $ssh_options $ssh_user@$pub "cd ${RDIR} && for f in cft*/cft-client.*.log; do echo \"@@FILE \${f%%/*}\"; cat \"\$f\"; done" \
            | awk -v D="${SCRIPT_DIR}" -v H="$pub" '/^@@FILE /{out=D"/cft-client."H"."$2".log"; next} {print > out}'
    elif ssh $ssh_options $ssh_user@$pub "ls ${RDIR}/cft*/cft-client.*.log" >/dev/null 2>&1; then
        scp $ssh_options $ssh_user@$pub:${RDIR}/cft*/cft-client.*.log ${SCRIPT_DIR}/
    else
        echo "Warning: No cft-client.*.log files found on remote server: $pub"
    fi
done

echo "ok: benchmark done"

# Aggregate the client statistics. Stat.String() in benchmark/stat.go writes
# mean / median / p95 / p99 to each client log.
avg_stat() { grep -E "^$1 = " ${SCRIPT_DIR}/cft-client.*.log 2>/dev/null | cut -d= -f2 | awk '{sum+=$1; n++} END {print (n>0) ? sum/n : 0}'; }
TPS=$(grep "Throughput = " ${SCRIPT_DIR}/cft-client.*.log 2>/dev/null | cut -d= -f2 | paste -sd+ | bc) || TPS=0
LAT=$(avg_stat mean)
P50=$(avg_stat median)
P95=$(avg_stat p95)
P99=$(avg_stat p99)
# A replica that panicked during the step means the protocol was not running
# correctly; completions counted by the surviving clients are not a throughput
# of the protocol. The step is recorded as failed (TPS 0).
CRASHED=0
for i in $(seq 1 $SERVER_WORKERS); do
    pub=$(host_of $i)
    if ssh $ssh_options $ssh_user@$pub "grep -l '^panic:' ${RDIR}/cft/server*.log" >/dev/null 2>&1; then
        echo "Warning: a cft-server panicked on $pub; step recorded as failed"
        CRASHED=1
    fi
done
[ "$CRASHED" = 1 ] && TPS=0
echo "{\"TPS\":${TPS}, \"latency\": ${LAT}, \"p50\": ${P50}, \"p95\": ${P95}, \"p99\": ${P99}, \"crashed\": ${CRASHED} }" > ${SCRIPT_DIR}/benchmark.log

cat ${SCRIPT_DIR}/benchmark.log
T_BENCH_DONE=$(date +%s%3N)
pkill cft-server
pkill cft-client
T_DRAIN_END=$(date +%s%3N)
T_BENCH_END=${T_DRAIN_END}
# ── Switch timing of this step ──────────────────────────────────────
cat > "${SCRIPT_DIR}/switch_timing.json" <<EOF
{
  "protocol":"${alg}",
  "t_switch_start":${T_SWITCH_START},
  "t_kill_done":${T_KILL_DONE},
  "t_deploy_done":${T_DEPLOY_DONE},
  "t_servers_ready":${T_SERVERS_READY},
  "t_clients_ready":${T_CLIENTS_READY},
  "t_bench_done":${T_BENCH_DONE},
  "t_cleanup_done":${T_DRAIN_END},
  "t_drain_start":${T_DRAIN_START},
  "t_drain_end":${T_DRAIN_END},
  "t_bench_end":${T_BENCH_END},
  "phases":{
    "kill_old_ms":$((T_KILL_DONE - T_SWITCH_START)),
    "deploy_ms":$((T_DEPLOY_DONE - T_KILL_DONE)),
    "server_boot_ms":$((T_SERVERS_READY - T_DEPLOY_DONE)),
    "client_boot_ms":$((T_CLIENTS_READY - T_SERVERS_READY)),
    "benchmark_ms":$((T_BENCH_DONE - T_CLIENTS_READY)),
    "cleanup_ms":$((T_DRAIN_END - T_BENCH_DONE)),
    "total_ms":$((T_DRAIN_END - T_SWITCH_START))
  }
}
EOF

# FORENSICS_MODE=full: run forensic-verify over the forensics log (the log is
# on a shared file system, so the driver reads it directly) and store its
# report next to the log. FORENSICS_MODE=log leaves the log in place.
if [ "$FORENSICS_MODE" = "full" ] && [ -n "$FORENSICS_LOG" ]; then
    VERIFY_BIN="${SCRIPT_DIR}/forensic-verify"
    VERIFY_OUT="${FORENSICS_LOG%.jsonl}.verify.jsonl"
    if [ -x "$VERIFY_BIN" ]; then
        /usr/bin/time -v "$VERIFY_BIN" "$FORENSICS_LOG" \
            >  "$VERIFY_OUT" \
            2> "${VERIFY_OUT%.jsonl}.time.txt" || \
            echo "warn: forensic-verify exit $?" >&2
        echo "forensics: verifier report at $VERIFY_OUT"
    else
        echo "warn: $VERIFY_BIN not found; skipping verify (run bin/build.sh)" >&2
    fi
fi

rm -f ${SCRIPT_DIR}/cft-server.*.log
rm -f ${SCRIPT_DIR}/cft-client*.log
