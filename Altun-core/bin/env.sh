# Deployment settings shared by every script in bin/ and mir/deployment/.
# Edit this file (or export the variables before calling a script) to match the cluster.

# ---- Cluster layout ---------------------------------------------------------
# Cluster hosts are named ${WORKER_PREFIX}<index> with a three-digit, zero-padded
# index. START_NODE is the index of the first host; replica hosts come first,
# client hosts follow. START_NODE="127.0.0.1" runs every process locally.
START_NODE=${START_NODE:-1}
WORKER_PREFIX=${WORKER_PREFIX:-worker-}
NODES_PER_WORKER=${NODES_PER_WORKER:-4}     # replicas per host (VPaxos needs at least two zones)
CLIENTS_PER_WORKER=${CLIENTS_PER_WORKER:-8} # client processes per host

host_of() { printf "%s%03d" "$WORKER_PREFIX" $((START_NODE + $1 - 1)); } # 1-based host index -> host name
num_of()  { printf "%03d" $((START_NODE + $1 - 1)); }                    # 1-based host index -> zero-padded index

# ---- Remote access ----------------------------------------------------------
local=${local:-false}
ssh_user=${ssh_user:-$USER}
private_key_file=${private_key_file:-$HOME/.ssh/id_rsa}
ssh_options="-i $private_key_file -o StrictHostKeyChecking=no -o ServerAliveInterval=60"

# Working directory on every cluster host (binaries, configurations and logs are
# copied there). It must be writable and its content is replaced before each run.
# REMOTE_BASE is its parent directory; helper scripts are copied there.
REMOTE_DIR=${REMOTE_DIR:-/tmp/altun/go}
REMOTE_BASE=${REMOTE_BASE:-$(dirname "$REMOTE_DIR")}

# Directory for forensics logs. Every replica appends to a file in this directory,
# so it must be reachable from all cluster hosts (a shared file system).
FORENSICS_DIR=${FORENSICS_DIR:-$HOME/altun/forensics}

# ---- Docker/WAN emulation (bin/altunwan.sh) ---------------------------------
# ALTUNWAN=1 runs every replica and client in its own container with a netem
# qdisc that emulates the cross-region delay; ALTUNWAN=0 runs host processes.
ALTUNWAN=${ALTUNWAN:-0}
WAN_DELAY_MS=${WAN_DELAY_MS:-25}             # one-way cross-region delay applied per step
ALTUNWAN_NHOSTS=${ALTUNWAN_NHOSTS:-6}        # hosts that take part in the emulation
ALTUNWAN_IMAGE=${ALTUNWAN_IMAGE:-altunwan_base:v1}
ALTUNWAN_CPREFIX=${ALTUNWAN_CPREFIX:-altunwan_}   # container names <prefix>r1..r4 / c1..c8
ALTUNWAN_DIR=${ALTUNWAN_DIR:-$REMOTE_BASE/altunwan} # binaries staged for the containers
