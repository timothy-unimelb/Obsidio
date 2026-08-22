// Overload stress profile for the admission gate. NOT a grading workload and
// never publishable: it ramps to 4x the published peak to show how the server
// degrades when /risk arrivals exceed capacity. Same mix, same thresholds.
import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

const BASE = __ENV.TARGET || 'http://localhost:8080';
const SYMBOLS = ['AAPL', 'GOOG', 'MSFT', 'AMZN', 'NVDA', 'META', 'TSLA', 'JPM'];
const pick = (arr) => arr[Math.floor(Math.random() * arr.length)];
const workScore = new Counter('work_score');

export const options = {
  scenarios: {
    siege: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '20s', target: 200 },
        { duration: '40s', target: 800 },
        { duration: '20s', target: 800 },
        { duration: '10s', target: 0 },
      ],
    },
  },
  thresholds: {
    'http_req_duration{tier:price}': ['p(95)<200'],
    'http_req_duration{tier:stats}': ['p(95)<500'],
    'http_req_duration{tier:risk}':  ['p(95)<1500'],
    'http_req_failed':               ['rate<0.01'],
  },
};

export default function () {
  const r = Math.random();
  let res, weight, tier;
  if (r < 0.6) {
    tier = 'price'; weight = 1;
    res = http.get(`${BASE}/price?symbol=${pick(SYMBOLS)}`, { tags: { tier, name: 'price' } });
  } else if (r < 0.9) {
    tier = 'stats'; weight = 3;
    res = http.get(`${BASE}/stats?symbol=${pick(SYMBOLS)}`, { tags: { tier, name: 'stats' } });
  } else {
    tier = 'risk'; weight = 10;
    res = http.get(`${BASE}/risk?seed=${Math.random()}`, { tags: { tier, name: 'risk' } });
  }
  const ok = check(res, { 'status is 200': (resp) => resp.status === 200 });
  if (ok) workScore.add(weight);
}
