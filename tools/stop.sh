#!/bin/bash

pkill -9 -f cft
pkill -9 -f bft
pkill -9 -f mir
pkill -9 -f hotstuff
echo "Killing previously running servers and clients"