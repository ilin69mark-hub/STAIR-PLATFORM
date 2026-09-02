// k6 load test for Health endpoint
// Usage: k6 run tests/load/health_test.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const healthDuration = new Trend('health_duration');

export const options = {
  stages: [
    { duration: '10s', target: 5 },
    { duration: '30s', target: 50 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(99)<100'], // 99% under 100ms
    errors: ['rate<0.01'],            // <1% errors
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  const res = http.get(`${BASE_URL}/api/v1/health`);

  check(res, {
    'status is 200': (r) => r.status === 200,
    'response time < 50ms': (r) => r.timings.duration < 50,
  });

  errorRate.add(res.status !== 200);
  healthDuration.add(res.timings.duration);

  sleep(0.05);
}
