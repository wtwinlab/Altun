#!/bin/bash -e

pkill -9 -f cft
pkill -9 -f mir
pkill -9 -f bft
pkill -9 -f mir
pkill -9 -f hotstuff
sleep 1
echo "Killing previously running servers and clients"