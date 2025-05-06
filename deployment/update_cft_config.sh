#!/bin/bash

# Function to check if jq is installed
check_jq_installed() {
    if ! command -v jq &> /dev/null
    then
        echo "jq could not be found, please install it first."
        exit 1
    fi
}

is_key_in_benchmark() {
    local config_file=$1
    local key=$2
    
    # Use jq to check if the key exists in the benchmark object
    if jq -e ".benchmark | has(\"$key\")" "$config_file" >/dev/null; then
        return 0  # Key exists in benchmark
    else
        return 1  # Key does not exist in benchmark
    fi
}
# Function to update JSON value
update_json_value() {
    local config_file=$1
    local key=$2
    local value=$3
    
    # Check if value is a number or a string
    if [[ "$value" =~ ^-?[0-9]+(\.[0-9]+)?$ ]]; then
        # If value is a number (including float)
        jq --arg key "$key" --argjson value "$value" \
           '.[$key] = $value' "$config_file" > tmp.json && mv tmp.json "$config_file"
    else
        # If value is a string
        jq --arg key "$key" --arg value "$value" \
           '.[$key] = $value' "$config_file" > tmp.json && mv tmp.json "$config_file"
    fi
}

# Function to update nested JSON object (for benchmark-related updates)
update_nested_json_value() {
    local config_file=$1
    local path=$2
    local value=$3
    
    # Split path into components
    IFS='.' read -ra path_parts <<< "$path"
    
    # Build jq filter string
    local filter="."
    for part in "${path_parts[@]}"; do
        filter+="[\"$part\"]"
    done
    
    # Check if value is a number or a string
    if [[ "$value" =~ ^-?[0-9]+(\.[0-9]+)?$ ]]; then
        # If value is a number (including float)
        jq --argjson value "$value" \
           "$filter = \$value" "$config_file" > tmp.json && mv tmp.json "$config_file"
    else
        # If value is a string
        jq --arg value "$value" \
           "$filter = \$value" "$config_file" > tmp.json && mv tmp.json "$config_file"
    fi
}

# Function to update JSON value for a benchmark setting
update_benchmark() {
    local config_file=$1
    local param=$2
    local value=$3

    # Update the benchmark settings inside the config.json
    update_nested_json_value "$config_file" "benchmark.$param" "$value"
}

# Function to update the value in config file
update_config_value() {
    local config_file=$1
    local key=$2
    local value=$3

    # Check if jq is installed
    check_jq_installed

    # Update the corresponding key with the new value
    if is_key_in_benchmark "$config_file" "$key"; then
            update_benchmark "$config_file" "$key" "$value"
    else
        # Otherwise update the value directly
        update_json_value "$config_file" "$key" "$value"
    fi

    echo "Updated $key with value: $value in $config_file"
}


# Function to update node addresses in a JSON config file
update_node_address() {
    local config_file=${1:-"config_cft.json"}
    local nreplicas=${2:-4}
    local NODES_PER_WORKER=${3:-2}
    local node_address=${4:-"127.0.0.1"}
    local tcp_base_port=${5:-31735}
    local http_base_port=${6:-18080}
    
    local num_workers=$((nreplicas / NODES_PER_WORKER + (nreplicas % NODES_PER_WORKER == 0 ? 0 : 1)))
    echo "Using node_address = $node_address in config file"
    echo "Using TCP base port: $tcp_base_port"
    echo "Using HTTP base port: $http_base_port"

    # if [ -z "$config_file" ] || [ -z "$nreplica" ]; then
    #     echo "Usage: update_node_address <config_file> <nodesize> [tcp_base_port] [http_base_port]"
    #     return 1
    # fi

    if [ ! -f "$config_file" ]; then
        echo "Error: Config file '$config_file' does not exist."
        return 1
    fi

        

    # Generate local address configuration
    generate_local_address_config() {
        local base_port=$1
        local protocol=$2
        local fixed_ip="127.0.0.1"
        echo "{"
        for ((i=1; i<=num_workers; i++)); do
            for ((j=1; j<=NODES_PER_WORKER; j++)); do
                local port=$((base_port + NODES_PER_WORKER * (i - 1) + j - 1))
                if [ $i -eq $num_workers ] && [ $j -eq $NODES_PER_WORKER ]; then
                    echo "  \"$i.$j\": \"$protocol://$fixed_ip:$port\""
                else
                    echo "  \"$i.$j\": \"$protocol://$fixed_ip:$port\"," 
                fi
            done
        done
        echo "}"
    }

    generate_remote_address_config() {
        local START_NODE=$1
        local protocol=$2
        local base_port=$3
        echo "{"
        for ((i=1; i<=num_workers; i++)); do
            pub_ip=$(printf "%03d" $((START_NODE + i - 1)))
            for ((j=1; j<=NODES_PER_WORKER; j++)); do
                local port=$((base_port + j - 1))
                if [ $i -eq $num_workers ] && [ $j -eq $NODES_PER_WORKER ]; then
                    echo "  \"$i.$j\": \"$protocol://worker-$pub_ip:$port\""
                else
                    echo "  \"$i.$j\": \"$protocol://worker-$pub_ip:$port\"," 
                fi
            done
        done
        echo "}"
    }

    if [[ "${node_address}" == "127.0.0.1" ]]; then
        echo "Generate local server address config: ${node_address}"
        tcp_config=$(generate_local_address_config $tcp_base_port "tcp")
        http_config=$(generate_local_address_config $http_base_port "http")
    else
        echo "Generate remoter server address config: ${node_address}"
        tcp_config=$(generate_remote_address_config ${node_address} "tcp" ${tcp_base_port})
        http_config=$(generate_remote_address_config ${node_address} "http" ${http_base_port})
    fi



    local temp_file=$(mktemp)

    # Update the JSON using jq
    jq --argjson tcp_config "$tcp_config" --argjson http_config "$http_config" '.address = $tcp_config | .http_address = $http_config' "$config_file" > "$temp_file"

    if [ $? -ne 0 ]; then
        echo "Error: Failed to update the JSON file."
        rm "$temp_file"
        return 1
    fi

    mv "$temp_file" "$config_file"
    echo "Config file '$config_file' has been updated with the new address configuration."
}

# Example usage: update_node_address config.json 3 1735 18080
# update_node_address config_cft.json 4 2
# Example of using the function
# Uncomment the following line to test the function
# update_config_value "config_cft.json" "algorithm" "vpaxos"
# update_config_value "config_cft.json" "W" "0.7"
# update_config_value "config_cft.json" "N" "700000"
# update_config_value "config.json" "policy" ${leaderrotation}
# update_config_value "config.json" "batch-size ${batchsize} 
# update_config_value "config.json" "payload-size ${payloadsize} 
# update_config_value "config.json" "clients ${clients}
# update_config_value "config.json" "N" "${replicas}"
