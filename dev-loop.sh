#!/bin/bash

# Exit on error
set -e

# Create FIFO if it doesn't exist
FIFO="/tmp/sonoserve-dev-loop.fifo"
if [[ ! -p "$FIFO" ]]; then
    mkfifo "$FIFO"
fi

echo "Development loop started. Send 'rebuild' to $FIFO to trigger rebuild."
echo "Example: echo rebuild > $FIFO"

while true; do
    # Clear log files
    > sonoserve.stderr.log
    
    echo "Building project..."
    echo "rebuilding" > server.status
    make build
    
    # Start server in background
    echo "Starting server..."
    ./bin/sonoserve 2>sonoserve.stderr.log &
    SERVER_PID=$!
    
    # Wait for server to be ready
    echo "Waiting for server to be ready..."
    while ! curl -s http://localhost:8080/health >/dev/null 2>&1; do
        sleep 0.5
    done
    echo "Server is ready (PID: $SERVER_PID)"
    echo "ready" > server.status
    
    # Wait for rebuild command
    echo "Waiting for rebuild command..."
    read command < "$FIFO"
    
    if [[ "$command" == "rebuild" ]]; then
        echo "Rebuild requested, stopping server..."
        echo "rebuilding" > server.status
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
        echo "Server stopped, restarting loop..."
    else
        echo "Unknown command: $command"
    fi
done