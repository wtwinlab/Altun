#!/usr/bin/env bash

appname="${1:-"cft-server"}"

pids=( $(pgrep -f `${appname}`) )
for pid in "${pids[@]}"; do
  if [[ $pid != $$ ]]; then
    kill "$pid"
  fi
done