# NevUp Track 1: Trade Journal Engine

Backend implementation for the NevUp Hiring Hackathon 2026

## Submission Documents

* **Architecture Decisions**: [DECISIONS.md](DECISIONS.md)
* **API Specification**: [must_follow_docs/nevup_openapi.yaml](must_follow_docs/nevup_openapi.yaml)
* **Load Test Script**: [loadtest/trade.js](loadtest/trade.js)
* **Load Test Report**: [loadtest/report.html](loadtest/report.html)

## Deployment details

* **Public Repository**: [Insert GitHub Repository URL]
* **Live Deployment URL**: [Insert Live URL]

## Startup

Run the complete pipeline utilizing the included Docker configuration. Zero manual interventions are necessary.

```bash
docker compose up -d
```

## Overview

A high-throughput trade journaling system constructed in Go. It handles idempotent trade submissions, JWT-based tenant separation, and computes real-time behavioral metrics utilizing a TimescaleDB background pipeline.

Key technical specifications enforced:
* Framework: Go 1.23, utilizing `chi` for routing
* Authorization: HS256 JWT, complete row-level isolation
* Storage: PostgreSQL with TimescaleDB for metrics
* Caching and Messaging: Redis Streams for asynchronous workloads
* Observability: Structured zerolog outputs with trace identifiers per request

Load testing results confirm API write latency significantly under the required parameter constraints limit at peak concurrency. Reference the load test report for specific request metrics.
