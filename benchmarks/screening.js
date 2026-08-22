// A proportional 90-second screening profile for candidate promotion.
// This is deliberately separate from k6/grading.js, which must remain untouched.

import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

const BASE = __ENV.TARGET || 'http://localhost:8080';
const SYMBOLS = ['AAPL', 'GOOG', 'MSFT', 'AMZN', 'NVDA', 'META', 'TSLA', 'JPM'];
const pick = (values) => values[Math.floor(Math.random() * values.length)];
const workScore = new Counter('work_score');

export const options = {
  scenarios: {
    screen: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '20s', target: 50 },
        { duration: '40s', target: 100 },
        { duration: '20s', target: 200 },
        { duration: '10s', target: 0 },
      ],
    },
  },
  thresholds: {
    'http_req_duration{tier:price}': ['p(95)<200'],
    'http_req_duration{tier:stats}': ['p(95)<500'],
    'http_req_duration{tier:risk}': ['p(95)<1500'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  const draw = Math.random();
  let response;
  let weight;
  let tier;

  if (draw < 0.6) {
    tier = 'price';
    weight = 1;
    response = http.get(`${BASE}/price?symbol=${pick(SYMBOLS)}`, { tags: { tier, name: 'price' } });
  } else if (draw < 0.9) {
    tier = 'stats';
    weight = 3;
    response = http.get(`${BASE}/stats?symbol=${pick(SYMBOLS)}`, { tags: { tier, name: 'stats' } });
  } else {
    tier = 'risk';
    weight = 10;
    response = http.get(`${BASE}/risk?seed=${Math.random()}`, { tags: { tier, name: 'risk' } });
  }

  const correctStatus = check(response, { 'status is 200': (result) => result.status === 200 });
  if (correctStatus) workScore.add(weight);
}
