# DECISIONS.md

## Architectural Decisions Record (ADR)

### 1. No ORM — Raw SQL via pgx/v5

**Decision**: Use raw SQL with `pgx/v5` for all database operations.

**Rationale**: ORMs add a layer of abstraction that obscures query execution plans, makes performance tuning difficult, and often generates suboptimal SQL (e.g., N+1 queries). For a high-throughput trading system with strict p95 latency targets (≤150ms write, ≤200ms read), raw SQL gives full control over:
- `INSERT ... ON CONFLICT DO NOTHING RETURNING *` for idempotency
- `time_bucket()` and TimescaleDB-specific functions
- Partial indexes and chunk exclusion in queries
- Optimal use of `pgxpool` for connection pooling

### 2. TimescaleDB for Time-Series Analytics

**Decision**: Use TimescaleDB hypertables for `behavioral_metrics`.

**Rationale**: The `behavioral_metrics` table is written to on every trade close and read on every metrics query. TimescaleDB provides:
- Automatic chunk partitioning by time (1-hour chunks)
- Chunk exclusion during range queries → O(relevant_chunks) instead of full table scan
- `time_bucket()` aggregate function for flexible granularity (hourly, daily, 30d)
- Compression policies for older data (future optimization)

### 3. Redis Streams for Async Analytics Pipeline

**Decision**: Decouple metric computation from the HTTP write path using Redis Streams consumer groups.

**Rationale**: The TDD mandates that HTTP 200 is returned *before* metrics computation. This architectural choice:
- Guarantees bounded write latency (independent of metric computation time)
- Provides at-least-once delivery via consumer groups + XACK
- Enables horizontal scaling of workers later
- Uses Redis as an already-deployed dependency (no Kafka/RabbitMQ overhead)

### 4. Sliding Window via Redis ZSET

**Decision**: Implement overtrading detection with `ZADD NX + ZREMRANGEBYSCORE + ZCOUNT` in a single pipeline.

**Rationale**: A 30-minute sliding window with real-time trade counting requires sub-millisecond operations. Redis ZSET with timestamps as scores provides O(log N) operations for all three operations, pipelined in one round-trip.

### 5. Row-Level Tenancy via JWT `sub` Claim

**Decision**: Enforce tenancy at the handler level by comparing JWT `sub` (userId) with the requested resource's `userId`. Return 403 on mismatch, never 404.

**Rationale**: Returning 404 for cross-tenant access leaks information about resource existence. The 403 response with `traceId` provides auditability without information disclosure.

### 6. Cache-Aside with 30s TTL

**Decision**: Use Redis as a cache-aside layer for metrics queries with a 30-second TTL.

**Rationale**: Metrics data changes slowly relative to query frequency. A 30s TTL balances freshness (pipeline processes events within seconds) with read performance (cache hit avoids TimescaleDB query entirely).

### 7. Idempotency via ON CONFLICT DO NOTHING

**Decision**: `POST /trades` uses `INSERT ... ON CONFLICT (trade_id) DO NOTHING RETURNING *` with a fallback `SELECT`.

**Rationale**: This approach:
- Is fully ACID-compliant (single statement)
- Returns 200 with the original record on duplicate (not 409)
- Requires no separate idempotency key table
- Is immune to race conditions between concurrent inserts of the same trade_id

### 8. Multi-Stage Docker Build

**Decision**: Use `golang:1.23-alpine` builder → `alpine:3.20` runtime.

**Rationale**: Alpine runtime provides ca-certificates and tzdata needed for HTTPS and timezone-aware timestamps, while keeping the final image small (~15MB per binary). Using alpine instead of scratch for operational flexibility (shell access for debugging).
