#!/usr/bin/env bash

set -eo pipefail

protoc -I=./ --go_out=$GOPATH/src/ ./*.proto



