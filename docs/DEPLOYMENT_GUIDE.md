# Deployment Guide

## Overview

This platform offers **dual-mode architecture** to fit different deployment scenarios:

## Mode Comparison

| Aspect | Centralized (SQLite) | Distributed (IPFS + BadgerDB) |
|--------|---------------------|------------------------------|
| **Use Case** | Development, single-node, simple ops | True P2P, distributed networks, censorship-resistant |
| **Setup** | Easy - just run | Moderate - requires IPFS connectivity |
| **Performance** | Fast (local DB) | Good (IPFS + cache) |
| **Decentralization** | ❌ No | ✅ Yes |
| **Data Location** | Local SQLite file | IPFS network |
| **User Profiles** | SQLite table | IPFS + IPNS |
| **Scalability** | Vertical only | Horizontal (add nodes) |
| **Failure Mode** | DB corrupt = data loss | Distributed (survives node failures) |
| **Recommended For** | Development, staging, single-server production | P2P networks, censorship resistance |

## Quick Start

### Centralized Mode (Default)

```bash
# 1. Start IPFS (for article storage)
ipfs daemon

# 2. Set JWT secret
export NEWS_AUTH_JWT_SECRET="your-secret-min-32-chars"

# 3. Run (SQLite mode is default)
go run ./cmd/server
```

### Distributed Mode

```bash
# 1. Start IPFS
ipfs daemon

# 2. Configure for distributed mode
export NEWS_DATABASE_MODE=distributed
export NEWS_DATABASE_PATH=./data/badger-cache
export NEWS_AUTH_JWT_SECRET="your-secret-min-32-chars"

# 3. Run
go run ./cmd/server
```

## Production Deployment

### Option 1: Centralized (Simpler)

**Best for:**
- Controlled environments
- Single organization
- Predictable performance needs

**Docker Compose:**
```yaml
version: '3.8'
services:
  ipfs:
    image: ipfs/kubo:latest
    volumes:
      - ipfs-data:/data/ipfs

  news-server:
    build: .
    environment:
      - NEWS_DATABASE_MODE=sqlite
      - NEWS_DATABASE_PATH=/data/news.db
      - NEWS_AUTH_JWT_SECRET=${JWT_SECRET}
    volumes:
      - news-data:/data
    depends_on:
      - ipfs
```

**Monitoring:**
- Monitor SQLite file size
- Regular backups (SQLite + IPFS pins)
- Health checks on DB connection

### Option 2: Distributed (True P2P)

**Best for:**
- Decentralized networks
- Censorship resistance
- Community-run infrastructure

**Multi-Node Setup:**

```bash
# Node 1 (US East)
export NEWS_DATABASE_MODE=distributed
docker-compose up -d

# Node 2 (EU West)
export NEWS_DATABASE_MODE=distributed
docker-compose up -d

# Node 3 (Asia)
export NEWS_DATABASE_MODE=distributed
docker-compose up -d
```

All nodes:
- Share the same IPFS network
- Can serve any user/article
- Sync via IPNS
- Run independently

**Monitoring:**
- IPFS connectivity (`ipfs swarm peers`)
- IPNS resolution time
- Cache hit rates
- BadgerDB size

## Configuration Matrix

### Development

```yaml
database:
  mode: sqlite
  path: ./data/news.db

ipfs:
  api_endpoint: http://localhost:5001

logging:
  level: debug
  format: text
```

### Staging

```yaml
database:
  mode: sqlite  # or distributed for testing
  path: ./data/news.db

ipfs:
  api_endpoint: http://ipfs:5001

logging:
  level: info
  format: json
```

### Production (Centralized)

```yaml
database:
  mode: sqlite
  path: /var/lib/news/news.db
  max_open_conns: 25
  max_idle_conns: 10

ipfs:
  api_endpoint: http://ipfs-cluster:5001
  pin_articles: true

logging:
  level: info
  format: json

rate_limit:
  requests_per_minute: 100
  burst: 20
```

### Production (Distributed)

```yaml
database:
  mode: distributed
  path: /var/lib/news/badger-cache

ipfs:
  api_endpoint: http://localhost:5001
  pin_articles: true

logging:
  level: info
  format: json

rate_limit:
  requests_per_minute: 100
  burst: 20
```

## Scaling Strategies

### Centralized Mode

**Vertical Scaling:**
1. Increase server resources
2. Tune SQLite (WAL mode, cache size)
3. Add read replicas (SQLite replication)

**Horizontal Scaling:**
1. Load balancer
2. SQLite replication
3. Shared IPFS cluster
4. Separate search instances

### Distributed Mode

**Horizontal Scaling:**
1. Add more nodes (automatic)
2. Each node independently functional
3. IPFS cluster for redundancy
4. No coordination needed

**Performance Tuning:**
1. Increase BadgerDB cache TTL
2. Pre-warm caches
3. Pin popular content
4. Use IPFS over PubSub for faster IPNS

## Migration Guide

### SQLite → Distributed

```bash
# 1. Backup SQLite
cp data/news.db data/news.db.backup

# 2. Export users
sqlite3 data/news.db ".mode csv" "SELECT * FROM users" > users.csv

# 3. Run migration script
./scripts/migrate-to-distributed.sh users.csv

# 4. Change mode
export NEWS_DATABASE_MODE=distributed

# 5. Restart server
./news-server
```

### Distributed → SQLite

```bash
# 1. Export from IPFS
./scripts/export-from-ipfs.sh > data.json

# 2. Import to SQLite
./scripts/import-to-sqlite.sh data.json

# 3. Change mode
export NEWS_DATABASE_MODE=sqlite

# 4. Restart
./news-server
```

## Backup Strategies

### Centralized Mode

```bash
# Backup SQLite
sqlite3 data/news.db ".backup data/news-$(date +%Y%m%d).db"

# Backup IPFS pins
ipfs pin ls > ipfs-pins-$(date +%Y%m%d).txt

# Backup search index
tar -czf search-$(date +%Y%m%d).tar.gz data/search.bleve
```

### Distributed Mode

```bash
# Backup IPNS keys (critical!)
tar -czf ipns-keys-$(date +%Y%m%d).tar.gz ~/.ipfs/keystore/

# Backup BadgerDB cache (optional)
tar -czf badger-$(date +%Y%m%d).tar.gz data/badger-cache

# Backup IPFS pins
ipfs pin ls > ipfs-pins-$(date +%Y%m%d).txt
```

## Disaster Recovery

### Centralized Mode

**SQLite corrupted:**
1. Restore from backup
2. Re-sync IPFS pins
3. Rebuild search index

**IPFS data lost:**
1. Articles may be retrievable from network
2. Pin from other nodes
3. Worst case: metadata in SQLite intact

### Distributed Mode

**BadgerDB cache lost:**
1. Delete cache directory
2. Restart server
3. Cache rebuilds automatically

**IPNS keys lost:**
1. Restore from backup
2. If no backup: users need to re-register
3. Article data unaffected

**IPFS data lost:**
1. Fetch from network peers
2. Re-pin important content
3. Distributed nature provides resilience

## Monitoring & Observability

### Health Checks

```bash
# Basic health
curl http://localhost:8080/health

# Detailed readiness (checks DB, IPFS, search)
curl http://localhost:8080/health/ready

# Liveness
curl http://localhost:8080/health/live
```

### Metrics to Track

**Centralized:**
- SQLite query latency
- Database size
- Connection pool utilization
- IPFS upload/download rates

**Distributed:**
- IPNS resolution time
- BadgerDB cache hit rate
- IPFS peer count
- Cache size growth

## Troubleshooting

### Centralized Mode Issues

**Slow queries:**
```bash
# Check SQLite indexes
sqlite3 data/news.db ".indexes"

# Analyze query plan
sqlite3 data/news.db "EXPLAIN QUERY PLAN SELECT ..."
```

**Database locked:**
```bash
# Enable WAL mode
sqlite3 data/news.db "PRAGMA journal_mode=WAL;"
```

### Distributed Mode Issues

**Slow IPNS:**
```bash
# Check IPNS cache
ipfs config Ipns.RecordLifetime

# Increase cache time
ipfs config --json Ipns.RecordLifetime '"24h"'
```

**Low cache hit rate:**
```go
// Increase TTL in article_cache.go
entry := badger.NewEntry(cidKey, data).WithTTL(48 * time.Hour)
```

## Summary

Choose your mode based on requirements:

**Use Centralized (SQLite) if:**
- Single-node deployment
- Predictable performance critical
- Simpler operations preferred
- Traditional architecture familiar

**Use Distributed (IPFS) if:**
- True P2P architecture needed
- Censorship resistance required
- Multi-node deployment
- Community-run infrastructure

Both modes share the same API and can be switched via configuration!
