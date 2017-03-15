#!/bin/bash

/usr/sbin/setenforce 0

# Get all the deps in
yum -y install \
    docker \
    make \
    git \
    curl

service docker start

make docker-build-build
make docker-install
make docker-test
make docker-build
make docker-build-run
make docker-run-deploy