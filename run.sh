#!/bin/bash

set -e

PID=$(lsof -t +D "/Users/jocham/.kube-watcher/" 2>/dev/null)
if [ -z "$PID" ]; then
    # Fallback: check by process name if lsof didn't catch it
    PID=$(pgrep -f "kube-watcher" | grep -v $$)
fi

if [ -n "$PID" ]; then
    echo "Stopping existing kube-watcher (PID: $PID)..."
    kill -9 $PID
    sleep 1
fi

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
cd "$DIR"


if [ ! -f "./bin/kube-watcher" ] || [ "./cmd/main.go" -nt "./bin/kube-watcher" ]; then
    echo "Building kube-watcher..."
    go build -o ./bin/kube-watcher ./cmd/main.go
fi

echo "Starting Kube Watcher MCP Server..."
./bin/kube-watcher
