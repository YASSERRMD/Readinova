/**
 * Readinova k6 smoke test
 * Verifies the happy-path API flow under minimal load.
 *
 * Usage:
 *   k6 run tests/k6/smoke.js -e BASE_URL=http://localhost:8080
 */
import http from 'k6/http';
import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';

export const options = {
  vus: 2,
  duration: '30s',
  thresholds: {
    http_req_failed:   ['rate<0.01'],      // < 1% errors
    http_req_duration: ['p(95)<800'],      // 95th percentile < 800ms
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Generate a unique email per iteration to avoid conflicts in the seed org.
let counter = 0;

export default function () {
  counter++;
  const tag = `${Date.now()}_${counter}`;
  const email = `smoke_${tag}@example.com`;
  const password = 'SmokeTest123!';
  const orgName = `Smoke Org ${tag}`;

  // 1. Sign up a new organisation.
  const signupRes = http.post(
    `${BASE_URL}/v1/organisations`,
    JSON.stringify({ name: orgName, email, password }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  check(signupRes, { 'signup 201': (r) => r.status === 201 });
  if (signupRes.status !== 201) {
    sleep(1);
    return;
  }

  const accessToken = signupRes.json('access_token');
  const authHeaders = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${accessToken}`,
    },
  };

  // 2. Get /me.
  const meRes = http.get(`${BASE_URL}/v1/me`, authHeaders);
  check(meRes, { '/me 200': (r) => r.status === 200 });

  // 3. Get subscription.
  const subRes = http.get(`${BASE_URL}/v1/billing/subscription`, authHeaders);
  check(subRes, {
    'subscription 200': (r) => r.status === 200,
    'subscription tier free': (r) => r.json('tier') === 'free',
  });

  // 4. List assessments (empty).
  const listRes = http.get(`${BASE_URL}/v1/assessments`, authHeaders);
  check(listRes, { 'list assessments 200': (r) => r.status === 200 });

  sleep(1);
}
