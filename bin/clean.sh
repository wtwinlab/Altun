#!/usr/bin/env bash
#set -x

dir=$(dirname "${BASH_SOURCE[0]}")

rm -f ${dir}/cft-server.*.log
rm -f ${dir}/cft-client*.log
# mv ${dir}/cft-client*.log ${dir}//log/  > /dev/null 2>&1
rm -f ${dir}/benchmark.log