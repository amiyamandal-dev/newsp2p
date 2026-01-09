#!/bin/bash
#
# Liberation News - Start Bootstrap Server
# =========================================
# This script starts the bootstrap server that helps news nodes find each other.
# Just run this script - no configuration needed!
#

set -e

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║                                                           ║"
echo "║     LIBERATION NEWS - BOOTSTRAP SERVER LAUNCHER           ║"
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

# Build the bootstrap server if needed
BOOTSTRAP_BIN="$SCRIPT_DIR/bootstrap"
if [ ! -f "$BOOTSTRAP_BIN" ] || [ "$1" = "--rebuild" ]; then
    echo "Building bootstrap server..."
    go build -o bootstrap ./cmd/bootstrap
    echo "Build complete!"
    echo ""
fi

# Start the bootstrap server
echo "Starting bootstrap server..."
echo ""

exec "$BOOTSTRAP_BIN"
