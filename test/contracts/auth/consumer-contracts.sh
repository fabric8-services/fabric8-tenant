#!/bin/bash

. ./setenv.sh

# run test
go test -v -run 'Test*'
TEST_EXIT=$?

if [ "$TEST_EXIT" == "0" ]; then
    ./publish-contracts.sh
fi