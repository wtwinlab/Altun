#!/bin/bash

# Usage: ./update_config.sh <config_file> <num_workers> <nodes_per_worker>
# Example: ./update_config.sh config.json 3 3

if [ $# -ne 4 ]; then
    echo "Usage: $0 <config_file> <num_workers> <nodes_per_worker> <start_node>"
    echo "e.g.: ./update_config_nodes.sh config.json 4 3 70"
    exit 1
fi

CONFIG_FILE=$1
NUM_WORKERS=$2
NODES_PER_WORKER=$3
START_NODE=${4:-70}
# Check if the config file exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Error: Config file '$CONFIG_FILE' does not exist."
    exit 1
fi

# Check if jq is installed (needed for JSON processing)
if ! command -v jq &> /dev/null; then
    echo "Error: This script requires 'jq' to be installed."
    echo "Please install it with your package manager, e.g.:"
    echo "  apt-get install jq   # For Debian/Ubuntu"
    echo "  yum install jq       # For RHEL/CentOS"
    echo "  brew install jq      # For macOS with Homebrew"
    exit 1
fi

# Base ports
TCP_BASE_PORT=31735
HTTP_BASE_PORT=39080

# Generate new address configuration
generate_address_config() {
    local address_type=$1
    local base_port=$2
    local protocol=$3
    
    echo "{"
    for ((i=1; i<=NUM_WORKERS; i++)); do
        for ((j=1; j<=NODES_PER_WORKER; j++)); do
            PORT=$((base_port + j - 1))
            WORKER_ID=$(printf "%03d" $((START_NODE + i - 1)))
            # id=$((NODES_PER_WORKER * (i - 1) + j))
            # if (( id > NUM_NODES )); then
            #     break
            # fi
            # Add comma for all entries except the last one
            if [ $i -eq $NUM_WORKERS ] && [ $j -eq $NODES_PER_WORKER ]; then
                echo "      \"$i.$j\": \"$protocol://worker-$WORKER_ID:$PORT\""
            else
                echo "      \"$i.$j\": \"$protocol://worker-$WORKER_ID:$PORT\","
            fi
        done
    done
    echo "    }"
}

# Generate new configurations
TCP_CONFIG=$(generate_address_config "address" $TCP_BASE_PORT "tcp")
HTTP_CONFIG=$(generate_address_config "http_address" $HTTP_BASE_PORT "http")

# Create a temporary file for the new config
TEMP_FILE=$(mktemp)

# Read the original JSON and replace the address and http_address sections
jq --argjson tcp_config "$TCP_CONFIG" --argjson http_config "$HTTP_CONFIG" '.address = $tcp_config | .http_address = $http_config' "$CONFIG_FILE" > "$TEMP_FILE"

# Check if jq succeeded
if [ $? -ne 0 ]; then
    echo "Error: Failed to update the JSON file."
    rm "$TEMP_FILE"
    exit 1
fi

# Backup the original file
cp "$CONFIG_FILE" "${CONFIG_FILE}.bak"
echo "Original config backed up to ${CONFIG_FILE}.bak"

# Move the new config to the original file location
mv "$TEMP_FILE" "$CONFIG_FILE"
echo "Config file '$CONFIG_FILE' has been updated with the new address configuration."