import http from 'k6/http';
import { check, sleep } from 'k6';

// This test demonstrates Cache Stampede
// Without singleflight, each request during cache miss will hit the DB

export const options = {
  scenarios: {
    // 1000 VUsが同時に1リクエストずつ送信 = 一斉スパイク
    cache_stampede: {
      executor: 'per-vu-iterations',
      vus: 1000,          // 1000 concurrent users
      iterations: 1,      // Each VU sends 1 request
      maxDuration: '30s',
    },
  },
};

export function setup() {
  // Clear cache before the test
  const clearRes = http.get('http://app:8080/api/clear-cache');
  console.log('Cache cleared before test');
  return {};
}

export default function () {
  const res = http.get('http://app:8080/api/without-singleflight');

  check(res, {
    'status is 200': (r) => r.status === 200,
    'has response body': (r) => r.body.length > 0,
  });
}

export function teardown(data) {
  console.log('Test completed: without-singleflight');
  console.log('Check Grafana dashboard to see DB calls spike');
}
