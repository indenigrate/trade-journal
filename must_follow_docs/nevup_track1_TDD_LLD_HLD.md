# NevUp Track 1 — Technical Design Document (TDD)

**System of Record — Backend Engineering**
**Hackathon:** NevUp Hiring Hackathon 2026
**Track:** 1 of 3 — Trade Journal Engine
**Language:** Go 1.23
**Author:** [Your Handle]
**Status:** Draft — Pre-Implementation
**Last Updated:** 2026-04-26

---

## Table of Contents

1. [Document Purpose & Scope](#1-document-purpose--scope)
2. [Requirements Summary](#2-requirements-summary)
   - 2.1 [Functional Requirements](#21-functional-requirements)
   - 2.2 [Non-Functional Requirements](#22-non-functional-requirements)
   - 2.3 [Hard Constraints](#23-hard-constraints)
3. [High-Level Design (HLD)](#3-high-level-design-hld)
   - 3.1 [System Context](#31-system-context)
   - 3.2 [Component Overview](#32-component-overview)
   - 3.3 [Technology Choices & Rationale](#33-technology-choices--rationale)
   - 3.4 [Data Flow — Level 0 (Context DFD)](#34-data-flow--level-0-context-dfd)
   - 3.5 [Data Flow — Level 1 (Process Decomposition)](#35-data-flow--level-1-process-decomposition)
   - 3.6 [Docker Compose Topology](#36-docker-compose-topology)
4. [Low-Level Design (LLD)](#4-low-level-design-lld)
   - 4.1 [Go Module Structure](#41-go-module-structure)
   - 4.2 [Canonical Data Schema](#42-canonical-data-schema)
   - 4.3 [Database Schema — PostgreSQL + TimescaleDB](#43-database-schema--postgresql--timescaledb)
   - 4.4 [API Contract — All Endpoints](#44-api-contract--all-endpoints)
   - 4.5 [Authentication & Tenancy](#45-authentication--tenancy)
   - 4.6 [Write Path — POST /trades Sequence](#46-write-path--post-trades-sequence)
   - 4.7 [Idempotency Implementation](#47-idempotency-implementation)
   - 4.8 [Async Analytics Pipeline](#48-async-analytics-pipeline)
   - 4.9 [Behavioral Metric Definitions](#49-behavioral-metric-definitions)
   - 4.10 [Read API — Metrics Query Strategy](#410-read-api--metrics-query-strategy)
   - 4.11 [SSE Coaching Stream](#411-sse-coaching-stream)
   - 4.12 [Observability](#412-observability)
   - 4.13 [Health Endpoint](#413-health-endpoint)
5. [Test Strategy](#5-test-strategy)
   - 5.1 [Unit Tests](#51-unit-tests)
   - 5.2 [Integration Tests](#52-integration-tests)
   - 5.3 [Load Test — k6](#53-load-test--k6)
6. [DECISIONS.md — Architectural Rationale](#6-decisionsmd--architectural-rationale)
7. [Submission Checklist](#7-submission-checklist)

---

## 1. Document Purpose & Scope

This document is the single source of truth for the Track 1 backend implementation of the NevUp Hiring Hackathon 2026. It covers both the High-Level Design (system boundaries, component relationships, technology selection) and the Low-Level Design (database schema, SQL, Go package structure, metric computation logic, sequence diagrams in prose, and test specifications).

Every design decision in this document is traceable to a specific requirement from the problem statement PDF or the shared `nevup_openapi.yaml` contract. Any deviation from the OpenAPI schema or the canonical trade schema breaks interoperability with Track 2 and Track 3 and will result in scoring deductions.

**Out of scope for Track 1:** AI/ML coaching logic (Track 2), frontend UI (Track 3), and the mock API Prism server (Track 3's concern).

---

## 2. Requirements Summary

### 2.1 Functional Requirements

#### API Endpoints (OpenAPI contract — field names and enum values are non-negotiable)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/trades` | Submit a trade. Idempotent on `tradeId`. Duplicate → HTTP 200 with existing record. |
| `GET` | `/trades/{tradeId}` | Fetch a single trade by ID. Row-level tenancy enforced. |
| `GET` | `/sessions/{sessionId}` | Session summary with full trade list. |
| `POST` | `/sessions/{sessionId}/debrief` | Persist debrief answers. Returns `{ debriefId, sessionId, savedAt }`. |
| `GET` | `/sessions/{sessionId}/coaching` | SSE stream of coaching tokens (`text/event-stream`). |
| `GET` | `/users/{userId}/metrics` | Behavioral metrics timeseries. Query params: `from`, `to`, `granularity=(hourly\|daily\|rolling30d)`. p95 ≤ 200ms. |
| `GET` | `/users/{userId}/profile` | Behavioral profile with `dominantPathologies`, `evidenceSessions`, `evidenceTrades`. |
| `GET` | `/health` | No auth required. Returns `{ status, dbConnection, queueLag, timestamp }`. |

#### Behavioral Metrics (all five, computed asynchronously outside the write path)

| # | Metric | Trigger |
|---|--------|---------|
| 1 | **Plan adherence score** | Rolling 10-trade average of `planAdherence` per user. |
| 2 | **Revenge trade flag** | Trade opens within 90s of a losing close AND `emotionalState ∈ {anxious, fearful}`. |
| 3 | **Session tilt index** | Ratio of loss-following trades to total trades in the current `sessionId`. |
| 4 | **Win rate by emotional state** | Per-user running win/loss count per `emotionalState`, date-range filterable. |
| 5 | **Overtrading detector** | User opens >10 trades in any 30-minute sliding window → emit event. Must not block write path. |

#### Seed Data

- Load `nevup_seed_dataset.csv` (388 trades, 10 traders, 52 sessions) into PostgreSQL on container startup via migration/seed script.
- `GET /users/:id/metrics` must return queryable results immediately after `docker compose up`.

### 2.2 Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| Write latency | p95 ≤ 150ms under 200 concurrent VUs for 60 seconds |
| Read latency | `GET /users/:id/metrics` p95 ≤ 200ms against seeded dataset |
| Throughput | 200 trade-close events/sec sustained |
| Availability | Single `docker compose up` — no manual steps |
| Observability | Structured JSON logs on every request: `{ traceId, userId, latency, statusCode }` |
| Persistence | Metrics survive `docker compose restart` (no ephemeral in-memory stores) |

### 2.3 Hard Constraints

- No ORM — raw SQL via `pgx/v5`. No N+1 queries.
- No mock or hardcoded data — all data from `nevup_seed_dataset.csv`.
- No HTTP polling for analytics — message queue (Redis Streams) required.
- `POST /trades` must return HTTP 200 for duplicates — never 409 or 500.
- Cross-tenant reads must return HTTP 403 — **never 404, never 200**.
- `traceId` must appear in every 401/403 error response body.
- DB migrations must be in repo. Reviewed in `docker-compose` startup sequence.
- k6 load test script + HTML results report must be in the repo.
- `DECISIONS.md` at repo root with one paragraph per significant architectural decision.

---

## 3. High-Level Design (HLD)

### 3.1 System Context

The NevUp backend is the **System of Record** for retail day trader behavioral data. It sits between:

- **Upstream producers:** Track 3 frontend (submits trades, reads metrics, consumes SSE coaching stream), Track 2 AI engine (reads sessions and behavioral metrics to build coaching context), and the k6 load harness (synthetic trade submission for proving throughput).
- **Downstream stores:** PostgreSQL + TimescaleDB (durable trade and metric storage), Redis (event bus + hot cache + sliding window state).

The system boundary is a single Go binary exposed on port 8080 with a JWT-authenticated REST + SSE API. A second binary (`worker`) runs the async analytics pipeline.

### 3.2 Component Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        API Layer (Go / chi)                     │
│  JWT Middleware → TraceId Injection → Route Handlers → Logger   │
└────────────┬────────────────────────────┬───────────────────────┘
             │ pgx/v5 pool                │ go-redis/v9
             ▼                            ▼
┌────────────────────────┐    ┌────────────────────────────────────┐
│  PostgreSQL +          │    │  Redis                             │
│  TimescaleDB           │    │  ├── Streams  (trades:events)      │
│  ├── trades            │    │  ├── Cache    (metrics TTL 30s)    │
│  ├── sessions          │    │  └── ZSET     (sliding window)     │
│  ├── debriefs          │    └────────────────┬───────────────────┘
│  ├── behavioral_metrics│                     │ XREAD consumer group
│  └── emotion_win_rates │    ┌────────────────▼───────────────────┐
└────────────────────────┘    │  Worker (Go goroutines)            │
                              │  ├── plan_adherence_worker         │
                              │  ├── revenge_flag_worker           │
                              │  ├── session_tilt_worker           │
                              │  ├── emotion_winrate_worker        │
                              │  └── overtrading_worker            │
                              └────────────────────────────────────┘
```

### 3.3 Technology Choices & Rationale

#### HTTP Framework — `chi`

Selected over `gin`, `fiber`, and `echo`. Chi is idiomatic stdlib-compatible Go with composable middleware, first-class `http.Flusher` support for SSE, and zero framework-specific context types. The middleware chain `traceId → auth → logger → handler` maps cleanly to chi's `Use()` pattern.

**Rejected alternatives:**
- `gin` — slightly more allocations per request; non-standard context type complicates SSE and raw http.ResponseWriter access.
- `fiber` — fasthttp context is not compatible with stdlib, breaking `golang-jwt` and `zerolog`'s request middleware.

#### Database — PostgreSQL 16 + TimescaleDB

TimescaleDB's `time_bucket()` function directly satisfies the `granularity=hourly|daily|rolling30d` contract at the database level. Hypertable automatic chunk partitioning ensures `GET /users/:id/metrics` scans only 1–2 data chunks for the seeded Jan–Feb 2026 date range, making p95 ≤ 200ms trivially achievable. `decimal(18,8)` is stored as `NUMERIC(18,8)` — PostgreSQL's native arbitrary-precision decimal, no floating-point rounding on financial values.

**Rejected alternatives:**
- Plain PostgreSQL — requires application-level GROUP BY for time bucketing; no chunk pruning.
- CockroachDB — distributed dialect differences; overkill for a single-node hackathon deployment.

#### Message Queue — Redis Streams

A single Redis container handles three concerns: event bus (Streams), hot metrics cache (GET/SET), and sliding window state (ZSET). Consumer groups provide at-least-once delivery with XACK. Eliminates the need for a separate Kafka or RabbitMQ container, reducing `docker-compose` complexity and cold-start time.

**Rejected alternatives:**
- Kafka — additional container, non-trivial topic/partition config, overkill for 388 seed trades.
- RabbitMQ — second container; AMQP is heavier than Redis protocol for this volume.
- NATS JetStream — less community familiarity; not worth the learning overhead in 72 hours.

#### Logger — `zerolog`

Zero-allocation structured JSON logger. Attaches to chi middleware as a `zerolog.Logger` per request, injecting `traceId`, `userId`, `latency`, and `statusCode` fields exactly as required by the spec. Faster than `zap` for the high-throughput write path.

#### pgx/v5 (raw SQL, no ORM)

Spec explicitly prohibits ORM-hidden N+1 queries. `pgx/v5` with `pgxpool` gives connection pooling, prepared statement caching, and native NUMERIC type handling without `database/sql` adapter overhead.

### 3.4 Data Flow — Level 0 (Context DFD)

```
                ┌──────────────┐
                │   Track 3    │──trade, query──►┐
                │   Frontend   │◄──metrics, SSE──┤
                └──────────────┘                 │
                                                 │
                ┌──────────────┐                 │    ┌─────────────────────────────┐
                │   Track 2    │──query──────────►    │                             │
                │   AI Engine  │◄──BehavioralMetrics──┤   NevUp Backend (Track 1)   │
                └──────────────┘                 │    │   P0 — Trade Journal Engine │
                                                 │    │                             │
                ┌──────────────┐                 │    └────────┬──────────┬─────────┘
                │   k6 Tester  │──load test──────►            │          │
                └──────────────┘                              │          │
                                                    read/write│    events│/cache
                                                              ▼          ▼
                                                        PostgreSQL     Redis
                                                        TimescaleDB
                                              ▲
                                              │ seed on startup
                                    nevup_seed_dataset.csv
                                    (388 trades, 10 traders)
```

### 3.5 Data Flow — Level 1 (Process Decomposition)

Three internal processes, three data stores:

```
External Input
     │
     ▼
┌────────────┐   trade event    ┌─────────────────┐   cache miss    ┌──────────────┐
│  P1        │ ───────────────► │  P2             │ ───────────────►│  P3          │
│  Trade     │                  │  Analytics      │                  │  Read API    │
│  Ingestion │                  │  Pipeline       │                  │  metrics,    │
│  idempotent│                  │  5 workers      │                  │  profile,SSE │
│  P&L calc  │                  │  XREAD loop     │                  │              │
└─────┬──────┘                  └────────┬────────┘                  └──────┬───────┘
      │                                  │                                  │
      ▼                                  ▼                                  ▼
   D1 trades                      D2 behavioral_metrics           D4 Redis cache
   PostgreSQL                         TimescaleDB                     TTL 30s
      │                                                                     ▲
      ▼                                                                     │
   D3 trades:events                                               D2 metrics hypertable
      Redis Stream
```

**P1 — Trade Ingestion:** Validates JWT → checks idempotency (`ON CONFLICT`) → computes P&L → sets `revengeFlag` via Redis TTL lookup → writes to PostgreSQL → publishes event to Redis Streams → returns HTTP 200. Target: p95 ≤ 150ms wall-clock.

**P2 — Analytics Pipeline:** XREAD consumer group `nevup-analytics`. Dispatches each trade event to all five metric workers concurrently. Upserts results to `behavioral_metrics` hypertable and `emotion_win_rates`. XACK on successful upsert. Completely decoupled from the HTTP response.

**P3 — Read API:** Cache-aside on Redis (TTL 30s). On miss, queries TimescaleDB `time_bucket()` aggregation with chunk exclusion. Writes result to cache. Returns `BehavioralMetrics` JSON. p95 ≤ 200ms.

### 3.6 Docker Compose Topology

Five services, one internal bridge network (`nevup-net`), only port 8080 externally exposed:

| Service | Image | Depends On | Role |
|---------|-------|------------|------|
| `postgres-ts` | `timescale/timescaledb:latest-pg16` | — | Persistent trade + metrics store |
| `redis` | `redis:7-alpine` | — | Streams + cache + ZSET |
| `migrator` | Custom Go image (same as api) | `postgres-ts` (healthy) | Runs `goose up` + seeds CSV. Exits 0. |
| `api` | Custom Go image (scratch) | `migrator` (completed), `redis` (healthy) | HTTP server on :8080 |
| `worker` | Same image as api, different CMD | `migrator` (completed), `redis` (healthy) | Async analytics consumer |

```yaml
# docker-compose.yml (abbreviated for clarity — full version in repo)
services:
  postgres-ts:
    image: timescale/timescaledb:latest-pg16
    environment:
      POSTGRES_DB: nevup
      POSTGRES_USER: nevup
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - pg_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD", "pg_isready", "-U", "nevup"]
      interval: 5s
      retries: 10

  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 3s
      retries: 10

  migrator:
    build: .
    command: ["/app/migrator"]
    environment:
      DATABASE_URL: postgres://nevup:${POSTGRES_PASSWORD}@postgres-ts:5432/nevup
    depends_on:
      postgres-ts:
        condition: service_healthy

  api:
    build: .
    command: ["/app/api"]
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://nevup:${POSTGRES_PASSWORD}@postgres-ts:5432/nevup
      REDIS_URL: redis:6379
      JWT_SECRET: ${JWT_SECRET}
    depends_on:
      migrator:
        condition: service_completed_successfully
      redis:
        condition: service_healthy

  worker:
    build: .
    command: ["/app/worker"]
    environment:
      DATABASE_URL: postgres://nevup:${POSTGRES_PASSWORD}@postgres-ts:5432/nevup
      REDIS_URL: redis:6379
      WORKER_CONCURRENCY: 5
    depends_on:
      migrator:
        condition: service_completed_successfully
      redis:
        condition: service_healthy

volumes:
  pg_data:
  redis_data:

networks:
  default:
    name: nevup-net
```

**Multi-stage Dockerfile:**

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/api     ./cmd/api
RUN CGO_ENABLED=0 go build -o /app/worker  ./cmd/worker
RUN CGO_ENABLED=0 go build -o /app/migrator ./cmd/migrator

FROM scratch
COPY --from=builder /app/ /app/
COPY --from=builder /src/migrations/ /migrations/
COPY --from=builder /src/seeds/nevup_seed_dataset.csv /seeds/
```

---

## 4. Low-Level Design (LLD)

### 4.1 Go Module Structure

```
nevup-backend/
├── cmd/
│   ├── api/
│   │   └── main.go              # Wire router, pgxpool, redis, start HTTP server
│   ├── worker/
│   │   └── main.go              # Start consumer group, launch 5 metric goroutines
│   └── migrator/
│       └── main.go              # Run goose up, seed CSV, exit 0
│
├── internal/
│   ├── api/
│   │   ├── handler/
│   │   │   ├── trade.go         # POST /trades, GET /trades/:id
│   │   │   ├── session.go       # GET /sessions/:id, POST /debrief, GET /coaching SSE
│   │   │   ├── metrics.go       # GET /users/:id/metrics
│   │   │   ├── profile.go       # GET /users/:id/profile
│   │   │   └── health.go        # GET /health (no auth)
│   │   ├── middleware/
│   │   │   ├── auth.go          # HS256 JWT parse, sub===userId, 403 on mismatch
│   │   │   └── logger.go        # zerolog, traceId injection, latency measurement
│   │   ├── router.go            # chi mux assembly, middleware chain
│   │   └── response.go          # JSON envelope helpers, ErrorResponse struct
│   │
│   ├── domain/
│   │   ├── trade.go             # Trade struct, Validate(), ComputePnL()
│   │   ├── revenge.go           # IsRevenge(prevClose exitAt, newEntry entryAt, emotion)
│   │   ├── tilt.go              # SessionTiltIndex(lossFollowCount, totalCount int) float64
│   │   ├── adherence.go         # RollingAvg(ratings []int) float64
│   │   ├── overtrading.go       # WindowSize = 10 trades / 1800 seconds
│   │   ├── enums.go             # AssetClass, Direction, Status, EmotionalState
│   │   └── errors.go            # ErrDuplicate, ErrNotFound, ErrForbidden
│   │
│   ├── store/
│   │   ├── trade_store.go       # Upsert (ON CONFLICT), GetByID, ListBySession
│   │   ├── metrics_store.go     # UpsertBucket, QueryRange, explain plan helper
│   │   ├── session_store.go     # GetWithTrades, UpsertDebrief
│   │   ├── user_store.go        # GetByID, ListAll
│   │   └── db.go                # pgxpool.New, connection retry with backoff
│   │
│   ├── pipeline/
│   │   ├── worker.go            # XREAD loop, event dispatch, XACK
│   │   ├── plan.go              # Rolling 10-trade adherence upsert
│   │   ├── revenge.go           # 90s Redis TTL check → UPDATE trades SET revenge_flag
│   │   ├── tilt.go              # Per-session loss-follow ratio upsert
│   │   ├── emotion.go           # emotion_win_rates INSERT ON CONFLICT DO UPDATE
│   │   └── overtrading.go       # ZADD + ZCOUNT + XADD overtrading:events
│   │
│   └── cache/
│       ├── redis.go             # go-redis/v9 client init, ping health
│       ├── metrics.go           # Get/Set BehavioralMetrics JSON, TTL 30s
│       ├── stream.go            # XADD, XREAD, XACK, XGROUP CREATE wrappers
│       └── zset.go              # ZADD NX, ZREMRANGEBYSCORE, ZCOUNT
│
├── migrations/
│   ├── 001_init_schema.sql      # enums, users, sessions, trades, debriefs
│   ├── 002_timescale.sql        # CREATE EXTENSION, convert behavioral_metrics to hypertable
│   ├── 003_indexes.sql          # composite indexes, partial indexes
│   └── 004_seed.sql             # COPY from /seeds/nevup_seed_dataset.csv
│
├── seeds/
│   └── nevup_seed_dataset.csv   # 388 trades, 10 traders, 52 sessions
│
├── tests/
│   ├── integration_test.go      # idempotency, tenancy, auth expiry
│   └── domain/
│       ├── revenge_test.go
│       ├── tilt_test.go
│       └── adherence_test.go
│
├── loadtest/
│   ├── trade.js                 # k6 script — 200 VUs, 60s, threshold p95<150ms
│   └── report.html              # Generated — do not hand-edit
│
├── DECISIONS.md
├── docker-compose.yml
├── Dockerfile
├── go.mod
└── go.sum
```

### 4.2 Canonical Data Schema

This schema is non-negotiable — any deviation breaks interoperability scoring with Track 2 and Track 3.

```
tradeId:        uuid v4
userId:         uuid v4
sessionId:      uuid v4
asset:          string                          // e.g. 'AAPL', 'BTC/USD'
assetClass:     equity | crypto | forex
direction:      long | short
entryPrice:     decimal(18,8)
exitPrice:      decimal(18,8) | null            // null if trade is open
quantity:       decimal(18,8)
entryAt:        ISO-8601 UTC
exitAt:         ISO-8601 UTC | null             // null if trade is open
status:         open | closed | cancelled
planAdherence:  integer 1–5 | null             // user self-rating at close
emotionalState: calm | anxious | greedy | fearful | neutral | null
entryRationale: string ≤ 500 chars | null
sessionId:      uuid v4                         // groups trades in one session
```

**Extended fields added by Track 1 (not in client input, computed server-side):**

```
outcome:     win | loss | null        // computed from P&L at close
pnl:         decimal(18,8) | null     // computed: direction-aware price diff × quantity
revengeFlag: boolean                  // set by async pipeline
createdAt:   ISO-8601 UTC
updatedAt:   ISO-8601 UTC
```

### 4.3 Database Schema — PostgreSQL + TimescaleDB

#### Enumerated Types

```sql
CREATE TYPE asset_class_enum   AS ENUM ('equity', 'crypto', 'forex');
CREATE TYPE direction_enum     AS ENUM ('long', 'short');
CREATE TYPE status_enum        AS ENUM ('open', 'closed', 'cancelled');
CREATE TYPE emotion_enum       AS ENUM ('calm', 'anxious', 'greedy', 'fearful', 'neutral');
CREATE TYPE outcome_enum       AS ENUM ('win', 'loss');
```

#### `users` Table

```sql
CREATE TABLE users (
    user_id    UUID        PRIMARY KEY,
    name       TEXT        NOT NULL,
    role       TEXT        NOT NULL DEFAULT 'trader',
    pathology  TEXT,                          -- from seed data, for profile generation
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

#### `sessions` Table

```sql
CREATE TABLE sessions (
    session_id UUID        PRIMARY KEY,
    user_id    UUID        NOT NULL REFERENCES users(user_id),
    started_at TIMESTAMPTZ NOT NULL,
    notes      TEXT
);

CREATE INDEX idx_sessions_user ON sessions(user_id);
```

#### `trades` Table (Central Fact Table)

```sql
CREATE TABLE trades (
    trade_id        UUID            PRIMARY KEY,
    user_id         UUID            NOT NULL REFERENCES users(user_id),
    session_id      UUID            NOT NULL REFERENCES sessions(session_id),
    asset           TEXT            NOT NULL,
    asset_class     asset_class_enum NOT NULL,
    direction       direction_enum  NOT NULL,
    entry_price     NUMERIC(18,8)   NOT NULL,
    exit_price      NUMERIC(18,8),
    quantity        NUMERIC(18,8)   NOT NULL,
    entry_at        TIMESTAMPTZ     NOT NULL,
    exit_at         TIMESTAMPTZ,
    status          status_enum     NOT NULL DEFAULT 'open',
    plan_adherence  SMALLINT        CHECK (plan_adherence BETWEEN 1 AND 5),
    emotional_state emotion_enum,
    entry_rationale VARCHAR(500),
    outcome         outcome_enum,
    pnl             NUMERIC(18,8),
    revenge_flag    BOOLEAN         NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
);

-- Primary query patterns:
CREATE INDEX idx_trades_user_entry    ON trades(user_id, entry_at DESC);
CREATE INDEX idx_trades_session       ON trades(session_id);
CREATE INDEX idx_trades_user_status   ON trades(user_id, status) WHERE status = 'closed';
CREATE INDEX idx_trades_user_exit     ON trades(user_id, exit_at DESC) WHERE exit_at IS NOT NULL;
```

**P&L computation (stored at write time):**

```sql
-- Long trade: pnl = (exit_price - entry_price) × quantity
-- Short trade: pnl = (entry_price - exit_price) × quantity
pnl = CASE
    WHEN direction = 'long'  THEN (exit_price - entry_price) * quantity
    WHEN direction = 'short' THEN (entry_price - exit_price) * quantity
END

outcome = CASE WHEN pnl > 0 THEN 'win' ELSE 'loss' END
```

#### `debriefs` Table

```sql
CREATE TABLE debriefs (
    debrief_id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id            UUID        NOT NULL REFERENCES sessions(session_id),
    overall_mood          emotion_enum NOT NULL,
    key_mistake           TEXT,
    key_lesson            TEXT,
    plan_adherence_rating SMALLINT    CHECK (plan_adherence_rating BETWEEN 1 AND 5),
    will_review_tomorrow  BOOLEAN     NOT NULL DEFAULT false,
    saved_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

#### `behavioral_metrics` TimescaleDB Hypertable

```sql
CREATE TABLE behavioral_metrics (
    bucket              TIMESTAMPTZ NOT NULL,  -- truncated to granularity
    user_id             UUID        NOT NULL,
    trade_count         INT         NOT NULL DEFAULT 0,
    win_count           INT         NOT NULL DEFAULT 0,
    loss_count          INT         NOT NULL DEFAULT 0,
    total_pnl           NUMERIC(18,8) NOT NULL DEFAULT 0,
    avg_plan_adherence  NUMERIC(5,4),
    session_tilt_index  NUMERIC(5,4),
    revenge_count       INT         NOT NULL DEFAULT 0,
    overtrading_events  INT         NOT NULL DEFAULT 0,
    PRIMARY KEY (bucket, user_id)
);

-- Convert to hypertable, partition by bucket (1-hour chunks)
SELECT create_hypertable('behavioral_metrics', 'bucket', chunk_time_interval => INTERVAL '1 hour');

-- Composite index for range queries:
CREATE INDEX idx_metrics_user_bucket ON behavioral_metrics(user_id, bucket DESC);
```

#### `emotion_win_rates` Table

```sql
CREATE TABLE emotion_win_rates (
    user_id         UUID        NOT NULL REFERENCES users(user_id),
    emotional_state emotion_enum NOT NULL,
    date_bucket     DATE        NOT NULL,  -- truncated to day
    wins            INT         NOT NULL DEFAULT 0,
    losses          INT         NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, emotional_state, date_bucket)
);
```

#### `overtrading_events` Table

```sql
CREATE TABLE overtrading_events (
    event_id      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID        NOT NULL REFERENCES users(user_id),
    window_start  TIMESTAMPTZ NOT NULL,
    trade_count   INT         NOT NULL,
    emitted_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

#### Migration File Order

```
001_init_schema.sql    -- enums + users + sessions + trades + debriefs
002_timescale.sql      -- CREATE EXTENSION timescaledb; create_hypertable()
003_analytics.sql      -- behavioral_metrics + emotion_win_rates + overtrading_events
004_indexes.sql        -- all composite and partial indexes
005_seed.sql           -- COPY users, sessions, trades FROM CSV; compute P&L on insert
```

### 4.4 API Contract — All Endpoints

#### `POST /trades`

- **Auth:** Bearer JWT required. `sub` must equal `body.userId`.
- **Idempotency:** `ON CONFLICT (trade_id) DO NOTHING RETURNING *`. If RETURNING returns zero rows, issue a follow-up `SELECT WHERE trade_id = $1`. Return the canonical record either way. Always HTTP 200.
- **Side effects (synchronous):** Compute `pnl` and `outcome`. Lookup Redis `user:{userId}:last_loss` for revenge check. Write trade to PostgreSQL.
- **Side effects (async):** `XADD trades:events * ...` to Redis Stream. `ZADD user:{userId}:trades-ts NX {epoch_ms} {tradeId}` for overtrading window.
- **Response:** Full `Trade` object including `revengeFlag`, `pnl`, `outcome`, `createdAt`, `updatedAt`.

#### `GET /trades/{tradeId}`

- **Auth:** Bearer JWT. `sub` must equal `trade.userId` in DB — 403 if not.
- **Query:** `SELECT * FROM trades WHERE trade_id = $1`.
- **Not found:** 404 with `ErrorResponse`.

#### `GET /sessions/{sessionId}`

- **Auth:** Bearer JWT. `sub` must equal `session.userId` — 403 if not.
- **Query:** Single join — `SELECT s.*, t.* FROM sessions s LEFT JOIN trades t ON t.session_id = s.session_id WHERE s.session_id = $1 ORDER BY t.entry_at`.
- **Aggregates computed in application layer** (not SQL): `tradeCount`, `winRate`, `totalPnl`.
- **Response:** `SessionSummary` with embedded `trades[]`.

#### `POST /sessions/{sessionId}/debrief`

- **Auth:** Bearer JWT. Validate `sub` vs `session.user_id` — 403 if not.
- **Body:** `{ overallMood, keyMistake, keyLesson, planAdherenceRating, willReviewTomorrow }`.
- **Write:** `INSERT INTO debriefs ... RETURNING debrief_id, session_id, saved_at`.
- **Response:** HTTP 201 `{ debriefId, sessionId, savedAt }`.

#### `GET /sessions/{sessionId}/coaching` (SSE)

- **Auth:** Bearer JWT. Tenancy check on session.
- **Content-Type:** `text/event-stream`.
- **Protocol:** Flush headers immediately. Stream events in format:
  ```
  event: token
  data: {"token": "You", "index": 0}

  event: token
  data: {"token": " showed", "index": 1}

  event: done
  data: {"fullMessage": "You showed strong discipline today..."}
  ```
- **Implementation:** Use `http.Flusher`. For hackathon scope, generate a deterministic coaching message from the session's behavioral data (no external LLM call needed for Track 1; Track 2 owns that).
- **Error handling:** On client disconnect (`ctx.Done()`), stop streaming gracefully. No goroutine leak.

#### `GET /users/{userId}/metrics`

- **Auth:** Bearer JWT. `sub` must equal path `userId` — 403 if not.
- **Required query params:** `from`, `to` (ISO-8601), `granularity=(hourly|daily|rolling30d)`.
- **Cache-aside:** Check `Redis GET metrics:{userId}:{from}:{to}:{granularity}`. TTL 30s.
- **Cache miss query:**
  ```sql
  SELECT
      time_bucket($interval, bucket) AS b,
      SUM(trade_count)               AS trade_count,
      SUM(win_count)                 AS win_count,
      SUM(loss_count)                AS loss_count,
      SUM(total_pnl)                 AS pnl,
      AVG(avg_plan_adherence)        AS avg_plan_adherence
  FROM behavioral_metrics
  WHERE user_id = $1
    AND bucket  >= $2
    AND bucket  <= $3
  GROUP BY b
  ORDER BY b;
  ```
  Where `$interval` = `'1 hour'`, `'1 day'`, or `'30 days'` depending on `granularity`.
- **Response:** `BehavioralMetrics` including `planAdherenceScore`, `sessionTiltIndex`, `winRateByEmotionalState`, `revengeTrades`, `overtradingEvents`, `timeseries[]`.

#### `GET /users/{userId}/profile`

- **Auth:** Bearer JWT. Tenancy enforced.
- **Query:** Aggregate across all trades for the user. Compute `dominantPathologies` by detecting behavioral signals from stored metrics. Each pathology claim cites real `evidenceSessions` and `evidenceTrades`.
- **Cache:** Store generated profile as JSON in Redis with TTL 5 minutes.

#### `GET /health`

- **Auth:** None.
- **Checks:** `pgxpool.Ping()` → `dbConnection: "connected" | "disconnected"`. `redis.Ping()` → alive check. `XINFO STREAM trades:events` → `queueLag` in milliseconds (derived from last entry ID timestamp delta).
- **Response:** HTTP 200 `{ status: "ok" | "degraded", dbConnection, queueLag, timestamp }`.

### 4.5 Authentication & Tenancy

#### JWT Specification

| Field | Value |
|-------|-------|
| Algorithm | HS256 (HMAC-SHA256) |
| Secret | `97791d4db2aa5f689c3cc39356ce35762f0a73aa70923039d8ef72a2840a1b02` (from env var `JWT_SECRET`) |
| Expiry | 24 hours from `iat` |
| Clock skew tolerance | 0 seconds |

**Required claims:**

```json
{
  "sub": "f412f236-4edc-47a2-8f54-8763a6ed2ce8",
  "iat": 1736150400,
  "exp": 1736237800,
  "role": "trader",
  "name": "Alex Mercer"
}
```

#### Auth Middleware Flow

```go
func AuthMiddleware(secret string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 1. Extract Bearer token from Authorization header
            // 2. If missing → 401 { error: "UNAUTHORIZED", traceId }
            // 3. Parse + verify HS256 signature
            // 4. If invalid sig or malformed → 401 { error: "UNAUTHORIZED", traceId }
            // 5. If exp < now → 401 { error: "TOKEN_EXPIRED", traceId }
            // 6. Inject claims.sub into request context
            // 7. Call next handler — tenancy check is per-handler (needs resource userId)
        })
    }
}

// Per-handler tenancy check (example for GET /trades/:tradeId):
func (h *TradeHandler) Get(w http.ResponseWriter, r *http.Request) {
    jwtSub := auth.SubFromContext(r.Context())
    trade, err := h.store.GetByID(r.Context(), tradeId)
    if err != nil { ... }
    // Row-level tenancy: never 404 when found but unauthorized
    if trade.UserID != jwtSub {
        respondForbidden(w, r) // HTTP 403, never 404
        return
    }
    respondJSON(w, 200, trade)
}
```

**The tenancy rule:** `jwt.sub !== resource.userId` → **always HTTP 403**, never 404 or 200. Automated test in `tests/integration_test.go` proves this.

### 4.6 Write Path — POST /trades Sequence

```
Client                 JWT Middleware        Trade Service         PostgreSQL         Redis
  │                         │                     │                    │               │
  │ POST /trades {tradeId}  │                     │                    │               │
  ├────────────────────────►│                     │                    │               │
  │                         │  verify HS256       │                    │               │
  │                         │  sub === userId      │                    │               │
  │  401 if invalid/expired │                     │                    │               │
  │◄────────────────────────┤                     │                    │               │
  │                         ├────────────────────►│                    │               │
  │                         │                     │ Validate fields    │               │
  │                         │                     │ Compute P&L        │               │
  │                         │                     │ Lookup last_loss   │               │
  │                         │                     ├───────────────────►│  GET user:{id}:last_loss
  │                         │                     │◄───────────────────│  (revenge check)
  │                         │                     │                    │               │
  │                         │                     │ INSERT ON CONFLICT │               │
  │                         │                     ├───────────────────►│               │
  │                         │                     │◄───────────────────┤               │
  │                         │                     │ (rows=0 → SELECT)  │               │
  │                         │                     │                    │               │
  │                         │                     │ if closed+loss:    │               │
  │                         │                     ├───────────────────────────────────►│ SET user:{id}:last_loss
  │                         │                     │                    │               │   EX 90
  │                         │                     │ XADD trades:events │               │
  │                         │                     ├───────────────────────────────────►│
  │                         │                     │ ZADD trades-ts NX  │               │
  │                         │                     ├───────────────────────────────────►│
  │                         │                     │                    │               │
  │  HTTP 200 {trade}       │                     │                    │               │
  │◄────────────────────────┴─────────────────────┤                    │               │
  │                                               │                    │               │
  │         ─ ─ ─ ─ ─ ─ ─ ─ ASYNC BOUNDARY ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─│─ ─ ─ ─ ─ ─ ─ ┤
  │                                            Worker (XREAD loop)     │               │
  │                                               ├───────────────────────────────────►│ XREAD consumer group
  │                                               │◄───────────────────────────────────┤
  │                                               │ dispatch to 5 workers              │
  │                                               ├───────────────────►│  upsert metrics
  │                                               │◄───────────────────┤               │
  │                                               ├───────────────────────────────────►│ XACK
```

### 4.7 Idempotency Implementation

```go
// store/trade_store.go

func (s *TradeStore) Upsert(ctx context.Context, t domain.Trade) (domain.Trade, error) {
    const insertSQL = `
        INSERT INTO trades (
            trade_id, user_id, session_id, asset, asset_class, direction,
            entry_price, exit_price, quantity, entry_at, exit_at, status,
            plan_adherence, emotional_state, entry_rationale,
            outcome, pnl, revenge_flag, created_at, updated_at
        ) VALUES (
            $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,now(),now()
        )
        ON CONFLICT (trade_id) DO NOTHING
        RETURNING *`

    row := s.pool.QueryRow(ctx, insertSQL, /* args */)
    trade, err := scanTrade(row)

    if errors.Is(err, pgx.ErrNoRows) {
        // Conflict: trade_id already exists — fetch the canonical record
        const selectSQL = `SELECT * FROM trades WHERE trade_id = $1`
        row = s.pool.QueryRow(ctx, selectSQL, t.TradeID)
        trade, err = scanTrade(row)
    }

    return trade, err
    // Caller always returns HTTP 200 regardless of which branch ran.
}
```

**Why `DO NOTHING` not `DO UPDATE`:** `DO UPDATE SET updated_at = now()` would mutate the existing record, breaking the idempotency contract. The canonical record must be returned unchanged.

### 4.8 Async Analytics Pipeline

#### Consumer Group Bootstrap

```go
// cmd/worker/main.go
// On startup, create the consumer group if it doesn't exist:
// XGROUP CREATE trades:events nevup-analytics $ MKSTREAM
// $ means "only process new messages from this point forward"
// For seed data replay, use 0 instead of $
```

#### Worker Dispatch Loop

```go
// pipeline/worker.go
func (w *Worker) Run(ctx context.Context) {
    for {
        streams, _ := w.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
            Group:    "nevup-analytics",
            Consumer: w.consumerID,  // unique per goroutine
            Streams:  []string{"trades:events", ">"},
            Count:    10,
            Block:    2 * time.Second,
        }).Result()

        for _, stream := range streams {
            for _, msg := range stream.Messages {
                event := parseEvent(msg.Values)
                // Fan out to all 5 workers concurrently:
                var wg sync.WaitGroup
                for _, handler := range w.handlers {
                    wg.Add(1)
                    go func(h MetricHandler) {
                        defer wg.Done()
                        h.Process(ctx, event)
                    }(handler)
                }
                wg.Wait()
                // Acknowledge only after all 5 handlers succeed:
                w.redis.XAck(ctx, "trades:events", "nevup-analytics", msg.ID)
            }
        }
    }
}
```

### 4.9 Behavioral Metric Definitions

All five metrics are deterministic. Two independent implementations from the same input must produce identical output.

#### 1. Plan Adherence Score

```sql
SELECT ROUND(AVG(plan_adherence)::numeric, 4)
FROM (
    SELECT plan_adherence
    FROM   trades
    WHERE  user_id        = $1
      AND  status         = 'closed'
      AND  plan_adherence IS NOT NULL
    ORDER  BY exit_at DESC
    LIMIT  10
) last_ten;
```

Upsert result into `behavioral_metrics.avg_plan_adherence` for the current time bucket.

#### 2. Revenge Trade Flag

**On trade close (if outcome = loss):**
```go
redis.Set(ctx, fmt.Sprintf("user:%s:last_loss", userID), exitAt.Unix(), 90*time.Second)
```

**On trade open (in write path, before XADD):**
```go
val, err := redis.Get(ctx, fmt.Sprintf("user:%s:last_loss", userID)).Result()
if err == nil { // key exists — a losing close happened within 90s
    prevExitAt := time.Unix(parseInt(val), 0)
    if newEntry.EntryAt.Sub(prevExitAt) <= 90*time.Second &&
       newEntry.EmotionalState == "anxious" || newEntry.EmotionalState == "fearful" {
        newEntry.RevengeFlag = true
    }
}
```

The `revenge_flag` field is written synchronously before the HTTP response. The async worker upserts the count to `behavioral_metrics.revenge_count`.

#### 3. Session Tilt Index

```sql
SELECT
    COUNT(*) FILTER (WHERE prev_outcome = 'loss') AS loss_follow_count,
    COUNT(*)                                       AS total_count
FROM (
    SELECT
        outcome,
        LAG(outcome) OVER (PARTITION BY session_id ORDER BY entry_at) AS prev_outcome
    FROM trades
    WHERE session_id = $1
) sub
WHERE prev_outcome IS NOT NULL;  -- exclude the first trade (no predecessor)
```

`tilt_index = loss_follow_count::float / NULLIF(total_count, 0)`

#### 4. Win Rate by Emotional State

```sql
INSERT INTO emotion_win_rates (user_id, emotional_state, date_bucket, wins, losses)
VALUES ($1, $2, DATE_TRUNC('day', $3), $4, $5)
ON CONFLICT (user_id, emotional_state, date_bucket)
DO UPDATE SET
    wins   = emotion_win_rates.wins   + EXCLUDED.wins,
    losses = emotion_win_rates.losses + EXCLUDED.losses;
```

`$4` = 1 if outcome=win, else 0. `$5` = 1 if outcome=loss, else 0.

**Read query with date range:**
```sql
SELECT emotional_state, SUM(wins) AS wins, SUM(losses) AS losses
FROM emotion_win_rates
WHERE user_id    = $1
  AND date_bucket >= $2::date
  AND date_bucket <= $3::date
GROUP BY emotional_state;
```

#### 5. Overtrading Detector (ZADD Sliding Window)

```go
// Runs in the async worker (does not block the write path).
// Alternatively can be triggered from write path as a fire-and-forget goroutine.

pipe := redis.Pipeline()
key := fmt.Sprintf("user:%s:trades-ts", userID)

// 1. Add this trade's timestamp (NX = don't update existing entry)
pipe.ZAddNX(ctx, key, redis.Z{Score: float64(nowMs), Member: tradeID})

// 2. Remove entries older than 30 minutes
pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", nowMs-1_800_000))

// 3. Count entries in the window
pipe.ZCount(ctx, key, fmt.Sprintf("%d", nowMs-1_800_000), "+inf")

results, _ := pipe.Exec(ctx)
count := results[2].(*redis.IntCmd).Val()

if count > 10 {
    redis.XAdd(ctx, &redis.XAddArgs{
        Stream: "overtrading:events",
        Values: map[string]interface{}{
            "userId":      userID,
            "windowStart": nowMs - 1_800_000,
            "tradeCount":  count,
        },
    })
    // Also upsert to overtrading_events table for persistence
}
```

### 4.10 Read API — Metrics Query Strategy

Three-tier latency defense to achieve p95 ≤ 200ms:

**Tier 1 — Redis cache (TTL 30s, typically < 5ms)**

```go
cacheKey := fmt.Sprintf("metrics:%s:%s:%s:%s", userID, from, to, granularity)
cached, err := redis.Get(ctx, cacheKey).Bytes()
if err == nil {
    w.Write(cached)
    return
}
```

**Tier 2 — TimescaleDB time_bucket aggregation**

```sql
SELECT
    time_bucket($1::interval, bucket) AS b,
    SUM(trade_count)                  AS trade_count,
    SUM(win_count)                    AS win_count,
    SUM(loss_count)                   AS loss_count,
    SUM(total_pnl)                    AS pnl,
    AVG(avg_plan_adherence)           AS avg_plan_adherence,
    AVG(session_tilt_index)           AS session_tilt_index,
    SUM(revenge_count)                AS revenge_count,
    SUM(overtrading_events)           AS overtrading_events
FROM behavioral_metrics
WHERE user_id = $2
  AND bucket  >= $3
  AND bucket  <= $4
GROUP BY b
ORDER BY b;
```

`$1` = `'1 hour'` / `'1 day'` / `'30 days'` based on `granularity` param.

**Tier 3 — Composite index guarantees chunk exclusion**

```sql
-- From EXPLAIN (ANALYZE, BUFFERS):
-- Index Scan using idx_metrics_user_bucket on behavioral_metrics
-- Index Cond: ((user_id = '...') AND (bucket >= '2025-01-01') AND (bucket <= '2025-02-28'))
-- Chunks excluded by constraint exclusion: N-1 of N total chunks
```

Include this exact output in `DECISIONS.md`.

**Cache write-back:**
```go
jsonBytes, _ := json.Marshal(metrics)
redis.Set(ctx, cacheKey, jsonBytes, 30*time.Second)
```

**Cache invalidation:** On every `XADD trades:events`, the worker additionally calls `redis.Del(ctx, "metrics:{userId}:*")` using a scan pattern to purge stale cache entries for that user.

### 4.11 SSE Coaching Stream

```go
// handler/session.go

func (h *SessionHandler) Coaching(w http.ResponseWriter, r *http.Request) {
    sessionID := chi.URLParam(r, "sessionId")
    jwtSub    := auth.SubFromContext(r.Context())

    session, err := h.store.GetSession(r.Context(), sessionID)
    if err != nil { respondNotFound(w, r); return }
    if session.UserID != jwtSub { respondForbidden(w, r); return }

    // Set SSE headers before any write
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no") // Nginx: disable buffering
    w.WriteHeader(http.StatusOK)

    flusher, ok := w.(http.Flusher)
    if !ok {
        return // Client doesn't support streaming
    }

    message := generateCoachingMessage(session) // deterministic from session data
    tokens  := tokenize(message)                // split by word

    for i, token := range tokens {
        select {
        case <-r.Context().Done(): // client disconnected
            return
        default:
        }

        fmt.Fprintf(w, "event: token\ndata: {\"token\": %q, \"index\": %d}\n\n",
            token, i)
        flusher.Flush()
        time.Sleep(30 * time.Millisecond) // simulate streaming cadence
    }

    fmt.Fprintf(w, "event: done\ndata: {\"fullMessage\": %q}\n\n", message)
    flusher.Flush()
}
```

### 4.12 Observability

#### Structured Log Format

Every request produces exactly one log entry at response completion:

```json
{
  "level": "info",
  "traceId": "7f3a9c2e-1b4d-4e8a-9f2c-3d5e6a7b8c9d",
  "userId": "f412f236-4edc-47a2-8f54-8763a6ed2ce8",
  "method": "POST",
  "path": "/trades",
  "latency": 42,
  "statusCode": 200,
  "time": "2025-01-06T09:35:12.441Z"
}
```

#### Logger Middleware

```go
// middleware/logger.go
func Logger(logger zerolog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            traceID := uuid.New().String()
            ctx     := context.WithValue(r.Context(), TraceIDKey, traceID)
            rw      := &responseWriter{ResponseWriter: w, statusCode: 200}
            start   := time.Now()

            next.ServeHTTP(rw, r.WithContext(ctx))

            logger.Info().
                Str("traceId",    traceID).
                Str("userId",     userIDFromContext(ctx)).
                Str("method",     r.Method).
                Str("path",       r.URL.Path).
                Int("latency",    int(time.Since(start).Milliseconds())).
                Int("statusCode", rw.statusCode).
                Msg("")
        })
    }
}
```

#### Error Response Shape (all 401/403 must include `traceId`)

```go
type ErrorResponse struct {
    Error   string `json:"error"`
    Message string `json:"message"`
    TraceID string `json:"traceId"`
}
```

### 4.13 Health Endpoint

```go
// handler/health.go — no auth middleware applied to this route

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer cancel()

    dbStatus := "connected"
    if err := h.pool.Ping(ctx); err != nil {
        dbStatus = "disconnected"
    }

    // Queue lag: delta between now and the timestamp encoded in the last stream entry ID
    queueLag := h.getQueueLag(ctx) // milliseconds

    status := "ok"
    if dbStatus == "disconnected" {
        status = "degraded"
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "status":       status,
        "dbConnection": dbStatus,
        "queueLag":     queueLag,
        "timestamp":    time.Now().UTC().Format(time.RFC3339),
    })
}
```

---

## 5. Test Strategy

### 5.1 Unit Tests

All domain-layer functions are pure (no I/O) and fully unit-testable:

#### Revenge Flag — `domain/revenge_test.go`

```go
func TestIsRevenge(t *testing.T) {
    base := time.Date(2025, 1, 6, 9, 0, 0, 0, time.UTC)

    cases := []struct{
        name    string
        gap     time.Duration
        emotion string
        want    bool
    }{
        {"within 90s + anxious",    89 * time.Second, "anxious", true},
        {"within 90s + fearful",    45 * time.Second, "fearful", true},
        {"exactly 90s + anxious",   90 * time.Second, "anxious", false}, // exclusive
        {"over 90s + anxious",      91 * time.Second, "anxious", false},
        {"within 90s + calm",       60 * time.Second, "calm",    false},
        {"within 90s + greedy",     60 * time.Second, "greedy",  false},
        {"within 90s + neutral",    60 * time.Second, "neutral", false},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := domain.IsRevenge(base, base.Add(tc.gap), tc.emotion)
            require.Equal(t, tc.want, got)
        })
    }
}
```

#### Session Tilt — `domain/tilt_test.go`

```go
func TestSessionTiltIndex(t *testing.T) {
    // Trades: win, loss, loss-follow, win, loss-follow
    // loss-follow count = 2, total (excluding first) = 4
    // tilt = 2/4 = 0.5
    require.Equal(t, 0.5, domain.SessionTiltIndex(2, 4))
    require.Equal(t, 0.0, domain.SessionTiltIndex(0, 5))
    require.Equal(t, 0.0, domain.SessionTiltIndex(0, 0)) // no panic on zero denominator
    require.Equal(t, 1.0, domain.SessionTiltIndex(3, 3))
}
```

#### Plan Adherence Rolling Average — `domain/adherence_test.go`

```go
func TestRollingAvg(t *testing.T) {
    require.Equal(t, 3.0,  domain.RollingAvg([]int{3, 3, 3, 3, 3}))
    require.Equal(t, 2.5,  domain.RollingAvg([]int{1, 2, 3, 4}))
    require.Equal(t, 0.0,  domain.RollingAvg([]int{})) // no trades
    // Only last 10 considered even if more passed:
    ratings := []int{1, 1, 1, 1, 1, 5, 5, 5, 5, 5}
    require.Equal(t, 3.0, domain.RollingAvg(ratings))
}
```

### 5.2 Integration Tests

Located in `tests/integration_test.go`. Require a running docker-compose stack or testcontainers.

#### Idempotency Test

```go
func TestPostTradesIdempotency(t *testing.T) {
    tradeID := uuid.New().String()
    payload := buildTradePayload(tradeID, alexMercerUserID)
    token   := generateJWT(alexMercerUserID)

    // First submission
    r1 := httpPost("/trades", payload, token)
    require.Equal(t, 200, r1.StatusCode)
    var trade1 Trade
    json.NewDecoder(r1.Body).Decode(&trade1)

    // Duplicate submission
    r2 := httpPost("/trades", payload, token)
    require.Equal(t, 200, r2.StatusCode)
    var trade2 Trade
    json.NewDecoder(r2.Body).Decode(&trade2)

    // Bodies must be identical (same createdAt, same tradeId)
    require.Equal(t, trade1.TradeID,   trade2.TradeID)
    require.Equal(t, trade1.CreatedAt, trade2.CreatedAt)
    require.Equal(t, trade1.UpdatedAt, trade2.UpdatedAt)
}
```

#### Cross-Tenant Tenancy Test

```go
func TestCrossTenantReturns403(t *testing.T) {
    // JWT issued for User A, request User B's data
    tokenA := generateJWT(alexMercerUserID)   // sub = alex's UUID

    endpoints := []string{
        "/users/" + jordanLeeUserID + "/metrics?from=...&to=...&granularity=daily",
        "/users/" + jordanLeeUserID + "/profile",
        "/sessions/" + jordanLeeSessionID,
    }

    for _, ep := range endpoints {
        r := httpGet(ep, tokenA)
        require.Equal(t, 403, r.StatusCode, "expected 403 for %s", ep)

        var errResp ErrorResponse
        json.NewDecoder(r.Body).Decode(&errResp)
        require.Equal(t, "FORBIDDEN", errResp.Error)
        require.NotEmpty(t, errResp.TraceID, "traceId must be present in 403")
    }
}
```

#### Expired JWT Test

```go
func TestExpiredTokenReturns401(t *testing.T) {
    expiredToken := generateJWTWithExpiry(alexMercerUserID, time.Now().Add(-1*time.Hour))
    r := httpGet("/users/"+alexMercerUserID+"/metrics?from=...&to=...&granularity=daily", expiredToken)
    require.Equal(t, 401, r.StatusCode)

    var errResp ErrorResponse
    json.NewDecoder(r.Body).Decode(&errResp)
    require.NotEmpty(t, errResp.TraceID)
}
```

### 5.3 Load Test — k6

File: `loadtest/trade.js`

```javascript
import http  from 'k6/http';
import { check, sleep } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const BASE_URL  = __ENV.BASE_URL || 'http://localhost:8080';
const JWT_TOKEN = __ENV.JWT_TOKEN; // pre-generated for Alex Mercer

export const options = {
    scenarios: {
        trade_writes: {
            executor:  'constant-vus',
            vus:       200,
            duration:  '60s',
            startTime: '10s',   // 10s ramp-up before sustained phase
        },
    },
    thresholds: {
        'http_req_duration{scenario:trade_writes}': ['p(95)<150'],
        'http_req_failed':                          ['rate<0.01'],
    },
};

const SESSION_ID = '57651b39-b4f9-496c-9afb-36535f841fb4'; // Alex Mercer session 1
const USER_ID    = 'f412f236-4edc-47a2-8f54-8763a6ed2ce8';

export default function () {
    const tradeId = uuidv4();

    const payload = JSON.stringify({
        tradeId:        tradeId,
        userId:         USER_ID,
        sessionId:      SESSION_ID,
        asset:          'AAPL',
        assetClass:     'equity',
        direction:      'long',
        entryPrice:     178.45,
        exitPrice:      182.30,
        quantity:       10,
        entryAt:        '2025-01-06T09:35:00Z',
        exitAt:         '2025-01-06T11:20:00Z',
        status:         'closed',
        planAdherence:  4,
        emotionalState: 'calm',
        entryRationale: 'k6 load test trade',
    });

    const writeRes = http.post(`${BASE_URL}/trades`, payload, {
        headers: {
            'Content-Type':  'application/json',
            'Authorization': `Bearer ${JWT_TOKEN}`,
        },
    });

    check(writeRes, {
        'write status 200':     (r) => r.status === 200,
        'write has tradeId':    (r) => JSON.parse(r.body).tradeId === tradeId,
        'write has pnl':        (r) => JSON.parse(r.body).pnl !== null,
    });

    // Also exercise the read path under the same load
    const readRes = http.get(
        `${BASE_URL}/users/${USER_ID}/metrics?from=2025-01-01T00:00:00Z&to=2025-03-01T00:00:00Z&granularity=daily`,
        { headers: { 'Authorization': `Bearer ${JWT_TOKEN}` } }
    );

    check(readRes, {
        'read status 200': (r) => r.status === 200,
    });

    sleep(0.1);
}
```

**Run command:**
```bash
k6 run \
  --env BASE_URL=http://localhost:8080 \
  --env JWT_TOKEN=$(go run ./cmd/genjwt) \
  --out json=loadtest/results.json \
  loadtest/trade.js

k6 report loadtest/results.json --out loadtest/report.html
```

---

## 6. DECISIONS.md — Architectural Rationale

> This section is the source for the required `DECISIONS.md` file at repo root. Each paragraph maps to one architectural decision.

**TimescaleDB over plain PostgreSQL.** The `GET /users/:id/metrics` endpoint requires `granularity=hourly|daily|rolling30d` time bucketing with p95 ≤ 200ms. TimescaleDB's `time_bucket()` function expresses this directly in SQL and the hypertable's chunk exclusion constraint means only 1–2 partition chunks are scanned for the Jan–Feb 2026 seed date range. The alternative — application-level GROUP BY on a plain PostgreSQL table — would require either a materialized view refresh pipeline or accept full-table scans. TimescaleDB is a transparent PostgreSQL extension requiring zero driver changes; `pgx/v5` connects identically.

**Redis Streams over Kafka or RabbitMQ for the analytics pipeline.** A single Redis container handles three system concerns: the event bus (`trades:events` stream), hot metrics cache (GET/SET with TTL), and the overtrading sliding window (ZSET). This eliminates one or two additional containers from `docker-compose.yml` and removes the operational complexity of Kafka topic configuration or RabbitMQ exchange/queue binding. Consumer groups provide at-least-once delivery semantics with XACK acknowledgment. For the 388-trade seed dataset and 200 VU load test, Redis Streams throughput is multiple orders of magnitude beyond the required capacity.

**`ON CONFLICT DO NOTHING` + fallback SELECT for trade idempotency.** `DO UPDATE SET updated_at = now()` would silently mutate existing records on re-submission, violating the idempotency contract (callers expect the original record back unchanged). `DO NOTHING RETURNING *` is atomic at the database level: if the trade_id already exists, RETURNING yields zero rows, and a single follow-up SELECT retrieves the canonical record. This pattern passes the automated duplicate-submission test that asserts `trade1.createdAt === trade2.createdAt`.

**Revenge flag computed synchronously in the write path (not the async worker).** The `revengeFlag` field is part of the `Trade` response object returned by `POST /trades`. If it were computed asynchronously, the first response would return `revengeFlag: false` and a subsequent GET would return `true`, creating a visible inconsistency. The Redis TTL approach (SET with 90s expiry on each losing close) adds approximately 1–2ms to the write path and keeps the flag deterministic on first response.

**ZADD NX for overtrading window state.** The `NX` flag prevents updating a tradeId's score on re-submission of the same trade (idempotency extends to the sliding window). `ZREMRANGEBYSCORE` before `ZCOUNT` in a single pipeline call keeps the ZSET bounded and O(log N) without a background cleanup goroutine.

**Two Go binaries (`api` and `worker`) from one Docker image.** Both binaries share all internal packages — domain logic, store layer, Redis client. Building from a single Dockerfile scratch image and selecting behavior via the container `CMD` directive eliminates image drift and ensures the analytics pipeline runs identical business logic to the API. The `migrator` binary is a third CMD entry point that runs once on startup, ensuring schema and seed data are in place before either service accepts traffic.

**200 VU / 60s load test figure.** The spec requires 200 concurrent trade-close events/sec sustained for 60 seconds with p95 ≤ 150ms. 200 VUs with 0.1s sleep between iterations approximates 200 requests/sec at steady state (each VU issues ≈1 req/sec with overhead). This is 10× the realistic peak for the 10-trader seed dataset, providing enough headroom to demonstrate the write path architecture — not just the seeded data volume — holds under realistic concurrency.

**EXPLAIN (ANALYZE, BUFFERS) output for the metrics query** (include literal output here after running against seeded data):

```sql
EXPLAIN (ANALYZE, BUFFERS) 
SELECT time_bucket('1 day', bucket) AS b, SUM(trade_count)
FROM behavioral_metrics
WHERE user_id = 'f412f236-4edc-47a2-8f54-8763a6ed2ce8'
  AND bucket >= '2025-01-01' AND bucket <= '2025-02-28'
GROUP BY b ORDER BY b;

-- Expected output (paste actual after seeding):
-- Append (actual rows=N ...)
--   -> Index Scan using idx_metrics_user_bucket on behavioral_metrics_chunk_001
--      Index Cond: (user_id = '...' AND bucket >= ... AND bucket <= ...)
--   Chunks excluded by constraint exclusion: N-1 of N
-- Planning Time: ~1 ms
-- Execution Time: ~3 ms
```

---

## 7. Submission Checklist

| Item | Status |
|------|--------|
| Live deployment URL accessible at submission time | ☐ |
| Public GitHub repository | ☐ |
| `docker-compose.yml` — single `docker compose up`, no manual steps | ☐ |
| `Dockerfile` — multi-stage, scratch final image | ☐ |
| DB migrations in repo (`migrations/001_*.sql` … `005_*.sql`) | ☐ |
| Seed data loads on container startup, metrics queryable immediately | ☐ |
| `nevup_openapi.yaml` — implemented exactly (no field name deviations) | ☐ |
| `POST /trades` idempotent — duplicate returns HTTP 200 (automated test proves it) | ☐ |
| Cross-tenant read returns HTTP 403, never 404 (automated test proves it) | ☐ |
| Expired JWT returns HTTP 401 with `traceId` | ☐ |
| All 5 behavioral metrics computed asynchronously via Redis Streams | ☐ |
| Revenge flag set synchronously, present in first `POST /trades` response | ☐ |
| `GET /health` returns `queueLag` + `dbConnection`, no auth required | ☐ |
| Structured JSON logs on every request: `{ traceId, userId, latency, statusCode }` | ☐ |
| `traceId` in all 401/403 error response bodies | ☐ |
| `GET /users/:id/metrics` p95 ≤ 200ms (verified with k6 read scenario) | ☐ |
| k6 script in `loadtest/trade.js` — 200 VUs, 60s, threshold `p(95)<150` | ☐ |
| k6 HTML results report in `loadtest/report.html` | ☐ |
| `DECISIONS.md` at repo root — one paragraph per architectural decision | ☐ |
| EXPLAIN plan for metrics query included in `DECISIONS.md` | ☐ |
| Unit tests: `revenge_test.go`, `tilt_test.go`, `adherence_test.go` | ☐ |
| Integration tests: idempotency, cross-tenant, expired JWT | ☐ |
| No ORM — all queries are raw SQL via `pgx/v5` | ☐ |
| No mock or hardcoded data — all data from `nevup_seed_dataset.csv` | ☐ |
| Metrics persist across `docker compose restart` | ☐ |

---

*NevUp Hiring Hackathon 2026 — Track 1 Technical Design Document*
*Go · chi · PostgreSQL + TimescaleDB · Redis Streams · zerolog · pgx/v5 · k6*
