START_NODE=60 #59 # e.g.START_NODE=45 tests for remote address or START_NODE="127.0.0.1" for local address.
NODES_PER_WORKER=4 # because of vpaxos needs at least 2 zones
CLIENTS_PER_WORKER=8

# Check avaliable workers per server.
# echo "CPU Sockets: $(lscpu | grep 'Socket(s)' | awk '{print $2}')"
# echo "Cores per Socket: $(lscpu | grep 'Core(s) per socket' | awk '{print $4}')"
# echo "Total Threads (Logical CPUs): $(nproc --all)"

local=false
ssh_user="acount"
private_key_file="/users/account/.ssh/id_rsa"
ssh_options="-i $private_key_file -o StrictHostKeyChecking=no -o ServerAliveInterval=60"

REMOTE_DIR="/data-ssd/account/go"

