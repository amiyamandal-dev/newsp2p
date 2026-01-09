# Liberation News - Quick Start Guide

Welcome! This guide will help you get started with Liberation News in just a few steps.

## What You Need

- A computer (Mac, Linux, or Windows)
- Go programming language installed ([download here](https://golang.org/dl/))

## Option 1: Quick Start (Recommended)

### Step 1: Start the Bootstrap Server (on one computer)

The bootstrap server helps other news servers find each other. You only need ONE bootstrap server for your network.

```bash
# Make the script executable (first time only)
chmod +x start-bootstrap.sh

# Start the bootstrap server
./start-bootstrap.sh
```

You'll see output like this:
```
╔═══════════════════════════════════════════════════════════╗
║          LIBERATION NEWS - BOOTSTRAP SERVER               ║
╚═══════════════════════════════════════════════════════════╝

  Peer ID: 12D3KooWxxxxx...

  Share these addresses with other nodes:
    /ip4/192.168.1.100/tcp/4001/p2p/12D3KooWxxxxx...
```

**Copy one of the addresses** - you'll share this with other people.

### Step 2: Start the News Server

On any computer that wants to join the network:

```bash
# Make the script executable (first time only)
chmod +x start-news-server.sh

# Start the news server
./start-news-server.sh
```

Open your browser and go to: **http://localhost:12345**

That's it! Your news server will automatically find and connect to other nodes.

## Option 2: Using Docker (Even Easier!)

If you have Docker installed:

```bash
# Start everything with one command
docker-compose up -d

# View logs
docker-compose logs -f

# Stop everything
docker-compose down
```

## Connecting to a Specific Bootstrap Server

If someone shared their bootstrap server address with you:

1. Open `configs/config.yaml`
2. Find the `bootstrap_peers` section
3. Add the address:

```yaml
p2p:
  bootstrap_peers:
    - /ip4/THEIR_IP/tcp/4001/p2p/THEIR_PEER_ID
```

Or set it as an environment variable:

```bash
export BOOTSTRAP_URL=http://THEIR_IP:8081/bootstrap
./start-news-server.sh
```

## Web Interface

Once running, access your node at:

- **Home Page**: http://localhost:12345
- **Create Article**: http://localhost:12345/create
- **Network Status**: http://localhost:12345/network

## Bootstrap Server Status

Check your bootstrap server status at:

- **Web Dashboard**: http://localhost:8081
- **API Status**: http://localhost:8081/status
- **Connected Peers**: http://localhost:8081/peers

## Common Issues

### "Go is not installed"

Download and install Go from: https://golang.org/dl/

### "Port already in use"

Another program is using port 4001 or 12345. Either:
- Stop the other program
- Or change the port in `configs/config.yaml`

### "Can't connect to other nodes"

1. Make sure you're on the same network, OR
2. Configure port forwarding on your router for port 4001

## Need Help?

- Check the logs for error messages
- Make sure your firewall allows connections on ports 4001 and 12345
- Open an issue on GitHub

---

Happy publishing! Your voice matters.
