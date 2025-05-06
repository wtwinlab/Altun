#!/bin/bash

CONFIG_FOLDER="/users/tianwen/go/src/github.com/wtwinlab/cft_collector/bin/configs_random_gen_set"
TARGET_DIR=history_data
mkdir $TARGET_DIR
rm  benchmark.log
rm  config_RL.json
for json_file in ${CONFIG_FOLDER}/*.json; do
    if [ -f "$json_file" ]; then
        echo "apply config ${json_file}"
        cp "$json_file" config_RL.json
        file_name=config_RL.json
        # Add additional actions here, such as sending an alert or logging
        alg=$(grep algorithm ${file_name} |  cut -d \" -f 4)
        echo "run $json_file"
        echo "ok: new config generated ${alg}"
        echo "ok: new config applied chain"
        ./apply-cft.sh ${alg}
        mv cft-client.*.log ./log/
        echo "ok: cft-client benchmark"
        ./cft-client -id 1.1 -config ${file_name}
        echo "ok: benchmark done"
        TPS=$(grep "Throughput = " cft-client.*.log | cut -d = -f 2)
        LAT=$(grep "mean = " cft-client.*.log | cut -d = -f 2)
        echo "{\"TPS\":${TPS}, \"latency\": ${LAT} }" > benchmark.log
        cat benchmark.log
        
        
        # Get the count of files in the target directory
        FILE_COUNT=$(ls -1q "$TARGET_DIR" | wc -l)

        # Generate the new filename
        jname=$(basename $json_file)
        NEW_FILENAME="$jname.txt"
        cp config_RL.json $TARGET_DIR/$NEW_FILENAME
        echo "\n" >> $TARGET_DIR/$NEW_FILENAME
        cat benchmark.log >> $TARGET_DIR/$NEW_FILENAME
        rm config_RL.json 
        rm benchmark.log

    fi
done
