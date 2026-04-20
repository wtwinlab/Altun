#!/bin/bash
set -x

json_file="config_RL.json"
bench_file="benchmark.log"
consensus="mirbft"

cp "config_RL_default.json" $json_file
# json_key="nodesize"
# # values=(4 8 12 16 24 32 48 64 96)
# values=(48 64 96)

# json_key="batchsize"
# values=(1 10 50 100 500 1000 5000 10000 50000 100000)
# values=(10000 20000 50000 100000)

# json_key="ratelimit"
# values=(100 200 500 1000 2000 5000 10000 50000 100000 500000 1000000)

# json_key="requests"
# values=(1000 5000 10000 50000 100000)
# 

# json_key="concurrency"
# values=(1 4 10 20 50 100 200 500 1000)

json_key="payloadsize"
values=(64 128 256 512 1024 2048 4096 8192 10240)

jq --arg val "$consensus" --arg key "algorithm" '.[$key] = $val' "$json_file" > "${json_file}.tmp" && mv -f "${json_file}.tmp" "$json_file"
  
echo "id,TPS,latency,consensus,$json_key" >> exp-$json_key.txt
for size in "${values[@]}"; do
  echo "Setting: $size"
  rm -f $bench_file
  jq --argjson val "$size" --arg key "$json_key" '.[$key] = $val' "$json_file" > tmp.json && mv tmp.json "$json_file"
  cat "$json_file"

  ./run_config.sh config_RL.json

  if [ -f "$bench_file" ]; then
    tps=$(jq -r '.TPS' "$bench_file")
    lat=$(jq -r '.latency' "$bench_file")
    echo "0,$tps,$lat,$consensus,$size" >> exp-$json_key.txt
  fi
  
done

cat exp-$json_key.txt