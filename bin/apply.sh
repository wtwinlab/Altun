#!/usr/bin/env bash
#set -x

dir=$(dirname "${BASH_SOURCE[0]}")
DEFAULT_CONFIG_FILE="${dir}/config.json"
echo $DEFAULT_CONFIG_FILE

ppkill () {
        pids=( $(pgrep -f $1) )
        for pid in "${pids[@]}"; do
                if [[ $pid != $$ ]]; then
                        kill "$pid"
                fi
        done
}


ppkill cft-server
ppkill cft-client
rm -f ${dir}/cft-server.*.log
rm -f ${dir}/cft-client*.log
# mv ${dir}/cft-client*.log ${dir}//log/  > /dev/null 2>&1
rm -f ${dir}/benchmark.log

defaultAlg=$(grep algorithm ${DEFAULT_CONFIG_FILE} |  cut -d \" -f 4)
alg=${1:-${defaultAlg}}

echo "ok: new config generated ${alg}"
./cft-server -id 1.1 -algorithm=${alg} &
./cft-server -id 1.2 -algorithm=${alg} &
./cft-server -id 1.3 -algorithm=${alg} &
./cft-server -id 2.1 -algorithm=${alg} &
./cft-server -id 2.2 -algorithm=${alg} &
./cft-server -id 2.3 -algorithm=${alg} &
./cft-server -id 3.1 -algorithm=${alg} &
./cft-server -id 3.2 -algorithm=${alg} &
./cft-server -id 3.3 -algorithm=${alg} & 









