#!/usr/bin/env bash

PID_FILE=cft-server.pid

PID=$(cat "${PID_FILE}");

if [ -z "${PID}" ]; then
    echo "Process id for cft-servers is written to location: {$PID_FILE}"
    go build ../cft-server/
    go build ../cft-client/
    go build ../cmd/
    ./cft-server -log_dir=. -log_level=debug -id 1.1 &
    echo $! >> ${PID_FILE}
    ./cft-server -log_dir=. -log_level=debug -id 1.2 &
    echo $! >> ${PID_FILE}
    ./cft-server -log_dir=. -log_level=debug -id 1.3 &
    echo $! >> ${PID_FILE}
    ./cft-server -log_dir=. -log_level=debug -id 2.1 &
    echo $! >> ${PID_FILE}
    ./cft-server -log_dir=. -log_level=debug -id 2.2 &
    echo $! >> ${PID_FILE}
    ./cft-server -log_dir=. -log_level=debug -id 2.3 &
    echo $! >> ${PID_FILE}
    ./cft-server -log_dir=. -log_level=debug -id 3.1 &
    echo $! >> ${PID_FILE}
    ./cft-server -log_dir=. -log_level=debug -id 3.2 &
    echo $! >> ${PID_FILE}
    ./cft-server -log_dir=. -log_level=debug -id 3.3 &
    echo $! >> ${PID_FILE}
else
    echo "cft-servers are already started in this folder."
    exit 0
fi
