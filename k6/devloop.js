// Obsidio DEV-LOOP script: a ~75-second version of grading.js for fast
// iteration. Identical request mix, tags, and work_score metric — but it jumps
// straight to the 200-VU steady state instead of the 3-minute ramp, because
// that hold is where all the signal is.
//
//   k6 run -e TARGET=http://127.0.0.1:8080 k6/devloop.js
//
// RULES OF USE:
//   - Dev-loop numbers are ONLY comparable to other dev-loop numbers.
//     Never mix them with grading.js runs or record them in bench/history.jsonl.
//   - Directional tool: trust big deltas, re-check close ones with the full
//     grading script before claiming or recording anything.
//   - p95s here read ~2x HIGHER than grading.js p95s: this script samples only
//     peak load, while grading.js blends 3 minutes of light ramp into its
//     percentiles. A threshold "failure" here means "confirm with grading.js",
//     not "we fail the bar". work_score is the metric to iterate on
//     (measured dev-loop noise: ~3% run-to-run vs ~10% for the full script).

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
        { duration: '10s', target: 200 }, // fast ramp straight to peak
        { duration: '60s', target: 200 }, // steady-state hold: the signal
        { duration: '5s',  target: 0 },   // drain
      ],
    },
  },

  // Same bars as grading.js so a busted threshold still screams.
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
