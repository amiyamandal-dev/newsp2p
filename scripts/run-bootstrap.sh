#!/bin/bash
# Liberation News - Bootstrap Server Launcher
# This script starts the dedicated bootstrap server for the P2P network

set -e

# Default values
P2P_PORT=${P2P_PORT:-4001}
HTTP_PORT=${HTTP_PORT:-8081}
DATA_DIR=${DATA_DIR:-./data/bootstrap}

echo "==================================="
echo "Liberation News Bootstrap Server"
echo "==================================="
echo ""
echo "P2P Port:  $P2P_PORT"
echo "HTTP Port: $HTTP_PORT"
echo "Data Dir:  $DATA_DIR"
echo ""

# Build if needed
if [ ! -f "./bootstrap" ] || [ "$1" = "--rebuild" ]; then
    echo "Building bootstrap server..."
    go build -o bootstrap ./cmd/bootstrap
    echo "Build complete."
    echo ""
fi

# Run the bootstrap server
./bootstrap \
    -p2p-port=$P2P_PORT \
    -http-port=$HTTP_PORT \
    -data-dir=$DATA_DIR \
    -rendezvous=liberation-news-network
