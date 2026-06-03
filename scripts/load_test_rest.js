import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const latency = new Trend('http_latency', true);

export const options = {
  stages: [
    { duration: '30s', target: 1000 },
    { duration: '1m',  target: 3000 },
    { duration: '30s', target: 5000 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<200'],
    errors: ['rate<0.01'],
  },
};

const BASE = 'http://localhost:9999';

export default function () {
  // Rotate through key endpoints
  const endpoints = [
    { method: 'GET',  url: `${BASE}/api/status` },
    { method: 'GET',  url: `${BASE}/api/health` },
    { method: 'GET',  url: `${BASE}/api/channels` },
    { method: 'GET',  url: `${BASE}/api/peers` },
    { method: 'GET',  url: `${BASE}/api/mesh/stats` },
    { method: 'GET',  url: `${BASE}/api/nostr/stats` },
  ];

  const ep = endpoints[__ITER % endpoints.length];
  const start = Date.now();

  const res = http.request(ep.method, ep.url);

  latency.add(Date.now() - start);

  check(res, {
    'status 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  sleep(0.01);
}
