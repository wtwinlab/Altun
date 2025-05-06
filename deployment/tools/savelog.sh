#!/usr/bin/env bash

num=$1
cp config.json exp${num}.log
tail -10 cft-client.*.log >> exp${num}.log

