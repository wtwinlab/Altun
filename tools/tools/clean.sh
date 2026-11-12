#!/usr/bin/env bash
set -x

. ./env.sh

for i in $(seq 45 49); do
    pub_ip=$(printf "%03d" $((i)))
    pub="worker-${pub_ip}"
    ssh $ssh_user@$pub $ssh_options "pkill -9 -f cft && pkill -9 -f mir && pkill -9 -f bft && pkill -9 -f hotstuff && rm -rf /data-ssd/tianwen/go/* "
done