#!/bin/bash


# if [ ! -d /data-ssd/tianwen/workers/worker0${1}_Altun/Altun-core ]; then
    [ -d /data-ssd/tianwen/workers/worker0${1}_Altun/ ] | mkdir -p  /data-ssd/tianwen/workers/worker0${1}_Altun/
    rm -rf /data-ssd/tianwen/workers/worker0${1}_Altun/Altun-core/
    cp -rf /users/tianwen/go/src/github.com/wtwinlab/Altun-core/  /data-ssd/tianwen/workers/worker0${1}_Altun/
# fi

