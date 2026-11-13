#!/usr/bin/env bash
# Docker-based multi-host WAN emulation for the benchmark scripts.
#
# Every replica and client runs in its own long-lived container on the cluster hosts
# (START_NODE .. START_NODE+ALTUNWAN_NHOSTS-1, from env.sh). Containers sit on Docker's default
# bridge; replica ports are published 1:1 (host port == container port). Each container has an
# /etc/hosts that maps every slot alias "altunwan-<WWW>-r<j>" / "altunwan-<WWW>-c<m>" to the
# container IP when the slot is on the same host, else to that host's IP. Cross-region delay is a
# netem qdisc inside each container's own network namespace (host network untouched).
#
#   altunwan.sh up                  create (once) / start the containers on every host, write /etc/hosts
#   altunwan.sh hosts               rewrite /etc/hosts in every container (after a container restart)
#   altunwan.sh netem <n> <clients> apply WAN_DELAY_MS / WAN_JITTER_MS / WAN_CLIENT_REGION for a step with n replicas
#   altunwan.sh show                print the qdisc/filters of every container
#   altunwan.sh probe [n]           TCP RTT probe between containers (same host, same region, cross region)
#   altunwan.sh status              list the containers on every host
#   altunwan.sh stop                docker stop the containers (they are never removed)
#
# Objects created per host: image ALTUNWAN_IMAGE (built with docker/mkrootfs.sh), containers
# <ALTUNWAN_CPREFIX>r1..r4 (replica slots) and <ALTUNWAN_CPREFIX>c1..c8 (client slots); directories
# REMOTE_DIR and ALTUNWAN_DIR, bind-mounted into every container.
set -u
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
. "${SCRIPT_DIR}/env.sh"
NH=${ALTUNWAN_NHOSTS:-6}
IMG=${ALTUNWAN_IMAGE:-altunwan_base:v1}
CP=${ALTUNWAN_CPREFIX:-altunwan_}   # container name prefix
NR=4                      # replica slots per host (NODES_PER_WORKER max)
NC=${CLIENTS_PER_WORKER}  # client slots per host
SSH="ssh $ssh_options -o ConnectTimeout=10 -o BatchMode=yes"
RDIR=${REMOTE_DIR:?}
WDIR=${ALTUNWAN_DIR:?}

# Ports of replica slot j (1..4): CFT tcp / http, mir (config-gen.sh's cumulative
# 38001/38011/38031/38061), hotstuff replica / client, probe.
# mir listens on base, base+1, base+2 (one port per dispatcher, d = 3): base..base+9 is published.
slot_ports() { local j=$1 m=$((38001 + 5 * (j - 1) * j)); echo "$((31735 + j - 1)) $((18080 + j - 1)) $m-$((m + 9)) $((39001 + 10 * (j - 1))) $((39002 + 10 * (j - 1))) $((39500 + j))"; }
# Ephemeral source-port range of client slot m: slots 1..NC/2 are region A (32768-49151),
# slots NC/2+1..NC region B (49152-65535); 4096 ports each, disjoint per host, so the host's
# MASQUERADE keeps the source port and a replica can tell a client's region by destination port.
client_range() { local m=$1 half=$((NC / 2)) lo
  if [ "$m" -le "$half" ]; then lo=$((32768 + (m - 1) * 4096)); else lo=$((49152 + (m - half - 1) * 4096)); fi
  echo "$lo $((lo + 4095))"; }

host_ip() { getent hosts "$1" | awk '{print $1; exit}'; }

cmd_up() {
  local pids=()
  for i in $(seq 1 $NH); do
    local h=$(host_of $i) w=$(num_of $i) script=""
    script+="set -e; mkdir -p $RDIR $WDIR/bin; "
    script+="docker image inspect $IMG >/dev/null; "
    for j in $(seq 1 $NR); do
      local p="" ports=$(slot_ports $j)
      for q in $ports; do p+=" -p $q:$q"; done
      set -- $ports
      script+="if ! docker inspect ${CP}r$j >/dev/null 2>&1; then docker run -d --name ${CP}r$j --hostname altunwan-$w-r$j --label altunwan=1 --user $(id -u):$(id -g) --cap-add NET_ADMIN --ulimit nofile=1024:1048576 -v $RDIR:$RDIR -v $WDIR:$WDIR -e HS_REPLICA_PORT=$4 -e HS_CLIENT_PORT=$5 -e ALTUNWAN_PROBE_PORT=$6 $p $IMG /usr/bin/sleep infinity >/dev/null; fi; "
      script+="[ \"\$(docker inspect -f '{{.State.Running}}' ${CP}r$j)\" = true ] || docker start ${CP}r$j >/dev/null; "
    done
    for m in $(seq 1 $NC); do
      local rg=$(client_range $m)
      script+="if ! docker inspect ${CP}c$m >/dev/null 2>&1; then docker run -d --name ${CP}c$m --hostname altunwan-$w-c$m --label altunwan=1 --user $(id -u):$(id -g) --cap-add NET_ADMIN --ulimit nofile=1024:1048576 --sysctl net.ipv4.ip_local_port_range='$rg' -v $RDIR:$RDIR -v $WDIR:$WDIR $IMG /usr/bin/sleep infinity >/dev/null; fi; "
      script+="[ \"\$(docker inspect -f '{{.State.Running}}' ${CP}c$m)\" = true ] || docker start ${CP}c$m >/dev/null; "
    done
    script+="echo $h: \$(docker ps --filter name=^${CP} --format '{{.Names}}' | wc -l) running"
    ( $SSH $ssh_user@$h "$script" </dev/null ) &
    pids+=($!)
  done
  for p in "${pids[@]}"; do wait $p; done
  cmd_hosts
}

# /etc/hosts of every container (container IPs on the default bridge can change on restart).
cmd_hosts() {
  local tmp=$(mktemp -d) pids=()
  for i in $(seq 1 $NH); do
    local h=$(host_of $i)
    $SSH $ssh_user@$h "docker ps --filter name=^${CP} --format '{{.Names}}' | xargs -r docker inspect -f '{{.Name}} {{.NetworkSettings.IPAddress}}'" </dev/null > $tmp/$i &
    pids+=($!)
  done
  for p in "${pids[@]}"; do wait $p; done
  # driver-side copy of the alias -> container IP / host IP map (hotstuff puts both in its TLS certificates)
  : > "${SCRIPT_DIR}/altunwan-ips.txt"
  for k in $(seq 1 $NH); do
    while read -r name ip; do echo "altunwan-$(num_of $k)-${name#/${CP}} $ip $(host_ip $(host_of $k))"; done < $tmp/$k >> "${SCRIPT_DIR}/altunwan-ips.txt"
  done
  pids=()
  for i in $(seq 1 $NH); do
    local h=$(host_of $i) w=$(num_of $i)
    local common="127.0.0.1 localhost"$'\n'
    for k in $(seq 1 $NH); do common+="$(host_ip $(host_of $k)) $(host_of $k)"$'\n'; done
    local script=""
    while read -r name ip; do
      name=${name#/}
      local f="$common"
      for k in $(seq 1 $NH); do
        local wk=$(num_of $k) hip=$(host_ip $(host_of $k))
        for s in $(seq 1 $NR) ; do
          if [ $k = $i ]; then f+="$(awk -v n=/${CP}r$s '$1==n{print $2}' $tmp/$k) altunwan-$wk-r$s"$'\n'; else f+="$hip altunwan-$wk-r$s"$'\n'; fi
        done
        for s in $(seq 1 $NC) ; do
          if [ $k = $i ]; then f+="$(awk -v n=/${CP}c$s '$1==n{print $2}' $tmp/$k) altunwan-$wk-c$s"$'\n'; else f+="$hip altunwan-$wk-c$s"$'\n'; fi
        done
      done
      script+="docker exec -u 0 -i $name sh -c 'cat > /etc/hosts' <<'EOF_HOSTS'"$'\n'"$f"EOF_HOSTS$'\n'
    done < $tmp/$i
    ( $SSH $ssh_user@$h "bash -s" <<< "$script" ) &
    pids+=($!)
  done
  for p in "${pids[@]}"; do wait $p; done
  echo "altunwan: /etc/hosts written in $(cat $tmp/* | wc -l) containers"
}

# netem for one step. Regions: replica host 1..ceil(SW/2) = region A, the rest region B
# (n = 8: host START_NODE = A (CFT zone 1, ids 1-4), START_NODE+1 = B (zone 2, ids 5-8)).
# Clients: slot m <= CLIENTS_PER_WORKER/2 on each client host = region A, the rest region B
# (client ids 1-4, 9-12, ... = A; 5-8, 13-16, ... = B). A container delays (egress) every packet to
# an other-region replica host; a replica container also delays packets to client hosts whose
# destination port lies in the other region's client port range (see client_range).
cmd_netem() {
  local n=${1:?n} clients=${2:?clients} d=${WAN_DELAY_MS:-0} jit=${WAN_JITTER_MS:-0}
  # WAN_CLIENT_REGION=split (default: slots 1..NC/2 in A, the rest in B) | A | B (every client slot in that region;
  # the replicas of the other region then delay all their packets to the client hosts)
  local creg=${WAN_CLIENT_REGION:-split}
  case "$creg" in split|A|B) ;; *) echo "altunwan: WAN_CLIENT_REGION=$creg (split|A|B)"; return 1 ;; esac
  local npw=$NODES_PER_WORKER
  local sw=$(( (n + npw - 1) / npw )) cw=$(( (clients + NC - 1) / NC ))
  local half=$(( (sw + 1) / 2 ))
  local A=() B=() C=()
  for i in $(seq 1 $sw); do if [ $i -le $half ]; then A+=($(host_ip $(host_of $i))); else B+=($(host_ip $(host_of $i))); fi; done
  for i in $(seq $((sw + 1)) $((sw + cw))); do C+=($(host_ip $(host_of $i))); done
  local delay="delay ${d}ms"; [ "$jit" != 0 ] && delay+=" ${jit}ms"
  local pids=()
  for i in $(seq 1 $NH); do
    local h=$(host_of $i) script="" names=()
    for j in $(seq 1 $NR); do names+=(${CP}r$j); done
    for m in $(seq 1 $NC); do names+=(${CP}c$m); done
    for name in "${names[@]}"; do
      local t="tc qdisc del dev eth0 root 2>/dev/null; "
      local region="" other=() cport=""
      if [ $i -le $sw ] && [[ $name == ${CP}r* ]]; then
        if [ $i -le $half ]; then region=A; other=("${B[@]}"); cport="0xc000"; else region=B; other=("${A[@]}"); cport="0x8000"; fi
        if [ "$creg" != split ]; then if [ $region = $creg ]; then cport=""; else cport="all"; fi; fi
      elif [ $i -gt $sw ] && [ $i -le $((sw + cw)) ] && [[ $name == ${CP}c* ]]; then
        local m=${name#${CP}c}
        if [ $m -le $((NC / 2)) ]; then region=A; other=("${B[@]}"); else region=B; other=("${A[@]}"); fi
        if [ "$creg" = A ]; then region=A; other=("${B[@]}"); elif [ "$creg" = B ]; then region=B; other=("${A[@]}"); fi
        cport=""
      fi
      if [ "$d" != 0 ] && [ -n "$region" ] && [ ${#other[@]} -gt 0 ]; then
        t+="tc qdisc add dev eth0 root handle 1: prio bands 3 priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 && "
        t+="tc qdisc add dev eth0 parent 1:1 handle 10: pfifo limit 100000 && "
        t+="tc qdisc add dev eth0 parent 1:2 handle 20: pfifo limit 100000 && "
        t+="tc qdisc add dev eth0 parent 1:3 handle 30: netem $delay limit 100000 && "
        for ip in "${other[@]}"; do t+="tc filter add dev eth0 parent 1: protocol ip prio 1 u32 match ip dst $ip/32 flowid 1:3 && "; done
        if [ "$cport" = all ]; then
          for ip in "${C[@]}"; do t+="tc filter add dev eth0 parent 1: protocol ip prio 2 u32 match ip dst $ip/32 flowid 1:3 && "; done
          t+="echo $h/$name region $region: netem $delay to ${other[*]} + client hosts, all clients in $creg"
        else
        if [ -n "$cport" ]; then
          for ip in "${C[@]}"; do t+="tc filter add dev eth0 parent 1: protocol ip prio 2 u32 match ip dst $ip/32 match ip dport $cport 0xc000 flowid 1:3 && "; done
        fi
        t+="echo $h/$name region $region: netem $delay to ${other[*]}${cport:+ + client hosts with dport/0xc000 = $cport}"
        fi
      else
        t+="true"
      fi
      script+="docker exec -u 0 $name sh -c '$t' & "
    done
    script+="wait"
    ( $SSH $ssh_user@$h "$script" </dev/null ) &
    pids+=($!)
  done
  local rc=0
  for p in "${pids[@]}"; do wait $p || rc=1; done
  echo "altunwan: netem WAN_DELAY_MS=$d WAN_JITTER_MS=$jit n=$n clients=$clients${WAN_CLIENT_REGION:+ client region $creg} (replica hosts A: ${A[*]} B: ${B[*]}; client hosts: ${C[*]})"
  return $rc
}

cmd_show() {
  for i in $(seq 1 $NH); do
    $SSH $ssh_user@$(host_of $i) "for c in \$(docker ps --filter name=^${CP} --format '{{.Names}}' | sort -V); do echo \"== \$(hostname) \$c\"; docker exec \$c tc qdisc show dev eth0; docker exec \$c tc filter show dev eth0 | grep -E 'match|flowid' ; done" </dev/null
  done
}

cmd_status() {
  for i in $(seq 1 $NH); do
    $SSH $ssh_user@$(host_of $i) "echo \"== \$(hostname)\"; docker ps -a --filter name=^${CP} --format '{{.Names}} {{.Status}} {{.Image}}' | sort -V | tr '\n' ';'; echo" </dev/null
  done
}

cmd_stop() {
  local pids=()
  for i in $(seq 1 $NH); do
    ( $SSH $ssh_user@$(host_of $i) "docker ps -q --filter name=^${CP} --filter label=altunwan=1 | xargs -r docker stop -t 2 | wc -l | sed 's/^/$(host_of $i): stopped /'" </dev/null ) &
    pids+=($!)
  done
  for p in "${pids[@]}"; do wait $p; done
}

# Probe: a TCP echo server in replica slot r1 of hosts 1 and 2 and r2 of host 1; RTTs from
# r2@1 (same host), c1/c8 on host 3 (client region A / B) and r1@2 (cross region).
cmd_probe() {
  local n=${1:-20} h1=$(host_of 1) h2=$(host_of 2) h3=$(host_of 3) w1=$(num_of 1) w2=$(num_of 2)
  for h in $h1 $h2; do
    $SSH $ssh_user@$h "for j in 1 2; do docker exec -d ${CP}r\$j sh -c 'pgrep -x altunwan-probe >/dev/null || exec altunwan-probe serve \$ALTUNWAN_PROBE_PORT'; done" </dev/null
  done
  sleep 1
  local p1=$((39500 + 1)) p2=$((39500 + 2))
  echo "# from $h1 r2 (region A)"
  $SSH $ssh_user@$h1 "docker exec ${CP}r2 altunwan-probe rtt altunwan-$w1-r1:$p1 $n; docker exec ${CP}r2 altunwan-probe rtt altunwan-$w2-r1:$p1 $n" </dev/null
  echo "# from $h2 r2 (region B)"
  $SSH $ssh_user@$h2 "docker exec ${CP}r2 altunwan-probe rtt altunwan-$w2-r1:$p1 $n; docker exec ${CP}r2 altunwan-probe rtt altunwan-$w1-r1:$p1 $n" </dev/null
  echo "# from $h3 c1 (client region A) and c$NC (client region B)"
  $SSH $ssh_user@$h3 "for c in c1 c$NC; do docker exec ${CP}\$c altunwan-probe rtt altunwan-$w1-r1:$p1 $n; docker exec ${CP}\$c altunwan-probe rtt altunwan-$w2-r1:$p1 $n; done" </dev/null
  for h in $h1 $h2; do $SSH $ssh_user@$h "pkill -x altunwan-probe; true" </dev/null; done
}

case "${1:-}" in
  up) cmd_up ;;
  hosts) cmd_hosts ;;
  netem) shift; cmd_netem "$@" ;;
  show) cmd_show ;;
  probe) shift; cmd_probe "$@" ;;
  status) cmd_status ;;
  stop) cmd_stop ;;
  *) sed -n '2,21p' "$0"; exit 2 ;;
esac
