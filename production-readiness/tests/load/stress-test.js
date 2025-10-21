// k6 Load Test - Stress Test Scenario
// Push system to 1000+ concurrent users to find breaking point

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const errorRate = new Rate('errors');

export const options = {
  stages: [
    { duration: '2m', target: 1000 },  // Ramp up to 1000 users
    { duration: '5m', target: 1000 },  // Sustain 1000 users
    { duration: '2m', target: 1500 },  // Push to 1500 users
    { duration: '5m', target: 1500 },  // Sustain 1500 users
    { duration: '5m', target: 0 },     // Ramp down
  ],
  thresholds: {
    'http_req_duration': ['p(95)<300'], // Allow degraded performance
    'errors': ['rate<0.10'],            // Allow 10% error rate under stress
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const TEST_EMAIL = 'loadtest@example.com';
const TEST_PASSWORD = 'LoadTest123!';

export default function() {
  const loginRes = http.post(`${BASE_URL}/api/v1/auth/login/local`, JSON.stringify({
    email: TEST_EMAIL,
    password: TEST_PASSWORD,
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  check(loginRes, {
    'login success': (r) => r.status === 200,
  }) || errorRate.add(1);

  if (loginRes.status !== 200) {
    sleep(1);
    return;
  }

  const token = loginRes.json('accessToken');

  // Rapid requests to stress the system
  http.batch([
    ['GET', `${BASE_URL}/api/v1/agents`, null, { headers: { 'Authorization': `Bearer ${token}` } }],
    ['GET', `${BASE_URL}/api/v1/analytics/dashboard`, null, { headers: { 'Authorization': `Bearer ${token}` } }],
    ['GET', `${BASE_URL}/api/v1/mcp-servers`, null, { headers: { 'Authorization': `Bearer ${token}` } }],
  ]);

  sleep(0.5); // Shorter sleep for higher load
}
