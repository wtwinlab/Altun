#!/bin/bash -e

set -x


SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

. ${SCRIPT_DIR}/../../bin/env.sh

# Host naming and remote paths come from bin/env.sh:
#   WORKER_PREFIX  host name prefix, hosts are ${WORKER_PREFIX}<3-digit index> (default worker-)
#   REMOTE_DIR     per-host working directory that receives the mir<id> directories
#   REMOTE_BASE    parent of REMOTE_DIR, where stop.sh is placed (default: dirname of REMOTE_DIR)
WORKER_PREFIX=${WORKER_PREFIX:-worker-}
REMOTE_BASE=${REMOTE_BASE:-$(dirname "$REMOTE_DIR")}

getIP() {
	grep -w $1 ${SCRIPT_DIR}/cluster-instance.info | awk '{ print $2}'
}

servers=$(grep server ${SCRIPT_DIR}/cluster-instance.info | awk '{ print $1}')
clients=$(grep client ${SCRIPT_DIR}/cluster-instance.info | awk '{ print $1}')
NUM_CLIENTS=$(grep -c client ${SCRIPT_DIR}/cluster-instance.info)
NUM_NODES=$(grep -c server ${SCRIPT_DIR}/cluster-instance.info)
echo "NUM_CLIENTS:$NUM_CLIENTS, NUM_NODES:$NUM_NODES"
SERVER_WORKERS=$((NUM_NODES / NODES_PER_WORKER + (NUM_NODES % NODES_PER_WORKER == 0 ? 0 : 1)))
CLIENT_WORKERS=$((NUM_CLIENTS / CLIENTS_PER_WORKER + (NUM_CLIENTS % CLIENTS_PER_WORKER == 0 ? 0 : 1)))
echo "Use SERVER_WORKERS size = $SERVER_WORKERS *$NODES_PER_WORKER"
echo "Use CLIENT_WORKER size = $CLIENT_WORKERS *$CLIENTS_PER_WORKER"

# run servers
for i in $(seq 1 $SERVER_WORKERS); do
    WORKER_ID=$(printf "%03d" $((START_NODE + i - 1)))
    pub="${WORKER_PREFIX}${WORKER_ID}"
    for j in $(seq 1 $NODES_PER_WORKER); do
        id=$((NODES_PER_WORKER * (i - 1) + j))
        if (( id > NUM_NODES )); then
            break
        fi
        p="server${id}"
        if [ "${ALTUNWAN:-0}" = 1 ]; then
            # Container mode: replica id runs in container ${ALTUNWAN_CPREFIX}r$j of its host (files bind-mounted)
            ssh $ssh_user@$pub $ssh_options "docker exec -d -w ${REMOTE_DIR}/mir${id} ${ALTUNWAN_CPREFIX}r$j sh -c 'exec ./bin/mir-server ./config/serverconfig/config.yml server > server.out 2>&1'" &
        else
        ssh $ssh_user@$pub $ssh_options "cd ${REMOTE_DIR}/mir${id}/ && source run-server.sh  > /dev/null 2>&1 & " &
        fi
    done
done

# Clients are started in parallel, one ssh per client host; each client
# measures a MIR_WINDOW_MS window (default 10 s) from its START, then writes
# window.out and status=FINISHED.
echo "" > ${SCRIPT_DIR}/mir-exp.out   # a failed run must not reuse an old result
WINDOW_MS=${MIR_WINDOW_MS:-10000}
CLIENT_HOSTS=()
cpids=()
for i in $(seq 1 $CLIENT_WORKERS); do
    pub="${WORKER_PREFIX}$(printf "%03d" $((START_NODE + SERVER_WORKERS + i - 1)))"
    CLIENT_HOSTS+=($pub)
    cmd=""
    for j in $(seq 1 $CLIENTS_PER_WORKER); do
        id=$((CLIENTS_PER_WORKER * (i - 1) + j))
        (( id > NUM_CLIENTS )) && break
        if [ "${ALTUNWAN:-0}" = 1 ]; then
            # Container mode: client id runs in container ${ALTUNWAN_CPREFIX}c$j of its host (clear the status file, then start)
            cmd+="(cd ${REMOTE_DIR}/mir${id}/ && rm -rf status && docker exec -d -w ${REMOTE_DIR}/mir${id} ${ALTUNWAN_CPREFIX}c$j sh -c 'exec ./bin/mir-client ./config/clientconfig/config.yml client > client.out 2>&1'); "
        else
        cmd+="(cd ${REMOTE_DIR}/mir${id}/ && source run-client.sh >/dev/null 2>&1); "
        fi
    done
    ssh $ssh_user@$pub $ssh_options "$cmd" </dev/null &
    cpids+=($!)
done
for p in "${cpids[@]}"; do wait $p; done

# Wait until every client reports FINISHED. Hard cap: window + MIR_WAIT_EXTRA_S
# (default 60 s) after launch, so a hung client cannot stall the step.
t_launch=$(date +%s)
deadline=$((t_launch + WINDOW_MS/1000 + ${MIR_WAIT_EXTRA_S:-60}))
while true; do
    ready=0
    for pub in "${CLIENT_HOSTS[@]}"; do
        n=$(ssh $ssh_user@$pub $ssh_options "grep -l FINISHED ${REMOTE_DIR}/mir*/STATUS.sh 2>/dev/null | wc -l" </dev/null || echo 0)
        ready=$((ready + ${n:-0}))
    done
    echo "clients FINISHED: $ready/$NUM_CLIENTS"
    [ $ready -ge $NUM_CLIENTS ] && break
    if [ $(date +%s) -ge $deadline ]; then
        echo "Timeout: only $ready/$NUM_CLIENTS clients finished"
        break
    fi
    sleep 1
done
echo "Clients done after $(( $(date +%s) - t_launch )) s"

mkdir -p ${SCRIPT_DIR}/experiment-output
: > ${SCRIPT_DIR}/experiment-output/window.all
: > ${SCRIPT_DIR}/experiment-output/crash.all
for pub in "${CLIENT_HOSTS[@]}"; do
    ssh $ssh_user@$pub $ssh_options "cat ${REMOTE_DIR}/mir*/window.out 2>/dev/null; grep -H -i -m3 'panic\|fatal' ${REMOTE_DIR}/mir*/client.out 1>&2; true" </dev/null \
        >> ${SCRIPT_DIR}/experiment-output/window.all 2>> ${SCRIPT_DIR}/experiment-output/crash.all || true
done

for i in $(seq 1 $SERVER_WORKERS); do
    pub="${WORKER_PREFIX}$(printf "%03d" $((START_NODE + i - 1)))"
    ssh $ssh_user@$pub $ssh_options "grep -H -i -m3 'panic\|fatal' ${REMOTE_DIR}/mir*/server.out 1>&2; true" </dev/null 2>> ${SCRIPT_DIR}/experiment-output/crash.all || true
    # separate ssh: stop.sh's "pkill -f mir" would also match a command line naming mir* paths
    ssh $ssh_user@$pub $ssh_options "cd ${REMOTE_BASE}/ && source stop.sh" </dev/null || true
    for j in $(seq 1 $NODES_PER_WORKER); do
        id=$((NODES_PER_WORKER * (i - 1) + j))
        (( id > NUM_NODES )) && break
        scp -q $ssh_options $ssh_user@$pub:${REMOTE_DIR}/mir${id}/server.out ${SCRIPT_DIR}/experiment-output/server${id}.out </dev/null || true
        # keep logs of crashed replicas (stack traces) across steps
        if grep -q "fatal error\|^panic:" ${SCRIPT_DIR}/experiment-output/server${id}.out 2>/dev/null; then
            cp ${SCRIPT_DIR}/experiment-output/server${id}.out ${SCRIPT_DIR}/experiment-output/crash-$(date +%Y%m%d-%H%M%S)-server${id}.out
        fi
    done
done
for pub in "${CLIENT_HOSTS[@]}"; do
    ssh $ssh_user@$pub $ssh_options "pkill -9 -x mir-client; true" </dev/null || true
done
echo "== crash/panic lines:"; cat ${SCRIPT_DIR}/experiment-output/crash.all

# Throughput = sum over clients of (requests with F+1 DELIVERED inside the
# client's window) / window length; latency = mean submit-to-commit latency of
# those requests.  apply-mir.sh parses the printed lines.
python3 - ${SCRIPT_DIR}/experiment-output/window.all $NUM_CLIENTS > ${SCRIPT_DIR}/mir-exp.out <<'PY'
import sys
rows=[dict(kv.split("=",1) for kv in l.split()[1:]) for l in open(sys.argv[1]) if l.startswith("WINDOW")]
want=int(sys.argv[2])
print("Clients reporting: %d/%d" % (len(rows), want))
if rows:
    tps=sum(int(r["completed"])/(int(r["elapsed_us"])/1e6) for r in rows)
    done=sum(int(r["completed"]) for r in rows)
    lat=sum(int(r["latsum_us"]) for r in rows)/1000.0/done if done else 0
    starts=[int(r["start_us"]) for r in rows]
    if len(rows) == want:
        print("Throughput: %f r/s" % tps)
        print("Average end to end latency: %f ms" % lat)
    else:
        print("Partial (not reported) throughput: %f r/s latency %f ms" % (tps, lat))
    print("Completed: %d  Sent: %d  Window_s(mean): %.2f  Exhausted_clients: %d  Start_spread_s: %.2f" % (
        done, sum(int(r["sent"]) for r in rows), sum(int(r["elapsed_us"]) for r in rows)/1e6/len(rows),
        sum(r["exhausted"]=="true" for r in rows), (max(starts)-min(starts))/1e6))
    if all("persec" in r for r in rows):
        ps=[sum(int(r["persec"].split(",")[i]) for r in rows) for i in range(len(rows[0]["persec"].split(",")))]
        while ps and ps[-1]==0: ps.pop()
        print("Completed_per_second: " + " ".join(map(str,ps)))
PY
cat ${SCRIPT_DIR}/mir-exp.out
date
