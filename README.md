# NevUp Track 1: Trade Journal Engine

Backend implementation for the NevUp Hiring Hackathon 2026

## Submission Documents

* **Architecture Decisions**: [DECISIONS.md](DECISIONS.md)
* **API Specification**: [must_follow_docs/nevup_openapi.yaml](must_follow_docs/nevup_openapi.yaml)
* **Load Test Script**: [loadtest/trade.js](loadtest/trade.js)
* **Load Test Report**: [loadtest/report.html](loadtest/report.html)

## Deployment details

* **Public Repository**: [https://github.com/indenigrate/trade-journal](https://github.com/indenigrate/trade-journal)
* **Live Deployment URL**: [http://nevup.apnadomain.qzz.io](http://nevup.apnadomain.qzz.io)

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

## Testing Remote Deployment

You can verify your remote deployment functionality and performance using the included verification script. It will test cross-tenant data boundaries (HTTP 403), general uptime (HTTP 200), and execute the comprehensive `k6` load test against your live URL.

```bash
./test_remote.sh
```

*(Note: The script defaults to `http://nevup.apnadomain.qzz.io`. To test a different URL, prefix the command: `BASE_URL=http://your-domain.com ./test_remote.sh`)*

## Hackathon Compromises & Caveats

To strictly satisfy the requirement that the stack boots with a single `docker compose up -d` command with *zero* manual intervention on a cloned repository, the following architectural shortcuts were taken:

* **Pushed `.env` file**: Environment variables containing secrets (like `JWT_SECRET` and `POSTGRES_PASSWORD`) have been pushed directly to the public repository. This is a massive anti-pattern in production environments but guarantees the one-click, zero-config reviewer experience mandated by the submission guidelines.
* **Local NGINX proxy**: Instead of managing dedicated load balancer infrastructure (like AWS ALB or Cloudflare), an NGINX container is bundled directly within the docker-compose topology to securely proxy traffic and handle SSE requirements on port 80.
* **Co-located Database**: PostgreSQL/TimescaleDB and Redis run as containers continuously alongside the API rather than utilizing robust managed services (e.g., AWS RDS or ElastiCache), drastically simplifying the local deployment topology for the evaluator's `docker compose` teardown.
