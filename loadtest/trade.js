import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const JWT_TOKEN = __ENV.JWT_TOKEN;
const USER_ID = 'f412f236-4edc-47a2-8f54-8763a6ed2ce8';

const errorRate = new Rate('errors');
const postTradeDuration = new Trend('post_trade_duration');
const getMetricsDuration = new Trend('get_metrics_duration');

export const options = {
  stages: [
    { duration: '10s', target: 50 },
    { duration: '40s', target: 200 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<150'],
    errors: ['rate<0.01'],
  },
};

const headers = {
  'Content-Type': 'application/json',
  'Authorization': `Bearer ${JWT_TOKEN}`,
};

export default function () {
  const tradeId = `load-test-${__VU}-${__ITER}-${Date.now()}`;

  // POST /trades
  const tradePayload = JSON.stringify({
    tradeId: tradeId,
    userId: USER_ID,
    sessionId: '4f39c2ea-8687-41f7-85a0-1fafd3e976df',
    asset: 'AAPL',
    assetClass: 'equity',
    direction: 'long',
    entryPrice: 150.50,
    exitPrice: 155.00,
    quantity: 10,
    entryAt: '2025-03-01T10:00:00Z',
    exitAt: '2025-03-01T11:00:00Z',
    status: 'closed',
  });

  const postRes = http.post(`${BASE_URL}/trades`, tradePayload, { headers, tags: { name: 'POST /trades' } });
  postTradeDuration.add(postRes.timings.duration);

  check(postRes, {
    'POST /trades status is 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  // GET /users/:id/metrics
  const metricsRes = http.get(
    `${BASE_URL}/users/${USER_ID}/metrics?from=2025-01-01T00:00:00Z&to=2025-03-01T00:00:00Z&granularity=daily`,
    { headers, tags: { name: 'GET /metrics' } }
  );
  getMetricsDuration.add(metricsRes.timings.duration);

  check(metricsRes, {
    'GET /metrics status is 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  sleep(0.1);
}
