#!/bin/bash
# One switching run: fresh cluster, closed-loop load through the proxy, a
# signed schedule of policy-window directives, optional fault injection, and
# the offline history check.
#
#   run_switching.sh RUNDIR
#     DUR=30        seconds of load            CLIENTS=16  closed-loop clients
#     KEYS=200      key space                  EVERY=1s    directive period
#     COUNT=20      number of directives       WARM=3      seconds before the first directive
#     WINDOW0=1     first policy-window id     SEED=1      load generator seed
#     CTL_F, CTL_N  f and n carried by the directives (default F, N)
#     CTL_FB        certification bound f_B carried by the directives (default: f)
#     FAULT="cmd"   shell command started in the background after FAULT_AT seconds
#     plus everything cluster.sh understands (CERT, REQUIRE_CERT, BYZ_i, PROXY_ARGS, ...)
#
# Outputs in RUNDIR: history.jsonl (client history), switches.jsonl (directive
# reports), segments.jsonl (proxy journal), check.json / check.out (altun-check).
set -u
source "$(dirname "$0")/env.sh"
B=$ALTUN_BIN; K=$ALTUN_KEYS; H=$HOST
RUN=$1
DUR=${DUR:-30}; CLIENTS=${CLIENTS:-16}; KEYS=${KEYS:-200}; EVERY=${EVERY:-1s}; COUNT=${COUNT:-20}; WARM=${WARM:-3}
mkdir -p "$RUN"
"$SCRIPTS_DIR/cluster.sh" start "$RUN" || { "$SCRIPTS_DIR/cluster.sh" stop "$RUN"; exit 1; }
GOMAXPROCS=2 $PIN $B/altun-load -proxy http://$H:37400 -clients $CLIENTS -keys $KEYS \
   -duration ${DUR}s -grace 60s -out $RUN/history.jsonl -seed ${SEED:-1} > $RUN/load.log 2>&1 < /dev/null &
LOADPID=$!
sleep $WARM
if [ "$COUNT" -gt 0 ]; then
  $PIN $B/altun-ctl -key $K/ctl.key -proxy http://$H:37400 -every $EVERY -count $COUNT \
     -window ${WINDOW0:-1} -f ${CTL_F:-$F} -n ${CTL_N:-$N} ${CTL_FB:+-fb $CTL_FB} -out $RUN/switches.jsonl > /dev/null 2>&1 < /dev/null &
  CTLPID=$!
fi
if [ -n "${FAULT:-}" ]; then ( sleep ${FAULT_AT:-0}; RUN=$RUN SCRIPTS_DIR=$SCRIPTS_DIR bash -c "$FAULT" ) > $RUN/fault.log 2>&1 < /dev/null & fi
wait $LOADPID
[ -n "${CTLPID:-}" ] && wait $CTLPID
"$SCRIPTS_DIR/cluster.sh" stop "$RUN"
$B/altun-check -journal $RUN/segments.jsonl -out $RUN/check.json -export-csv $RUN/history.csv -export-meta $RUN/vlog_meta.jsonl $RUN/history.jsonl > $RUN/check.out 2>&1
echo "check exit $?" >> $RUN/check.out
tail -1 $RUN/load.log
grep -E '"(violations_total|lin_violating_keys|dup_positions|read_violations|ops|pending|segments)"' $RUN/check.json | tr -d '\n '; echo
