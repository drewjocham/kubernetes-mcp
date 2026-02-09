#!/bin/bash

# Start script for Kube Watcher MCP Server
set -e

# Get our directory where this script is located
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
cd "$DIR"


# Build if binary doesn't exist or source is newer
if [ ! -f "./bin/kube-watcher" ] || [ "./cmd/main.go" -nt "./bin/kube-watcher" ]; then
    echo "Building kube-watcher..."
    go build -o ./bin/kube-watcher ./cmd/main.go
fi

echo "Starting Kube Watcher MCP Server..."
./bin/kube-watcher
