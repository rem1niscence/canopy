#!/usr/bin/env bash

set -eo pipefail

protoc -I=./proto --go_out=$GOPATH/src/ ./proto/*



