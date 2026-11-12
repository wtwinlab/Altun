#!/bin/bash

function kill_port_process() {
    ports=(
        1735 1736 1737 1738 1739 1740 1741 1742 1743
        18080 18081 18082 18083 18084 18085 18086 18087 18088
    )


    for port in "${ports[@]}"; do
         # Find the PID(s) using the port
        pids=$(lsof -t -i tcp:$port 2>/dev/null)

        if [ -n "$pids" ]; then
            for pid in $pids; do     
                # Kill the process
                kill -9 $pid
            done
        fi
        echo "Killing port ${port}"
        lsof -i tcp:${port} | awk 'NR!=1 {print $2}' | xargs kill -9 2>/dev/null
    done
}

function wait_for_ports() {
    ports=(
        1735 1736 1737 1738 1739 1740 1741 1742 1743
        18080 18081 18082 18083 18084 18085 18086 18087 18088
    )


    for port in "${ports[@]}"; do
        while lsof -i tcp:$port >/dev/null 2>&1; do
            echo "Waiting for port $port to be free..."
            sleep 1
        done
        echo "Port $port is now free."
    done
}

project_dir=$1
cd $project_dir

echo "Killing server and client processes..."
pkill -f cftserver
pkill -f cftclient
sleep 2
# Kill processes using the target ports
kill_port_process

# Wait until all ports are free
wait_for_ports



cd -
