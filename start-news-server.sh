#!/bin/bash
#
# Liberation News - Start News Server
# ====================================
# This script starts the Liberation News server.
# It will automatically connect to bootstrap servers on the network.
#

set -e

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║                                                           ║"
echo "║        LIBERATION NEWS - SERVER LAUNCHER                  ║"
echo "║                                                           ║"
echo "╚═══════════════════════════════════════════════════════════╝"
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "ERROR: Go is not installed."
    echo ""
    echo "Please install Go first:"
    echo "  - macOS: brew install go"
    echo "  - Linux: sudo apt install golang-go"
    echo "  - Windows: Download from https://golang.org/dl/"
    echo ""
    exit 1
fi

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# Check for JWT secret
if [ -z "$NEWS_AUTH_JWT_SECRET" ]; then
    # Generate a random secret if not set
    export NEWS_AUTH_JWT_SECRET=$(openssl rand -base64 32 2>/dev/null || head -c 32 /dev/urandom | base64)
    echo "Generated temporary JWT secret (set NEWS_AUTH_JWT_SECRET for production)"
fi

# Build the server if needed
SERVER_BIN="$SCRIPT_DIR/server"
if [ ! -f "$SERVER_BIN" ] || [ "$1" = "--rebuild" ]; then
    echo "Building news server..."
    go build -o server ./cmd/server
    echo "Build complete!"
    echo ""
fi

# Show helpful info
echo "Starting Liberation News Server..."
echo ""
echo "Configuration:"
echo "  - Web UI:    http://localhost:12345"
echo "  - API:       http://localhost:12345/api/v1"
echo "  - P2P Port:  4001"
echo ""
echo "The server will automatically discover and connect to other nodes."
echo ""

# Start the server
exec "$SERVER_BIN"
