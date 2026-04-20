#!/usr/bin/env bash
dir=$(dirname "${BASH_SOURCE[0]}")
# $(pwd)
go build -o cft-server ${dir}/../cmd/cft-server/
go build -o cft-client ${dir}/../cmd/cft-client/
go build -o cft-cmd ${dir}/../cmd/cmd