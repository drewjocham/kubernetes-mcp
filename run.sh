#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
cd "$DIR"


if [ ! -f "./bin/kube-watcher" ] || [ "./cmd/main.go" -nt "./bin/kube-watcher" ]; then
    echo "Building kube-watcher..."
    go build -o ./bin/kube-watcher ./cmd/main.go
fi

echo "Starting Kube Watcher MCP Server..."
./bin/kube-watcher
