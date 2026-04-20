#!/bin/bash
#set -x
# File to be monitored
FILE_TO_MONITOR="config_RL.json"

# Interval between checks (in seconds)
CHECK_INTERVAL=3

# Get the initial modification time
if [ ! -f "$FILE_TO_MONITOR" ]; then
  echo "File init"
  cp config.json ${FILE_TO_MONITOR}
fi
version=0


if [ ! -f "$FILE_TO_MONITOR" ]; then
    echo "$FILE_TO_MONITOR does not exist. Assigning the current time."
    LAST_MODIFICATION_TIME=$(date +%s)
else
    LAST_MODIFICATION_TIME=$(stat -c %Y "$FILE_TO_MONITOR")
fi

echo $LAST_MODIFICATION_TIME
echo "Monitoring $FILE_TO_MONITOR for changes..."

# Infinite loop to check for changes
while true; do
  # Wait until the file exists
  while [ ! -f "$FILE_TO_MONITOR" ]; do
    echo "Waiting for $FILE_TO_MONITOR to be created..."
    sleep 1  # Check every 1 second (you can adjust this interval)
  done

  CURRENT_MODIFICATION_TIME=$(stat -c %Y "$FILE_TO_MONITOR")
  if [ "$CURRENT_MODIFICATION_TIME" -ne "$LAST_MODIFICATION_TIME" ]; then
    echo "The file '$FILE_TO_MONITOR' was modified at $(date)."
    LAST_MODIFICATION_TIME=$CURRENT_MODIFICATION_TIME
    # Add additional actions here, such as sending an alert or logging
    alg=$(grep algorithm ${FILE_TO_MONITOR} |  cut -d \" -f 4)
    echo "ok: new config generated ${alg}"
    echo "ok: new config applied chain"
    rm cft-server.*.log
     mv cft-client.*.log ./log/
    ./apply-cft.sh ${alg}
   # mv cft-client.*.log ./log/
    echo "ok: cft-client benchmark"
    ./cft-client -id 1.1 -config ${FILE_TO_MONITOR}
    echo "ok: benchmark done"
    TPS=$(grep "Throughput = " cft-client.*.log | cut -d = -f 2)
    LAT=$(grep "mean = " cft-client.*.log | cut -d = -f 2)
    echo "{\"TPS\":${TPS}, \"latency\": ${LAT} }" > benchmark.log
    #echo ((++version)) > RL_version
    cat benchmark.log
  fi

  # Sleep for the defined interval before checking again
  sleep "$CHECK_INTERVAL"
done

