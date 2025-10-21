// k6 Load Test - Normal Load Scenario
// 100 concurrent users for 5 minutes

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');

export const options = {
  stages: [
    { duration: '2m', target: 100 },  // Ramp up to 100 users
    { duration: '5m', target: 100 },  // Stay at 100 users
    { duration: '2m', target: 0 },    // Ramp down
  ],
  thresholds: {
    'http_req_duration': ['p(95)<100'], // 95% of requests < 100ms
    'errors': ['rate<0.01'],            // Error rate < 1%
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Test credentials (should be created before test)
const TEST_EMAIL = 'loadtest@example.com';
const TEST_PASSWORD = 'LoadTest123!';

export default function() {
  // 1. Login
  const loginRes = http.post(`${BASE_URL}/api/v1/auth/login/local`, JSON.stringify({
    email: TEST_EMAIL,
    password: TEST_PASSWORD,
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  check(loginRes, {
    'login status is 200': (r) => r.status === 200,
    'login has access token': (r) => r.json('accessToken') !== undefined,
  }) || errorRate.add(1);

  if (loginRes.status !== 200) {
    sleep(1);
    return;
  }

  const token = loginRes.json('accessToken');

  // 2. List agents
  const agentsRes = http.get(`${BASE_URL}/api/v1/agents?page=1&limit=20`, {
    headers: { 'Authorization': `Bearer ${token}` },
  });

  check(agentsRes, {
    'agents status is 200': (r) => r.status === 200,
    'agents response time < 100ms': (r) => r.timings.duration < 100,
  }) || errorRate.add(1);

  // 3. Get dashboard stats
  const dashboardRes = http.get(`${BASE_URL}/api/v1/analytics/dashboard`, {
    headers: { 'Authorization': `Bearer ${token}` },
  });

  check(dashboardRes, {
    'dashboard status is 200': (r) => r.status === 200,
    'dashboard response time < 200ms': (r) => r.timings.duration < 200,
  }) || errorRate.add(1);

  // 4. Get specific agent (if any exist)
  const agents = agentsRes.json('agents');
  if (agents && agents.length > 0) {
    const agentId = agents[0].id;
    const agentDetailRes = http.get(`${BASE_URL}/api/v1/agents/${agentId}`, {
      headers: { 'Authorization': `Bearer ${token}` },
    });

    check(agentDetailRes, {
      'agent detail status is 200': (r) => r.status === 200,
      'agent detail response time < 50ms': (r) => r.timings.duration < 50,
    }) || errorRate.add(1);
  }

  sleep(1);
}
