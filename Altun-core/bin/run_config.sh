#!/bin/bash
# run_config.sh — run one benchmark step for the protocol named in a JSON description.
#
# Usage: run_config.sh [config_RL.json]
#
# Dispatches to apply-cft.sh (paxos, epaxos, vpaxos, wpaxos), apply-bft.sh
# (chainedhotstuff, simplehotstuff, fasthotstuff) or mir/deployment/apply-mir.sh
# (pbft, mirbft) and leaves the step result in benchmark.log:
#   {"TPS": <req/s>, "latency": <ms>, ...}
set -x

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

JSON_FILE=${1:-"${SCRIPT_DIR}/config_RL.json"}

if [ ! -f "$JSON_FILE" ]; then
    echo "Error: $JSON_FILE does not exist."
    exit 1
fi

required_fields=(
    .algorithm
    .nodesize
    .clients
    .concurrency
    .ratelimit
    .batchsize
    .payloadsize
    .requests
    .writeratio
    .distribution
)

for field in "${required_fields[@]}"; do
    if ! jq -e "has(\"${field#.}\")" "$JSON_FILE" >/dev/null; then
        echo "Error: Required field '${field#.}' missing from JSON file"
        exit 1
    fi
done

alg=$(jq -r '.algorithm' "$JSON_FILE")
replicas=$(jq '.nodesize' "$JSON_FILE")
clients=$(jq '.clients' "$JSON_FILE")
concurrency=$(jq '.concurrency' "$JSON_FILE")
nratelimit=$(jq '.ratelimit' "$JSON_FILE")
batchsize=$(jq '.batchsize' "$JSON_FILE")
payloadsize=$(jq '.payloadsize' "$JSON_FILE")
requests=$(jq -r '.requests' "$JSON_FILE")
writeratio=$(jq -r '.writeratio' "$JSON_FILE")
distribution=$(jq -r '.distribution' "$JSON_FILE")


echo "======================= start running ================"
. ${SCRIPT_DIR}/env.sh
if [ "${ALTUNWAN:-0}" = 1 ]; then
    # Docker/WAN emulation: (re)apply this step's cross-region netem in every container.
    if ! bash ${SCRIPT_DIR}/altunwan.sh netem ${replicas} ${clients} > ${SCRIPT_DIR}/altunwan-netem.log 2>&1; then
        echo "altunwan netem failed (containers up? bin/altunwan.sh up)"; cat ${SCRIPT_DIR}/altunwan-netem.log; exit 1
    fi
    tail -1 ${SCRIPT_DIR}/altunwan-netem.log
fi
rm -f ${SCRIPT_DIR}/benchmark.log

if [[ "${alg}" == "paxos" || "${alg}" == "epaxos" || "${alg}" == "vpaxos" || "${alg}" == "wpaxos" ]]; then
    echo "Running CFT algorithm: ${alg}"
    ${SCRIPT_DIR}/apply-cft.sh ${JSON_FILE} > ${SCRIPT_DIR}/cft-run.log 2> ${SCRIPT_DIR}/cft-run.err
elif [[ "$alg" == "chainedhotstuff" || "${alg}" == "simplehotstuff" || "${alg}" == "fasthotstuff" ]]; then
    echo "Running BFT algorithm: ${alg}"
    ${SCRIPT_DIR}/apply-bft.sh ${alg} ${concurrency} ${replicas} ${clients} ${batchsize} ${payloadsize} ${nratelimit} > ${SCRIPT_DIR}/bft-run.log 2> ${SCRIPT_DIR}/bft-run.err
elif [[ ( "$alg" == "mirbft" || "$alg" == "pbft" ) && ( -n "${SLOW_PROPOSAL_MS:-}${ABSENT_IDS:-}${BFT_ABSENT_IDS:-}" ) ]]; then
    # The mir path (pbft, mirbft) has no fault injection, so it cannot run the
    # configured fault: the step is recorded as unavailable.
    echo "Running BFT algorithm: ${alg}: unavailable under faults (no fault injection on the mir path)"
    echo '{"TPS":0, "latency": 0, "unavailable": 1}' > ${SCRIPT_DIR}/benchmark.log
elif [[ "$alg" == "mirbft" ]]; then
    leaders=${replicas}
    echo "Running BFT algorithm: ${alg} ${leaders}"
    ${SCRIPT_DIR}/../mir/deployment/apply-mir.sh ${replicas} ${clients} ${concurrency} ${nratelimit} ${payloadsize} ${batchsize} ${requests} ${leaders} > ${SCRIPT_DIR}/mir-run.log
elif [[ "$alg" == "pbft" ]]; then
    leaders=1
    echo "Running BFT algorithm: ${alg}"
    ${SCRIPT_DIR}/../mir/deployment/apply-mir.sh ${replicas} ${clients} ${concurrency} ${nratelimit} ${payloadsize} ${batchsize} ${requests} ${leaders} > ${SCRIPT_DIR}/mir-run.log
else
  echo "Invalid Alg: ${alg}"
  exit 1
fi


# Validity rule: a step whose mean latency is below the floor did not run
# consensus (with the WAN emulation the floor is the one-way inter-region
# delay: a committed request needs a cross-region round trip), and TPS x
# request size above 5 GB/s exceeds the client NICs. Such a step is recorded
# as failed (TPS 0) before any selector sees it; the measured value is kept as
# TPS_measured.
if [ -s ${SCRIPT_DIR}/benchmark.log ]; then
    FLOOR=1
    [ "${ALTUNWAN:-0}" = 1 ] && [ "${WAN_DELAY_MS:-0}" != 0 ] && FLOOR=${WAN_DELAY_MS}
    # WAN_FLOOR_MS overrides the floor (region-local key ranges let WPaxos/VPaxos commit inside one region)
    [ -n "${WAN_FLOOR_MS:-}" ] && FLOOR=${WAN_FLOOR_MS}
    python3 - "${SCRIPT_DIR}/benchmark.log" "$FLOOR" "$payloadsize" <<'PY'
import json, re, sys
p, floor, size = sys.argv[1], float(sys.argv[2]), float(sys.argv[3])
txt = open(p).read()
try:
    d = json.loads(txt)
except Exception:
    m = re.search(r'"TPS"\s*:\s*([0-9.eE+-]+).*?"latency"\s*:\s*([0-9.eE+-]+)', txt, re.S)
    if not m:
        sys.exit(0)
    d = {"TPS": float(m.group(1)), "latency": float(m.group(2))}
try:
    tps, lat = float(d.get("TPS") or 0), float(d.get("latency") or 0)
except (TypeError, ValueError):
    sys.exit(0)
if tps > 0 and (lat < floor or tps * size > 5e9):
    d.update(TPS_measured=tps, TPS=0, invalid=f"latency {lat:.3f} ms < {floor:g} ms or > 5 GB/s")
    open(p, "w").write(json.dumps(d))
    print("validity: step recorded as failed:", d["invalid"], "measured TPS", tps)
PY
fi

cat ${SCRIPT_DIR}/benchmark.log 2>/dev/null || true
echo "ok: ${alg} apply config successfully. "
