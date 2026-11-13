#!/usr/bin/env bash
# Run TLC on one configuration of one specification directory.
#
#   run_tlc.sh <boundary|cert> <cfg-name> [workers] [heap]
#
# Environment:
#   TLA_JAR   path to tla2tools.jar (default: tools/tla2tools.jar next to this script)
#   TLC_OUT   output directory (default: <spec-dir>/out); TLC runs there on copies of the
#             modules and writes tlc_<cfg>.out, counterexample traces (MC_TTrace_*) and
#             the state directory states/<cfg>
#   TLC_TIMEOUT  seconds before TLC is stopped (default 7200)
set -u
cd "$(dirname "$0")"
SPEC=$1; CFG=$2; W=${3:-8}; H=${4:-8g}
JAR=${TLA_JAR:-$PWD/tools/tla2tools.jar}
OUT=${TLC_OUT:-$PWD/$SPEC/out}
[ -f "$JAR" ] || { echo "tla2tools.jar not found: set TLA_JAR (https://github.com/tlaplus/tlaplus/releases)" >&2; exit 2; }
[ -f "$SPEC/$CFG.cfg" ] || { echo "no configuration $SPEC/$CFG.cfg" >&2; exit 2; }
mkdir -p "$OUT/states"
cp "$SPEC"/*.tla "$SPEC/$CFG.cfg" "$OUT/"
( cd "$OUT" && timeout "${TLC_TIMEOUT:-7200}" java -XX:+UseParallelGC -Xmx"$H" -cp "$JAR" tlc2.TLC -workers "$W" -deadlock \
    -metadir "states/$CFG" -config "$CFG.cfg" MC.tla ) > "$OUT/tlc_$CFG.out" 2>&1
echo "$SPEC/$CFG rc=$?"
grep -E "Error|violated|states generated|distinct states|No error|depth|Finished|Parse" "$OUT/tlc_$CFG.out" | head -8
