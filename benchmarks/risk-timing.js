import http from 'k6/http';
import { Trend } from 'k6/metrics';

const BASE = __ENV.TARGET || 'http://localhost:8080';
const SYMBOLS = ['AAPL', 'GOOG', 'MSFT', 'AMZN', 'NVDA', 'META', 'TSLA', 'JPM'];
const pick = (values) => values[Math.floor(Math.random() * values.length)];

const riskQueue = new Trend('risk_queue_ms', true);
const riskHash = new Trend('risk_hash_ms', true);

export const options = {
  scenarios: {
    peakSlice: {
      executor: 'constant-vus',
      vus: 200,
      duration: '30s',
    },
  },
  summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'max'],
};

export default function () {
  const random = Math.random();

  if (random < 0.6) {
    http.get(`${BASE}/price?symbol=${pick(SYMBOLS)}`, { tags: { tier: 'price', name: 'price' } });
    return;
  }
  if (random < 0.9) {
    http.get(`${BASE}/stats?symbol=${pick(SYMBOLS)}`, { tags: { tier: 'stats', name: 'stats' } });
    return;
  }

  const response = http.get(`${BASE}/risk?seed=${Math.random()}`, { tags: { tier: 'risk', name: 'risk' } });
  const timing = response.headers['Server-Timing'] || '';
  const match = /risk_queue;dur=([0-9.]+), risk_hash;dur=([0-9.]+)/.exec(timing);
  if (match) {
    riskQueue.add(Number(match[1]));
    riskHash.add(Number(match[2]));
  }
}
