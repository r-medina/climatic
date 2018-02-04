#!/usr/bin/env bash

set -e

BASE=$GOPATH/src
CLIMA=github.com/r-medina/climatic

for d in $(find "$BASE/$CLIMA" -name "*.proto" | xargs -n1 dirname | uniq); do
    protoc -I$BASE --go_out=plugins=grpc:$BASE $d/*.proto
done
