#!/bin/bash
# Single-host deployment of the switching stack: N Paxos replicas (cft-kv),
# N chained-HotStuff replicas (hskv) and one proxy.
#
#   cluster.sh start RUNDIR      start everything; pids are recorded in RUNDIR/pids
#   cluster.sh proxy RUNDIR      (re)start only the proxy (recovery path)
#   cluster.sh kill RUNDIR NAME  SIGKILL one process (paxos1..N, hs1..N, proxy)
#   cluster.sh stop RUNDIR       terminate everything recorded in RUNDIR/pids
#
# Environment (besides env.sh):
#   CERT=1            the proxy certifies every sealed checkpoint (q_c = n - f_B attestations)
#   REQUIRE_CERT=1    replicas install a snapshot only with a valid certificate
#   CFT_BYZ_<i>=MODE  Byzantine behaviour of Paxos replica i: corrupt | lie | lie-ok | fork
#   BYZ_<i>=MODE      Byzantine behaviour of HotStuff replica i: corrupt | lie | lie-ok
#   CFT_NODIGEST=1, HS_NODIGEST=1   disable the segstart digest check on that instance
#   PROXY_ARGS="..."  extra altun-proxy flags (fault injection, unsafe modes)
#   HS_ROTATION       HotStuff leader rotation (default round-robin), HS_LOG log level
#
# Ports: Paxos peers 37001+i, Paxos HTTP 37101+i, HotStuff peers 37201+i,
# HotStuff KV front ends 37501+i, proxy 37400.
set -u
source "$(dirname "$0")/env.sh"
CMD=$1; RUN=$2
B=$ALTUN_BIN; K=$ALTUN_KEYS; H=$HOST
mkdir -p "$RUN"
PIDS=$RUN/pids

paxi_urls() { local s=""; for i in $(seq 1 $N); do s="$s${s:+,}http://$H:$((37100+i))"; done; echo $s; }
hs_urls()   { local s=""; for i in $(seq 1 $N); do s="$s${s:+,}http://$H:$((37500+i))"; done; echo $s; }

start_proxy() {
  ALTUN_RUN=$RUN GOMAXPROCS=4 $PIN $B/altun-proxy -listen $H:37400 -paxos $(paxi_urls) -hotstuff $(hs_urls) -f $F \
     -pids $PIDS -pub $K/ctl.pub -journal $RUN/segments.jsonl ${CERT:+-cert -cert-keys $K/att} ${PROXY_ARGS:-} >> $RUN/proxy.log 2>&1 < /dev/null &
  echo "proxy $!" >> $PIDS
  for t in $(seq 1 600); do curl -s -m 1 http://$H:37400/admin/status >/dev/null 2>&1 && return 0; sleep 0.1; done
  echo "proxy did not come up" >&2; return 1
}

case $CMD in
start)
  if ss -ltn | grep -qE ":37(0|1|2|3|4|5)[0-9][0-9] "; then echo "ports 370xx-375xx busy" >&2; ss -ltn | grep -E ":37[0-5][0-9][0-9] " >&2; exit 1; fi
  : > $PIDS
  { echo '{"address":{'; for i in $(seq 1 $N); do echo "\"1.$i\":\"tcp://$H:$((37000+i))\"$([ $i -lt $N ] && echo ,)"; done
    echo '},"http_address":{'; for i in $(seq 1 $N); do echo "\"1.$i\":\"http://$H:$((37100+i))\"$([ $i -lt $N ] && echo ,)"; done
    echo '},"policy":"consecutive","threshold":3,"thrifty":false,"buffer_size":1024,"chan_buffer_size":1024,"multiversion":false}'; } > $RUN/paxi.json
  for i in $(seq 1 $N); do
    cbz=CFT_BYZ_$i
    GOMAXPROCS=2 $PIN $B/cft-kv -id 1.$i -config $RUN/paxi.json -log_dir $RUN -log_level ERROR \
        -ephemeral_leader ${CFT_NODIGEST:+-unsafe-nodigest} -attkeys $K/att -name paxos$i ${!cbz:+-byz ${!cbz}} ${REQUIRE_CERT:+-require-cert} > $RUN/paxos$i.out 2>&1 < /dev/null &
    echo "paxos$i $!" >> $PIDS
  done
  for i in $(seq 1 $N); do
    bz=BYZ_$i
    GOMAXPROCS=2 $PIN $B/hskv -id $i -n $N -keys $K/hs -host $H -port 37200 -kvport 37500 \
        -log-level ${HS_LOG:-info} -leader-rotation ${HS_ROTATION:-round-robin} ${!bz:+-byz ${!bz}} ${HS_NODIGEST:+-unsafe-nodigest} \
        -attkeys $K/att ${REQUIRE_CERT:+-require-cert} > $RUN/hs$i.out 2>&1 < /dev/null &
    echo "hs$i $!" >> $PIDS
  done
  for i in $(seq 1 $N); do
    for t in $(seq 1 300); do curl -s -m 1 http://$H:$((37500+i))/altun/status >/dev/null 2>&1 && curl -s -m 1 http://$H:$((37100+i))/altun/status >/dev/null 2>&1 && break; sleep 0.1; done
  done
  start_proxy
  ;;
proxy) start_proxy ;;
kill)
  pid=$(awk -v n=$3 '$1==n{p=$2} END{print p}' $PIDS); [ -n "$pid" ] && kill -9 $pid && echo "killed $3 ($pid) at $(date +%s%N)" | tee -a $RUN/kills.log ;;
stop)
  awk '{print $2}' $PIDS | xargs -r kill 2>/dev/null; sleep 0.5; awk '{print $2}' $PIDS | xargs -r kill -9 2>/dev/null; true ;;
*) echo "usage: cluster.sh start|proxy|kill|stop RUNDIR [NAME]" >&2; exit 2 ;;
esac
