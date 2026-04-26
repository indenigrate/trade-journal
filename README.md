# NevUp Track 1: Trade Journal Engine

Backend implementation for the NevUp Hiring Hackathon 2026

## Submission Documents

* **Architecture Decisions**: [DECISIONS.md](DECISIONS.md)
* **API Specification**: [must_follow_docs/nevup_openapi.yaml](must_follow_docs/nevup_openapi.yaml)
* **Load Test Script**: [loadtest/trade.js](loadtest/trade.js)
* **Load Test Report**:
  * [View Rendered HTML (GitHub Pages)](https://indenigrate.github.io/trade-journal/loadtest/report.html)
  * [View Source (GitHub Repo)](https://github.com/indenigrate/trade-journal/blob/main/loadtest/report.html)

## Deployment details

* **Public Repository**: [https://github.com/indenigrate/trade-journal](https://github.com/indenigrate/trade-journal)
* **Live Deployment URL**: [https://nevup.apnadomain.qzz.io](https://nevup.apnadomain.qzz.io)

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

## Load Test Performance (Local vs Deployment)

The load test results generated in the `report.html` artifact primarily reflect the performance of the live remote deployment. Due to real-world internet transit routing and hardware constraints on EC2, local environment tests inherently register vastly faster latencies. Yet, both environments comfortably smash the structural constraints mandated by the hackathon:

* **Local**: Write p(95) ~25ms | Read p(95) ~5ms | ~1,860 RPS 
* **Deployment (EC2)**: Write p(95) ~115ms | Read p(95) ~75ms | ~878 RPS

Even burdened with physical internet transit latency to an AWS domain, the live deployment massively outperformed the `150ms` maximum Write threshold and the `200ms` maximum Read threshold, whilst sustaining zero dropped requests.
## Hackathon Compromises & Caveats

To strictly satisfy the requirement that the stack boots with a single `docker compose up -d` command with *zero* manual intervention on a cloned repository, the following architectural shortcuts were taken:

* **Pushed `.env` file**: Environment variables containing secrets (like `JWT_SECRET` and `POSTGRES_PASSWORD`) have been pushed directly to the public repository. This is a massive anti-pattern in production environments but guarantees the one-click, zero-config reviewer experience mandated by the submission guidelines.
* **Local NGINX proxy**: Instead of managing dedicated load balancer infrastructure (like AWS ALB or Cloudflare), an NGINX container is bundled directly within the docker-compose topology to securely proxy traffic and handle SSE requirements on port 80.
* **Co-located Database**: PostgreSQL/TimescaleDB and Redis run as containers continuously alongside the API rather than utilizing robust managed services (e.g., AWS RDS or ElastiCache), drastically simplifying the local deployment topology for the evaluator's `docker compose` teardown.
