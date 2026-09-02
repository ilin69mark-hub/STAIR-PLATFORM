// k6 load test for STAIR Platform API
// Usage: k6 run tests/load/api_test.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const apiDuration = new Trend('api_duration');
const requestCount = new Counter('requests');

// Options
export const options = {
  stages: [
    { duration: '30s', target: 10 },  // Ramp up to 10 VUs
    { duration: '1m', target: 10 },   // Stay at 10 VUs
    { duration: '30s', target: 20 },  // Ramp up to 20 VUs
    { duration: '2m', target: 20 },   // Stay at 20 VUs
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests under 500ms
    errors: ['rate<0.1'],             // Error rate under 10%
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Test data
const testUser = {
  email: `loadtest-${Date.now()}@example.com`,
  password: 'testpassword123',
  name: 'Load Test User',
};

let authToken = '';

export function setup() {
  // Register and login test user
  const registerRes = http.post(`${BASE_URL}/api/v1/auth/register`, JSON.stringify(testUser), {
    headers: { 'Content-Type': 'application/json' },
  });

  if (registerRes.status === 201 || registerRes.status === 409) {
    const loginRes = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
      email: testUser.email,
      password: testUser.password,
    }), {
      headers: { 'Content-Type': 'application/json' },
    });

    if (loginRes.status === 200) {
      const body = JSON.parse(loginRes.body);
      authToken = body.token || '';
    }
  }

  return { token: authToken };
}

export default function (data) {
  const headers = {
    'Content-Type': 'application/json',
  };

  if (data.token) {
    headers['Authorization'] = `Bearer ${data.token}`;
  }

  // Test health endpoint (no auth)
  const healthRes = http.get(`${BASE_URL}/api/v1/health`);
  check(healthRes, {
    'health status 200': (r) => r.status === 200,
  });
  errorRate.add(healthRes.status !== 200);
  apiDuration.add(healthRes.timings.duration);
  requestCount.add(1);

  sleep(0.1);

  // Test projects list (with auth)
  if (data.token) {
    const projectsRes = http.get(`${BASE_URL}/api/v1/projects`, { headers });
    check(projectsRes, {
      'projects status 200': (r) => r.status === 200,
    });
    errorRate.add(projectsRes.status !== 200);
    apiDuration.add(projectsRes.timings.duration);
    requestCount.add(1);
  }

  sleep(0.1);

  // Test stairs list (with auth)
  if (data.token) {
    const stairsRes = http.get(`${BASE_URL}/api/v1/stairs`, { headers });
    check(stairsRes, {
      'stairs status 200': (r) => r.status === 200,
    });
    errorRate.add(stairsRes.status !== 200);
    apiDuration.add(stairsRes.timings.duration);
    requestCount.add(1);
  }

  sleep(0.2);
}

export function teardown(data) {
  // Cleanup if needed
}
