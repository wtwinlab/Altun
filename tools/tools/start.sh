#!/usr/bin/env bash

PID_FILE=cft-server.pid

./cft-server -log_dir=. -log_level=info -id $1 -algorithm $2 -fz=0 &
echo $! >> ${PID_FILE}
