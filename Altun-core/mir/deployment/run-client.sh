#!/bin/bash -e

# echo "Killing previously running servers and clients"
# pkill mir-server
# pkill mir-client

echo "Removing previously created status files"
rm -rf status

echo "Starting client"
./bin/mir-client ./config/clientconfig/config.yml client &> client.out &

