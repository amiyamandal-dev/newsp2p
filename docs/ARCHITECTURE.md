# Architecture Overview

This platform supports **two deployment modes**: Centralized and Fully Distributed P2P.

## Deployment Modes

### 1. Centralized Mode (SQLite)

**Best for**: Single-node deployments, development, simpler operations

```
┌─────────────────┐
│   HTTP Client   │
└────────┬────────┘
         │
    ┌────┴─────┐
    │ Gin API  │
    └────┬─────┘
         │
    ┌────┴──────────────────┐
    │ Services (Bus Logic)  │
    └────┬──────────────────┘
         │
    ┌────┴────────────┬─────────────┬──────────┐
    │   SQLite DB     │    IPFS     │  Bleve   │
    │  (metadata)     │ (articles)  │ (search) │
    └─────────────────┴─────────────┴──────────┘
```

**Data Storage:**
- **SQLite**: User profiles, article metadata, indexes
- **IPFS**: Article content (permanent storage)
- **Bleve**: Full-text search index

**Pros:**
- Simple setup and deployment
- Fast local queries
- Familiar SQL-based operations
- Easy backups

**Cons:**
- Centralized database (single point of failure)
- Not truly P2P
- Harder to scale horizontally

### 2. Distributed Mode (IPFS + BadgerDB)

**Best for**: True P2P deployments, distributed networks, censorship resistance

```
┌─────────────────┐
│   HTTP Client   │
└────────┬────────┘
         │
    ┌────┴─────┐
    │ Gin API  │
    └────┬─────┘
         │
    ┌────┴──────────────────┐
    │ Services (Bus Logic)  │
    └────┬──────────────────┘
         │
    ┌────┴────────────┬─────────────┬──────────┐
    │  BadgerDB       │    IPFS     │  Bleve   │
    │ (cache only)    │ (all data)  │ (search) │
    └─────────────────┴─────────────┴──────────┘
                       │
                  ┌────┴────┐
                  │  IPNS   │
                  │ (users, │
                  │  feeds) │
                  └─────────┘
```

**Data Storage:**
- **IPFS**: Articles, user profiles, all persistent data
- **IPNS**: Mutable pointers to user profiles and feeds
- **BadgerDB**: Local cache/index (can be rebuilt from IPFS)
- **Bleve**: Full-text search index (can be rebuilt)

**Pros:**
- Fully decentralized and P2P
- No single point of failure
- Censorship resistant
- Data persists on IPFS network
- Can run anywhere with IPFS access

**Cons:**
- More complex setup
- IPNS resolution can be slower
- Requires IPFS daemon connectivity
- Cache rebuilding on first run

## Data Flow

### Centralized Mode

**User Registration:**
```
Client → API → UserService → SQLite
                           → Generate Ed25519 keys
                           → Hash password (bcrypt)
                           → Store in DB
```

**Article Creation:**
```
Client → API → ArticleService → Sign with Ed25519
                               → Upload to IPFS (get CID)
                               → Store metadata in SQLite
                               → Index in Bleve
                               → Return CID
```

**Article Retrieval:**
```
Client → API → ArticleService → Check SQLite (by CID)
                               → If not found: fetch from IPFS
                               → Verify signature
                               → Return article
```

### Distributed Mode

**User Registration:**
```
Client → API → UserService → Generate Ed25519 keys
                           → Hash password (bcrypt)
                           → Serialize user profile
                           → Upload to IPFS (get CID)
                           → Create IPNS key (user-{username})
                           → Publish CID to IPNS
                           → Cache in BadgerDB
```

**Article Creation:**
```
Client → API → ArticleService → Sign with Ed25519
                               → Upload to IPFS (get CID)
                               → Cache metadata in BadgerDB
                               → Index in Bleve
                               → Return CID
```

**Article Retrieval:**
```
Client → API → ArticleService → Check BadgerDB cache
                               → If not found: fetch from IPFS by CID
                               → Verify signature
                               → Cache in BadgerDB
                               → Return article
```

**User Profile Retrieval:**
```
Client → API → UserService → Resolve IPNS (/ipns/user-{username})
                           → Get latest profile CID
                           → Fetch from IPFS
                           → Cache in BadgerDB
                           → Return user profile
```

## Feed Synchronization (Both Modes)

Feeds work similarly in both modes, using IPFS + IPNS:

```
Background Sync Service (every 15 min):
1. Get recent articles (from SQLite or IPFS)
2. Create FeedManifest { articles: [CIDs...] }
3. Sign manifest
4. Upload manifest to IPFS → get CID
5. Publish to IPNS (/ipns/feed-{name})
6. Pin new manifest, unpin old
```

Subscribers can:
1. Resolve IPNS name → get latest manifest CID
2. Fetch manifest from IPFS
3. Verify signature
4. Fetch each article by CID

## Security Model

Both modes use the same security:

**Authentication:**
- JWT tokens (HS256, 24h expiry)
- Refresh tokens (7 days)
- Bcrypt password hashing (cost 12)

**Article Integrity:**
- Ed25519 signatures on all articles
- Each user has a key pair (generated on registration)
- Private key encrypted and stored
- Public key included with articles
- Signature verification on retrieval

**Content Addressing:**
- Articles accessed by CID (content hash)
- CIDs are immutable and tamper-proof
- Any modification changes the CID

## Switching Between Modes

Set `database.mode` in config:

```yaml
database:
  mode: "distributed"  # or "sqlite"
  path: "./data/news.db"  # SQLite path or BadgerDB path
```

Or via environment:
```bash
export NEWS_DATABASE_MODE=distributed
```

## Performance Comparison

| Operation | Centralized (SQLite) | Distributed (IPFS) |
|-----------|---------------------|-------------------|
| User lookup | ~1ms (local DB) | ~100-500ms (IPNS resolve) |
| Article by CID | ~50ms (if cached) | ~50-200ms (IPFS fetch) |
| Article search | ~50ms (Bleve) | ~50ms (Bleve) |
| Article creation | ~300ms | ~500ms (IPFS upload) |
| List articles | ~10ms (SQL query) | ~50ms (cache or IPFS) |

## Caching Strategy (Distributed Mode)

BadgerDB serves as a **local cache** only:

**Cache TTL:**
- Articles: 24 hours
- User profiles: 1 hour
- Search results: No cache (always query Bleve)

**Cache Invalidation:**
- On update: invalidate and refetch from IPFS
- On startup: can rebuild from IPFS if needed
- Periodic cleanup of expired entries

**Cache Rebuild:**
If BadgerDB is deleted, the system will:
1. Continue to function (fetch from IPFS)
2. Gradually rebuild cache as items are accessed
3. Performance improves as cache warms up

## Recommended Setup

**Development:** Centralized mode (SQLite)
- Fast iteration
- Simple debugging
- Local-first development

**Production (Single Node):** Centralized mode
- Easier operations
- Predictable performance
- Good for controlled deployments

**Production (P2P Network):** Distributed mode
- Multiple nodes can run independently
- Data survives node failures
- Censorship resistant
- True decentralization

## Migration Path

To migrate from centralized to distributed:

1. Export data from SQLite
2. Upload user profiles to IPFS, create IPNS keys
3. Articles already on IPFS (just need metadata migration)
4. Switch mode in config
5. Restart with BadgerDB cache

Script provided in `scripts/migrate-to-distributed.sh`
