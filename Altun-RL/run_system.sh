#!/bin/bash
# Restart the Altun consensus stack with the configuration that the selector
# wrote to config_RL.json. Called by utils.py on every step in restart mode.
#
# Usage: bash run_system.sh <project_dir> <true|false>
#   project_dir  Altun-core bin directory (holds run_config.sh and the configs)
#   second arg   whether the BFT pool is active for this step

if [ -z "$1" ]; then
  echo "Error: project_dir not provided."
  echo "Usage: $0 <project_dir> <true|false>"
  exit 1
fi

if [ -z "$2" ]; then
  echo "Error: BFT flag not provided."
  echo "Usage: $0 <project_dir> <true|false>"
  exit 1
fi

project_dir=$1
bft=$2

cd "$project_dir"

echo "Stopping server and client processes..."
pkill -f cft-server
pkill -f cft-client
pkill -f bft-server
pkill -f bft-client
sleep 2

# Deploy and benchmark the protocol selected in config_RL.json
echo "RUN: run_config.sh"
bash run_config.sh

cd -
