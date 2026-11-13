#!/bin/bash -e

# echo "Killing previously running servers and clients"
# pkill mir-server
# pkill mir-client


echo "Starting server"
./bin/mir-server ./config/serverconfig/config.yml server &> server.out &