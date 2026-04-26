import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';
import { htmlReport } from "https://raw.githubusercontent.com/benc-uk/k6-reporter/main/dist/bundle.js";
import { textSummary } from "https://jslib.k6.io/k6-summary/0.0.1/index.js";

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const JWT_TOKEN = __ENV.JWT_TOKEN;
const USER_ID = 'f412f236-4edc-47a2-8f54-8763a6ed2ce8';
const SESSION_ID = '4f39c2ea-8687-41f7-85a0-1fafd3e976df';

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
    'http_req_duration{name:write}': ['p(95)<150'],
    'http_req_duration{name:read}': ['p(95)<200'],
    errors: ['rate<0.01'],
  },
};

const headers = {
  'Content-Type': 'application/json',
  'Authorization': `Bearer ${JWT_TOKEN}`,
};

// Generate a RFC4122 v4 UUID
function uuidv4() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

// Pre-warm the metrics cache before the load test starts
export function setup() {
  const warmRes = http.get(
    `${BASE_URL}/users/${USER_ID}/metrics?from=2025-01-01T00:00:00Z&to=2025-03-01T00:00:00Z&granularity=daily`,
    { headers }
  );
  console.log(`Cache warm-up: status=${warmRes.status} latency=${warmRes.timings.duration}ms`);
}

export default function () {
  const tradeId = uuidv4();

  // POST /trades
  const tradePayload = JSON.stringify({
    tradeId: tradeId,
    userId: USER_ID,
    sessionId: SESSION_ID,
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

  const postRes = http.post(`${BASE_URL}/trades`, tradePayload, { headers, tags: { name: 'write' } });
  postTradeDuration.add(postRes.timings.duration);

  const postOk = check(postRes, {
    'POST /trades status is 200': (r) => r.status === 200,
  });
  if (!postOk) {
    errorRate.add(1);
    if (__ITER < 3) {
      console.log(`POST /trades failed: status=${postRes.status} body=${postRes.body}`);
    }
  }

  // GET /users/:id/metrics
  const metricsRes = http.get(
    `${BASE_URL}/users/${USER_ID}/metrics?from=2025-01-01T00:00:00Z&to=2025-03-01T00:00:00Z&granularity=daily`,
    { headers, tags: { name: 'read' } }
  );
  getMetricsDuration.add(metricsRes.timings.duration);

  check(metricsRes, {
    'GET /metrics status is 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  sleep(0.1);
}

export function handleSummary(data) {
  return {
    "loadtest/report.html": htmlReport(data),
    stdout: textSummary(data, { indent: " ", enableColors: true }),
  };
}

