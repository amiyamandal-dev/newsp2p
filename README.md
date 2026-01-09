# Distributed News Platform

A production-ready distributed news platform backend built with Go, IPFS, and full-text search capabilities.

## Features

- **Distributed Storage**: Articles stored on IPFS for permanent, decentralized content
- **Content Addressing**: Access articles via immutable IPFS CIDs
- **IPNS Feeds**: Mutable feeds published to IPNS for easy subscription
- **Cryptographic Signing**: Ed25519 signatures for article authenticity
- **Full-Text Search**: Bleve-powered search with filtering by author, category, tags, and date
- **User Authentication**: JWT-based authentication with access and refresh tokens
- **Rate Limiting**: Protection against API abuse
- **Production-Ready**: Structured logging, health checks, graceful shutdown

## Architecture

This platform supports **two deployment modes**:

### 1. Centralized Mode (SQLite) - Default
Best for: Single-node, development, simpler operations

```
Client → API → Services → SQLite + IPFS + Bleve
```

### 2. Distributed Mode (IPFS + BadgerDB)
Best for: True P2P, distributed networks, censorship resistance

```
Client → API → Services → IPFS (all data) + BadgerDB (cache) + Bleve
```

**Key Difference:**
- **Centralized**: Metadata in SQLite, articles on IPFS
- **Distributed**: Everything on IPFS/IPNS, BadgerDB for local caching only

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed comparison.

## Prerequisites

- Go 1.25+
- IPFS daemon (kubo)
- SQLite3

## Quick Start

### 1. Install IPFS

```bash
# macOS
brew install ipfs

# Linux
wget https://dist.ipfs.tech/kubo/v0.23.0/kubo_v0.23.0_linux-amd64.tar.gz
tar -xvzf kubo_v0.23.0_linux-amd64.tar.gz
cd kubo
sudo bash install.sh
```

### 2. Start IPFS Daemon

```bash
ipfs init
ipfs daemon
```

### 3. Configure Environment

```bash
cp .env.example .env
# Edit .env and set your JWT secret (minimum 32 characters)
```

### 4. Build and Run

```bash
# Build
go build -o news-server ./cmd/server

# Run
./news-server
```

The server will start on `http://localhost:8080`

## Configuration

Configuration can be provided via:
1. `configs/config.yaml`
2. Environment variables (prefixed with `NEWS_`)
3. `.env` file

### Key Configuration Options

| Variable | Default | Description |
|----------|---------|-------------|
| `NEWS_DATABASE_MODE` | sqlite | **Mode**: `sqlite` or `distributed` |
| `NEWS_SERVER_HOST` | 0.0.0.0 | HTTP server host |
| `NEWS_SERVER_PORT` | 12345 | HTTP server port |
| `NEWS_SERVER_MODE` | release | Server mode (debug/release) |
| `NEWS_DATABASE_PATH` | ./data/news.db | DB path (SQLite or BadgerDB) |
| `NEWS_IPFS_API_ENDPOINT` | http://localhost:5001 | IPFS API endpoint |
| `NEWS_AUTH_JWT_SECRET` | - | **Required**: JWT signing secret (32+ chars) |
| `NEWS_LOGGING_LEVEL` | info | Log level (debug/info/warn/error) |
| `NEWS_RATELIMIT_REQUESTS_PER_MINUTE` | 100 | Rate limit per IP |

## API Endpoints

### Authentication

```http
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/refresh
GET  /api/v1/auth/me (protected)
```

### Articles

```http
POST   /api/v1/articles (protected)
GET    /api/v1/articles/:cid
GET    /api/v1/articles?page=1&limit=20&author=&category=&from=&to=
PUT    /api/v1/articles/:id (protected)
DELETE /api/v1/articles/:id (protected)
POST   /api/v1/articles/:cid/verify
```

### Feeds

```http
GET  /api/v1/feeds
GET  /api/v1/feeds/:name
GET  /api/v1/feeds/:name/articles
POST /api/v1/feeds/:name/sync (protected)
```

### Search

```http
GET /api/v1/search?q=query&author=&category=&tags=&from=&to=&page=1&limit=20
```

### Health

```http
GET /health
GET /health/ready
GET /health/live
```

## Usage Examples

### Register a User

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "alice",
    "email": "alice@example.com",
    "password": "securepassword123"
  }'
```

### Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "alice",
    "password": "securepassword123"
  }'
```

### Create Article

```bash
curl -X POST http://localhost:8080/api/v1/articles \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -d '{
    "title": "Hello Decentralized World",
    "body": "This is my first article on IPFS!",
    "tags": ["intro", "ipfs"],
    "category": "general"
  }'
```

### Get Article by CID

```bash
curl http://localhost:8080/api/v1/articles/QmXoypizjW3WknFiJnKLwHCnL72vedxjQkDDP1mXWo6uco
```

### Search Articles

```bash
curl "http://localhost:8080/api/v1/search?q=decentralized&category=general&page=1&limit=10"
```

## Development

### Project Structure

```
newsp2p/
├── cmd/server/           # Application entry point
├── internal/
│   ├── api/             # HTTP handlers, middleware, router
│   ├── auth/            # JWT and signature management
│   ├── config/          # Configuration loading
│   ├── domain/          # Domain models
│   ├── ipfs/            # IPFS client and IPNS
│   ├── repository/      # Data access layer
│   ├── search/          # Search indexing (Bleve)
│   ├── service/         # Business logic
│   └── validator/       # Request validation
├── pkg/
│   ├── crypto/          # Cryptographic utilities
│   ├── logger/          # Structured logging
│   └── response/        # Standard API responses
├── migrations/          # Database migrations
├── configs/             # Configuration files
└── docs/                # Documentation
```

### Running Tests

```bash
go test ./...
```

### Building for Production

```bash
CGO_ENABLED=1 go build -ldflags="-s -w" -o news-server ./cmd/server
```

## P2P Bootstrap Server

For true peer-to-peer networking, run a dedicated bootstrap server that helps peers discover each other.

### Starting the Bootstrap Server

```bash
# Build and run
go build -o bootstrap ./cmd/bootstrap
./bootstrap

# Or use the helper script
./scripts/run-bootstrap.sh

# With custom ports
./bootstrap -p2p-port=4001 -http-port=8081
```

### Bootstrap Server Options

| Flag | Default | Description |
|------|---------|-------------|
| `-p2p-port` | 4001 | P2P listen port (TCP/QUIC) |
| `-http-port` | 8081 | HTTP API port for status |
| `-data-dir` | data | Directory for identity key |
| `-rendezvous` | liberation-news-network | Network rendezvous string |

### Bootstrap Server API

```bash
# Health check
curl http://localhost:8081/health

# Server status and connected peers
curl http://localhost:8081/status

# List connected peers
curl http://localhost:8081/peers

# Get bootstrap connection info
curl http://localhost:8081/bootstrap
```

### Connecting Nodes to Bootstrap Server

1. Start the bootstrap server and note the peer ID and address
2. Add the bootstrap address to your node's `configs/config.yaml`:

```yaml
p2p:
  bootstrap_peers:
    - /ip4/YOUR_BOOTSTRAP_IP/tcp/4001/p2p/BOOTSTRAP_PEER_ID
```

3. Start your news server nodes - they will automatically discover each other

## Docker Deployment

```bash
# Build and run with Docker Compose
docker-compose up -d

# View logs
docker-compose logs -f news-server

# Stop
docker-compose down
```

## Security Features

- **JWT Authentication**: Secure token-based authentication
- **Ed25519 Signatures**: Cryptographic article signing
- **Bcrypt Password Hashing**: Cost factor 12
- **Rate Limiting**: Per-IP request throttling
- **CORS Protection**: Configurable allowed origins
- **Input Validation**: Request validation on all endpoints

## Performance

- Article retrieval (DB): < 50ms (p95)
- Article creation: < 500ms (p95)
- Search queries: < 100ms (p95)
- Throughput: 100+ req/sec per instance

## Monitoring

- `/health`: Basic health check
- `/health/ready`: Readiness probe (checks DB, IPFS, search)
- `/health/live`: Liveness probe

## License

MIT

## Contributing

Contributions welcome! Please open an issue or submit a pull request.

## Troubleshooting

### IPFS Connection Issues

```bash
# Check if IPFS daemon is running
ipfs id

# Restart IPFS daemon
ipfs shutdown
ipfs daemon
```

### Database Errors

```bash
# Check database file permissions
ls -l data/news.db

# Delete and recreate (WARNING: loses data)
rm data/news.db
```

### Search Index Issues

```bash
# Rebuild search index (delete and restart server)
rm -rf data/search.bleve
```

## Support

For issues and questions, please open an issue on GitHub.
