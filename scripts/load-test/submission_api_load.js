import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    ramp_up: {
      executor: 'ramping-vus',
      stages: [
        { duration: '30s', target: 10 },
        { duration: '60s', target: 50 },
        { duration: '30s', target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_duration: ['p99<2000'],
    http_req_failed: ['rate<0.01'],
  },
};

const BASE = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  let res = http.get(`${BASE}/v1/health`);
  check(res, { 'health ok': (r) => r.status === 200 });
  res = http.get(`${BASE}/v1/leaderboard`);
  check(res, { 'leaderboard ok': (r) => r.status === 200 });
  sleep(1);
}
