#!/usr/bin/env bash

set -eo pipefail

protoc -I=./types/proto --go_out=$GOPATH/src/ ./types/proto/*



