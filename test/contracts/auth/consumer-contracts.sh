#!/bin/bash

. ./setenv.sh

# run test
go test -v -run 'Test*'
