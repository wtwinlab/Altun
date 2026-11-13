#!/bin/bash
# Named switching scenarios: fault injection at switch phases, Byzantine
# replicas, unsafe modes that disable one safeguard, and checkpoint
# certification.  Each scenario is one run_switching.sh run in OUTDIR/NAME.
#
#   scenarios.sh OUTDIR NAME [NAME...]
#
# Switch directions: c2b = Paxos -> HotStuff, b2c = HotStuff -> Paxos.
set -u
source "$(dirname "$0")/env.sh"
OUT=$1; shift
# restart the proxy as soon as it exits (used with -crash-at)
RESTART='pid=$(awk "\$1==\"proxy\"{p=\$2} END{print p}" $RUN/pids); while kill -0 $pid 2>/dev/null; do sleep 0.005; done; echo dead $(date +%s%N); PROXY_ARGS="" $SCRIPTS_DIR/cluster.sh proxy $RUN; echo up $(date +%s%N)'
for name in "$@"; do
  unset PROXY_ARGS FAULT FAULT_AT BYZ_1 BYZ_2 BYZ_3 BYZ_4 CFT_NODIGEST HS_NODIGEST CTL_F CTL_N
  unset CERT REQUIRE_CERT CTL_FB CFT_BYZ_1 CFT_BYZ_2 CFT_BYZ_3 CFT_BYZ_4
  export DUR=${DUR:-20} COUNT=${COUNT:-10} EVERY=${EVERY:-1500ms} CLIENTS=${CLIENTS:-16} KEYS=${KEYS:-200}
  case $name in
    # baseline
    base)                 ;;
    # proxy crash after the seal / after the import, then restart and recovery
    proxycrash_seal_c2b)  export PROXY_ARGS="-crash-at after-seal -crash-on 3" FAULT="$RESTART" ;;
    proxycrash_seal_b2c)  export PROXY_ARGS="-crash-at after-seal -crash-on 4" FAULT="$RESTART" ;;
    proxycrash_import_c2b) export PROXY_ARGS="-crash-at after-import -crash-on 3" FAULT="$RESTART" ;;
    proxycrash_import_b2c) export PROXY_ARGS="-crash-at after-import -crash-on 4" FAULT="$RESTART" ;;
    # the contact replica of the old instance is killed while draining / sealing
    killleader_drain_c2b) export PROXY_ARGS="-hook-at stopped -hook-on 3 -hook-cmd kill-old-leader" ;;
    killleader_seal_c2b)  export PROXY_ARGS="-hook-at drained -hook-on 3 -hook-cmd kill-old-leader" ;;
    killleader_drain_b2c) export PROXY_ARGS="-hook-at stopped -hook-on 4 -hook-cmd kill-old-leader" ;;
    killleader_seal_b2c)  export PROXY_ARGS="-hook-at drained -hook-on 4 -hook-cmd kill-old-leader" ;;
    # Byzantine HotStuff replicas
    byz_lie)              export BYZ_3=lie PROXY_ARGS="-fetch-from 2" ;;
    byz_lieok)            export BYZ_3=lie-ok ;;
    byz_corrupt)          export BYZ_3=corrupt ;;
    byz2_abort)           export BYZ_2=corrupt BYZ_3=corrupt ;;
    # directive asks for f larger than the instance supports
    quorum_f2)            export CTL_F=2 ;;
    # one safeguard disabled (unsafe modes)
    nc_noseal)            export PROXY_ARGS="-unsafe-noseal" ;;
    nc_nodigest_full)     export BYZ_3=lie PROXY_ARGS="-unsafe-noverify -fetch-from 2" CFT_NODIGEST=1 ;;
    nc_noverify_only)     export BYZ_3=lie PROXY_ARGS="-unsafe-noverify -fetch-from 2" ;;
    nc_nodigest_only)     export BYZ_3=lie PROXY_ARGS="-fetch-from 2" CFT_NODIGEST=1 ;;
    # checkpoint certification (paxos1 is the Paxos replica the proxy contacts first)
    cert_base)            export CERT=1 REQUIRE_CERT=1 CTL_FB=1 ;;
    cert_cft_lie)         export CERT=1 REQUIRE_CERT=1 CTL_FB=1 CFT_BYZ_1=fork ;;
    cert_cft_2lie)        export CERT=1 REQUIRE_CERT=1 CTL_FB=1 CFT_BYZ_2=fork CFT_BYZ_3=fork ;;
    cert_fb0)             export CERT=1 REQUIRE_CERT=1 CTL_FB=0 CFT_BYZ_1=fork ;;
    nc_leaderonly)        export CFT_BYZ_1=fork ;;   # certification off: the CFT seal is taken from the contact replica alone
    cert_multi)           export CERT=1 REQUIRE_CERT=1 CTL_FB=1 CFT_BYZ_1=fork BYZ_3=lie DUR=42 COUNT=24 ;;
    cert_scale)           export CERT=1 REQUIRE_CERT=1 CTL_FB=${CTL_FB:-$F} CTL_F=$F CTL_N=$N ;;   # e.g. N=7 F=2 or N=10 F=3 (keys generated for that N)
    *) echo "unknown scenario $name"; continue ;;
  esac
  echo "=== $name"
  "$SCRIPTS_DIR/run_switching.sh" "$OUT/$name"
done
