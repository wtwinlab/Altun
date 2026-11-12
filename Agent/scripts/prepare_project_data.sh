#!/bin/bash
for i in {70..89}; do
echo "COPY WORKER ${i}"
ssh worker-0${i}  "/users/tianwen/go/src/github.com/wtwinlab/Altun_RL_bft/Altun_RL/check_and_copy.sh ${i}"
done