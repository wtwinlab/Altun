#!/usr/bin/env bash

PID_FILE=cft-server.pid

PID=$(cat "${PID_FILE}");

if [ -z "${PID}" ]; then
    echo "Process id for cft-servers is written to location: {$PID_FILE}"
    go build ../cft-server/
    go build ../cft-client/
    go build ../cmd/
    ./cft-server -sim=true -log_dir=. -log_level=debug -algorithm=paxos &
    echo $! >> ${PID_FILE}
else
    echo "cft-servers are already started in this folder."
    exit 0
fi
