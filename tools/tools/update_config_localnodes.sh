#!/bin/bash

# Usage: ./update_config.sh <config_file> [tcp_base_port] [http_base_port]
# Example: ./update_config.sh config.json 1735 18080

if [ $# -lt 1 ] || [ $# -gt 3 ]; then
    echo "Usage: $0 <config_file> [tcp_base_port] [http_base_port]"
    echo "  config_file:   Path to your JSON configuration file"
    echo "  tcp_base_port: Optional - Base TCP port (default: 1735)"
    echo "  http_base_port: Optional - Base HTTP port (default: 18080)"
    exit 1
fi

CONFIG_FILE=$1
TCP_BASE_PORT=${2:-31735}    # Default TCP base port to 1735 if not provided
HTTP_BASE_PORT=${3:-18080}  # Default HTTP base port to 18080 if not provided

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

# Fixed number of nodes per worker (j)
NODES_PER_WORKER=3
# NUM_WORKERS=3
# Read nREPLICAS from the config file
NREPLICAS=$(jq -r '.bft_config.nREPLICAS // 0' "$CONFIG_FILE")

if [ "$NREPLICAS" -eq 0 ]; then
    echo "Error: Could not find 'bft_config.nREPLICAS' in the config file or its value is 0."
    exit 1
fi

echo "Found nREPLICAS = $NREPLICAS in config file"
echo "Using TCP base port: $TCP_BASE_PORT"
echo "Using HTTP base port: $HTTP_BASE_PORT"

# Calculate the number of workers (i) such that i*j ≤ nREPLICAS
if [ "$NREPLICAS" -eq 0 ]; then
        echo "Error: Could not find 'bft_config.nREPLICAS' in the config file or its value is 0."
        NUM_WORKERS=3
else
        echo "Use node size = $NREPLICAS in config file"
        NUM_WORKERS=$((NREPLICAS/NODES_PER_WORKER+1))
fi

# Fixed IP address
FIXED_IP="127.0.0.1"

# Generate new address configuration
generate_address_config() {
    local address_type=$1
    local base_port=$2
    local protocol=$3
    
    echo "{"
    for ((i=1; i<=NUM_WORKERS; i++)); do
        for ((j=1; j<=NODES_PER_WORKER; j++)); do
            # Port is base_port + i + j
            PORT=$((base_port + NODES_PER_WORKER*i - NODES_PER_WORKER + j - 1))
            
            # Add comma for all entries except the last one
            if [ $i -eq $NUM_WORKERS ] && [ $j -eq $NODES_PER_WORKER ]; then
                echo "      \"$i.$j\": \"$protocol://$FIXED_IP:$PORT\""
            else
                echo "      \"$i.$j\": \"$protocol://$FIXED_IP:$PORT\","
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