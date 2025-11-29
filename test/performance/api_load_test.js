import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 10 },  // Ramp up to 10 users
    { duration: '1m', target: 10 },   // Stay at 10 users
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests should be below 500ms
    http_req_failed: ['rate<0.01'],   // Less than 1% failure rate
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

export default function () {
  // Health check endpoint
  const healthRes = http.get(`${BASE_URL}/healthz`);
  check(healthRes, {
    'health check status is 200': (r) => r.status === 200,
  });

  // System status endpoint
  const statusRes = http.get(`${BASE_URL}/system/status`);
  check(statusRes, {
    'system status is 200': (r) => r.status === 200,
    'system status has modules': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.modules !== undefined;
      } catch {
        return false;
      }
    },
  });

  // Readiness endpoint
  const readyRes = http.get(`${BASE_URL}/readyz`);
  check(readyRes, {
    'readiness check status is 200': (r) => r.status === 200,
  });

  sleep(1);
}

export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
  };
}

function textSummary(data, opts) {
  const indent = opts.indent || '';
  let summary = `${indent}Performance Test Summary\n`;
  summary += `${indent}========================\n\n`;

  if (data.metrics) {
    summary += `${indent}HTTP Request Duration:\n`;
    if (data.metrics.http_req_duration) {
      const d = data.metrics.http_req_duration.values;
      summary += `${indent}  avg: ${d.avg?.toFixed(2) || 'N/A'}ms\n`;
      summary += `${indent}  p95: ${d['p(95)']?.toFixed(2) || 'N/A'}ms\n`;
      summary += `${indent}  max: ${d.max?.toFixed(2) || 'N/A'}ms\n`;
    }

    summary += `\n${indent}HTTP Requests:\n`;
    if (data.metrics.http_reqs) {
      summary += `${indent}  total: ${data.metrics.http_reqs.values.count || 0}\n`;
      summary += `${indent}  rate: ${data.metrics.http_reqs.values.rate?.toFixed(2) || 'N/A'}/s\n`;
    }

    if (data.metrics.http_req_failed) {
      summary += `\n${indent}Failures:\n`;
      summary += `${indent}  rate: ${(data.metrics.http_req_failed.values.rate * 100)?.toFixed(2) || 0}%\n`;
    }
  }

  return summary;
}
