#!/bin/bash

set -e

SCRIPT_HOME=$( cd "$( dirname "$0" )" && pwd )

cd "${SCRIPT_HOME}"/..

ARCHS="amd64 386"
OSS="darwin linux windows"
BINS="climasrv climactl"


for bin in $BINS; do
    for arch in $ARCHS; do
	for os in $OSS; do
	    echo building $os binary on $arch...
	    go build -o bin/$bin.$os-$arch ./cmd/$bin/*.go
	done
    done
done
