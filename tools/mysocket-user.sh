#!/bin/sh
docker run -it -v $3:/root/.ssh/id_rsa.pub ghcr.io/hellt/mysocketctl:0.1.0 mysocketctl account create \
    --name "containerlab" \
    --email "$1" \
    --password "$2" \
    --sshkey ~/.ssh/id_rsa.pub