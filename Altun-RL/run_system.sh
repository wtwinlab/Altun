#!/bin/bash

function kill_port_process() {
    # ports=(
    #     8080 8081 8082 8083 8084 8085 8086 8087 8088
    #     11735 11736 11737 11738 11739 11740 11741 11742 11743
    # )
    ports=(
        1735 1736 1737 1738 1739 1740 1741 1742 1743
        18080 18081 18082 18083 18084 18085 18086 18087 18088
    )


    for port in "${ports[@]}"; do
         # Find the PID(s) using the port
        pids=$(lsof -t -i tcp:$port 2>/dev/null)

        if [ -n "$pids" ]; then
            for pid in $pids; do
                # Find the parent PID
                ppid=$(ps -o ppid= -p $pid | tr -d ' ')

                echo "Killing process PID $pid and its parent PID $ppid using port $port"

                # Kill the process
                kill -9 $pid

                # Check if parent PID is not 1 (init process) before killing
                if [ "$ppid" -ne 1 ]; then
                    kill -9 $ppid 2>/dev/null
                fi
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

# 检查是否提供了第二个参数
if [ -z "$1" ]; then
  echo "Error: No first argument provided."
  echo "Usage: $0  project_dir <arg2>"
  exit 1
fi

# 检查是否提供了第二个参数
if [ -z "$2" ]; then
  echo "Error: No second argument provided."
  echo "Usage: $0 <arg1> <true/false>"
  exit 1
fi

project_dir=$1
bft=$2


cd $project_dir

echo "Killing server and client processes..."
pkill -f cft-server
pkill -f cft-client
pkill -f bft-server
pkill -f bft-client
sleep 2
# Kill processes using the target ports
# kill_port_process

# Wait until all ports are free
# wait_for_ports


# run system with config_RL.json
echo "RUN: run_config.sh"
bash run_config.sh #>> python_rl_log.txt 2>&1


cd -
