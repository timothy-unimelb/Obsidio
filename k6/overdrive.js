// Obsidio OVERDRIVE script: the devloop at DOUBLE the grading peak (400 VUs).
// Purpose: the judged resilience exhibit, not a score benchmark. The locked
// grading script may peak higher than 200 VUs and the grader's hardware may
// hash slower than ours — this script simulates that regime and asks one
// question: does the admission gate DEGRADE (bars still pass, score keeps
// accruing) or COLLAPSE (error storm / p95 blowout = disqualification)?
//
//   k6 run -e TARGET=http://127.0.0.1:8080 k6/overdrive.js
//
// RULES OF USE:
//   - Side runs only: NEVER record results in bench/history.jsonl (they are
//     not comparable to grading.js or devloop numbers).
//   - Compare only overdrive-to-overdrive, same machine, back to back.
//   - The graded thresholds are kept so the pass/fail verdict is automatic:
//     k6 exits non-zero = that build self-DQs at 2× overload.

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
        { duration: '10s', target: 400 }, // fast ramp to 2× grading peak
        { duration: '60s', target: 400 }, // sustained overload: the exhibit
        { duration: '5s',  target: 0 },   // drain
      ],
    },
  },

  // Same bars as grading.js: crossing one here = self-DQ at 2× overload.
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
