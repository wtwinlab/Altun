#!/bin/bash
set -x

# BFT env parameters
nconsensus="fasthotstuff simplehotstuff chainedhotstuff"
nREPLICAS="4 8 16 32 64"
nCLIENTS="1 10 100 500 1000 5000 10000"
nconcurrency="1 10 100 500 1000 5000 10000 100000"
nratelimit="10 100 500 1000 5000 10000 100000"
nbatchsize="1 10 100 500 1000 5000 10000 "
npayloadsize="0 128 256 512 1024 2048"
nleaderrotation="round-robin fixed"

alg="chainedhotstuff"
concurrency=500
replicas=4
clients=1
batchsize=1
payloadsize=128
leaderrotation="fixed"
ratelimit=""

resultfile="./output/t-l.csv"

mkdir -p ./benchmark
# 78965
# ./bft run --consensus chainedhotstuff --max-concurrent 10000 --replicas 4 --clients 4 --batch-size 1000 --payload-size 0 --leader-rotation fixed --measurement-interval 10s --duration 15s
# 43433
# ./bft run --consensus chainedhotstuff --max-concurrent 1000 --replicas 4 --clients 4 --batch-size 1000 --payload-size 0 --leader-rotation fixed --measurement-interval 10s --duration 15s
# 1599
#./bft run --consensus chainedhotstuff --max-concurrent 1000 --replicas 4 --clients 1 --batch-size 1000 --payload-size 0 --leader-rotation fixed --measurement-interval 10s --duration 15s

# 3690 
# Default
# ./bft run --consensus chainedhotstuff --max-concurrent 500 --replicas 4 --clients 4 --batch-size 1000 --payload-size 0 --leader-rotation fixed --measurement-interval 10s --duration 15s
# ./bft run --consensus chainedhotstuff --max-concurrent 500 --replicas 4 --clients 4 --batch-size 1000 --payload-size 128 --leader-rotation fixed --measurement-interval 10s --duration 15s
# error
# ./bft run --consensus chainedhotstuff --max-concurrent 500 --replicas 4 --clients 4 --batch-size 100 --payload-size 0 --leader-rotation fixed --measurement-interval 10s --duration 15s
# 708
# ./bft run --consensus chainedhotstuff --max-concurrent 500 --replicas 4 --clients 4 --batch-size 1 --payload-size 0 --leader-rotation fixed --measurement-interval 10s --duration 15s
# 694
# ./bft run --consensus chainedhotstuff --max-concurrent 500 --replicas 4 --clients 4 --batch-size 1 --payload-size 0 --leader-rotation fixed --measurement-interval 10s --duration 15s



for CNS in ${nconsensus}; do
    echo "Experiments for ${CNS}"
    # ./apply-bft.sh ${CNS} ${concurrency} ${replicas} ${clients} ${batchsize} ${payloadsize} ${leaderrotation} 
    #head -n 2 ${resultfile} >> "nconsensus.csv" 



    echo "Experiment1: ${nCLIENTS}"
    #echo "Throughput (commands/second),Latency (ms)" > "nclients.csv" 
    for TH in $nCLIENTS; do
        echo "./apply-bft.sh ${CNS} ${concurrency} ${replicas} ${TH} ${batchsize} ${payloadsize} ${leaderrotation} "
        ./apply-bft.sh ${CNS} ${concurrency} ${replicas} ${TH} ${batchsize} ${payloadsize} ${leaderrotation} 
        #head -n 2 ${resultfile} >> "nclients.csv" 
    done


    echo "Experiment2: ${nREPLICAS}"
    #echo "Throughput (commands/second),Latency (ms)" > "nreplicas.csv" 
    for TH in $nREPLICAS; do
        echo "./apply-bft.sh ${CNS} ${concurrency} ${TH} ${clients} ${batchsize} ${payloadsize} ${leaderrotation} "
        ./apply-bft.sh ${CNS} ${concurrency} ${TH} ${clients} ${batchsize} ${payloadsize} ${leaderrotation} 
        #head -n 2 ${resultfile} >> "nreplicas.csv" 
    done


    echo "Experiment3: ${nconcurrency}"
    #echo "Throughput (commands/second),Latency (ms)" > "nconcurrency.csv" 
    for TH in $nconcurrency; do
        echo "./apply-bft.sh ${CNS} ${TH} ${replicas} ${clients} ${batchsize} ${payloadsize} ${leaderrotation} "
        ./apply-bft.sh ${CNS} ${TH} ${replicas} ${clients} ${batchsize} ${payloadsize} ${leaderrotation} 
        #head -n 2 ${resultfile} >> "nconcurrency.csv" 
    done

     echo "Experiment3: ${nconcurrency}"
    #echo "Throughput (commands/second),Latency (ms)" > "nconcurrency.csv" 
    for TH in $nconcurrency; do
        batchsize=1000
        echo "./apply-bft.sh ${CNS} ${TH} ${replicas} ${clients} ${batchsize} ${payloadsize} ${leaderrotation} "
        ./apply-bft.sh ${CNS} ${TH} ${replicas} ${clients} ${batchsize} ${payloadsize} ${leaderrotation} 
        #head -n 2 ${resultfile} >> "nconcurrency.csv" 
    done

    echo "Experiment4: ${nleaderrotation}"
    #echo "Throughput (commands/second),Latency (ms)" > "nleaderrotation.csv" 
    for TH in ${nleaderrotation}; do
        echo "./apply-bft.sh ${CNS} ${concurrency} ${replicas} ${clients} ${batchsize} ${payloadsize} ${TH} "
        ./apply-bft.sh ${CNS} ${concurrency} ${replicas} ${clients} ${batchsize} ${payloadsize} ${TH} 
        #head -n 2 ${resultfile} >> "nleaderrotation.csv" 
    done

    echo "Experiment5: ${nbatchsize}"
    #echo "Throughput (commands/second),Latency (ms)" > "nbatchsize.csv" 
    for TH in $nbatchsize; do
        echo "./apply-bft.sh ${CNS} ${concurrency} ${replicas} ${clients} ${TH} ${payloadsize} ${leaderrotation} "
        ./apply-bft.sh ${CNS} ${concurrency} ${replicas} ${clients} ${TH} ${payloadsize} ${leaderrotation} 
        #head -n 2 ${resultfile} >> "nbatchsize.csv" 
    done


    echo "Experiment6: ${nratelimit}"
    #echo "Throughput (commands/second),Latency (ms)" > "nratelimit.csv" 
    for TH in $nratelimit; do
        echo "./apply-bft.sh ${CNS} ${concurrency} ${replicas} ${clients} ${batchsize} ${payloadsize} ${leaderrotation} ${TH}"
        ./apply-bft.sh ${CNS} ${concurrency} ${replicas} ${clients} ${batchsize} ${payloadsize} ${leaderrotation} ${TH}
        #head -n 2 ${resultfile} >> "nratelimit.csv" 
    done

    echo "Experiment7: ${npayloadsize}"
    #echo "Throughput (commands/second),Latency (ms)" > "npayloadsize.csv" 
    for TH in ${npayloadsize}; do
        echo "./apply-bft.sh ${CNS} ${concurrency} ${replicas} ${clients} ${batchsize} ${TH} ${leaderrotation} "
        ./apply-bft.sh ${CNS} ${concurrency} ${replicas} ${clients} ${batchsize} ${TH} ${leaderrotation} 
        #head -n 2 ${resultfile} >> "npayloadsize.csv" 
    done

done