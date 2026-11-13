#!/bin/bash
# Stop every consensus process started from <project_dir>.
# Usage: bash kill.sh <project_dir>

project_dir=$1
cd "$project_dir"

echo "Stopping server and client processes..."
pkill -f cft
pkill -f mir
sleep 2

cd -
