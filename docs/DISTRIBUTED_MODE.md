# Running in Distributed Mode

This guide shows how to run the platform in **fully distributed P2P mode** where all data is stored on IPFS.

## Overview

In distributed mode:
- **User profiles** → IPFS + IPNS
- **Articles** → IPFS (by CID)
- **Feeds** → IPFS manifests published to IPNS
- **BadgerDB** → Local cache only (can be deleted and rebuilt)

## Setup

### 1. Start IPFS Daemon

```bash
ipfs daemon
```

### 2. Configure for Distributed Mode

**Option A: Environment Variable**
```bash
export NEWS_DATABASE_MODE=distributed
export NEWS_DATABASE_PATH=./data/badger-cache
export NEWS_AUTH_JWT_SECRET="your-secret-key-min-32-characters"
```

**Option B: Config File**
```yaml
# configs/config.yaml
database:
  mode: distributed
  path: ./data/badger-cache
```

### 3. Run the Server

```bash
go run ./cmd/server
```

Or with Docker:
```yaml
# docker-compose.yml
environment:
  - NEWS_DATABASE_MODE=distributed
  - NEWS_DATABASE_PATH=/data/badger-cache
```

## How It Works

### User Registration

```
1. User submits: username, email, password
2. Server generates Ed25519 key pair
3. Server hashes password (bcrypt)
4. User profile serialized to JSON
5. Upload profile to IPFS → CID
6. Create IPNS key: "user-alice"
7. Publish CID to IPNS → /ipns/Qm...
8. Cache in BadgerDB for fast access
9. Return success
```

### User Login

```
1. User submits: username, password
2. Resolve IPNS: /ipns/user-alice → CID
3. Fetch profile from IPFS
4. Verify password
5. Generate JWT tokens
6. Return tokens
```

### Article Creation

```
1. User creates article
2. Sign with Ed25519
3. Upload to IPFS → CID
4. Cache metadata in BadgerDB
5. Index in Bleve
6. Return CID to user
```

### Article Retrieval

```
1. User requests article by CID
2. Check BadgerDB cache
3. If miss: fetch from IPFS
4. Verify signature
5. Cache in BadgerDB
6. Return article
```

## Advantages

**Decentralization:**
- No central database
- Data survives node failures
- Multiple nodes can run independently

**Censorship Resistance:**
- Content on IPFS is permanent
- IPNS provides mutable pointers
- No single point of control

**Data Portability:**
- Users own their data (via IPNS keys)
- Can access from any node
- Easy data migration

**Scalability:**
- Horizontal scaling via IPFS
- Each node is independent
- Cache improves local performance

## Trade-offs

**Slower User Lookups:**
- IPNS resolution: 100-500ms vs 1ms (SQLite)
- Mitigated by BadgerDB caching

**IPFS Dependency:**
- Requires IPFS daemon connectivity
- Network latency affects performance

**Cache Management:**
- BadgerDB cache can grow large
- Need periodic cleanup
- Cold start slower (empty cache)

## Performance Optimization

### 1. Pre-warm Cache

```bash
# Fetch popular profiles
curl http://localhost:8080/api/v1/users/alice
curl http://localhost:8080/api/v1/users/bob

# Fetch recent articles
curl http://localhost:8080/api/v1/articles?limit=100
```

### 2. Increase Cache TTL

```go
// In article_cache.go
entry := badger.NewEntry(cidKey, data).WithTTL(48 * time.Hour)  // 48h vs 24h
```

### 3. Pin Important Content

```bash
# Pin user profiles
ipfs pin add /ipns/user-alice
ipfs pin add /ipns/user-bob

# Pin feed manifests
ipfs pin add QmFeedManifest...
```

### 4. Use IPFS Cluster

For production, use IPFS Cluster for:
- Redundancy
- Load balancing
- Automatic pinning across nodes

## Monitoring

### Check IPNS Resolution

```bash
# Resolve user IPNS
ipfs name resolve /ipns/user-alice

# Should return: /ipfs/QmUserProfile...
```

### Check Cache Stats

```bash
# BadgerDB stats (add endpoint)
curl http://localhost:8080/api/v1/admin/cache/stats
```

### Monitor IPFS

```bash
# IPFS node stats
ipfs stats bw
ipfs swarm peers
```

## Troubleshooting

### Slow IPNS Resolution

**Problem:** User login takes 5-10 seconds

**Solutions:**
1. Increase IPNS cache: `ipfs config --json Ipns.RecordLifetime '"24h"'`
2. Use DHT with more peers
3. Consider IPNS over PubSub

### Cache Misses

**Problem:** Slow article retrieval

**Solutions:**
1. Increase BadgerDB TTL
2. Pre-warm cache on startup
3. Pin frequently accessed content

### IPFS Connection Issues

**Problem:** "IPFS unavailable" errors

**Solutions:**
1. Check IPFS daemon: `ipfs id`
2. Verify API endpoint: `NEWS_IPFS_API_ENDPOINT`
3. Check firewall/network

## Migration from SQLite

To migrate from centralized to distributed mode:

```bash
# 1. Export users from SQLite
sqlite3 data/news.db "SELECT * FROM users" > users.csv

# 2. Run migration script
./scripts/migrate-to-distributed.sh

# 3. Change config
export NEWS_DATABASE_MODE=distributed

# 4. Restart server
./news-server
```

## Best Practices

1. **Backup IPNS Keys**: Keys in `~/.ipfs/keystore/` are critical
2. **Pin Important Content**: Don't rely on cache for critical data
3. **Monitor IPFS Health**: Use `/health/ready` endpoint
4. **Regular Cache Cleanup**: Prevent BadgerDB from growing too large
5. **Use IPFS Cluster**: For production deployments

## Example Deployment

### Multi-Node P2P Network

```bash
# Node 1 (US)
docker-compose up -d
export NEWS_DATABASE_MODE=distributed

# Node 2 (EU)
docker-compose up -d
export NEWS_DATABASE_MODE=distributed

# Node 3 (Asia)
docker-compose up -d
export NEWS_DATABASE_MODE=distributed
```

All nodes share the same IPFS network:
- Articles accessible from any node
- Users can login from any node
- Feeds synchronized via IPNS
- No central coordination needed

## Comparison Table

| Feature | Centralized (SQLite) | Distributed (IPFS) |
|---------|---------------------|-------------------|
| Setup Complexity | ⭐ Easy | ⭐⭐⭐ Moderate |
| Performance | ⭐⭐⭐ Fast | ⭐⭐ Good |
| Decentralization | ❌ No | ✅ Yes |
| Scalability | ⭐⭐ Vertical | ⭐⭐⭐ Horizontal |
| Data Portability | ⭐ Export needed | ⭐⭐⭐ Native |
| Censorship Resistance | ❌ No | ✅ Yes |
| Operational Overhead | ⭐ Low | ⭐⭐ Moderate |

## Summary

Distributed mode provides a **truly P2P architecture** with:
- Full decentralization
- Censorship resistance
- Data ownership

Trade-offs include:
- More complex setup
- Slightly slower lookups
- IPFS dependency

**Recommended for:**
- P2P networks
- Censorship-resistant platforms
- Distributed communities
- Privacy-focused applications
