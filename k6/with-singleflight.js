import http from 'k6/http';
import { check, sleep } from 'k6';

// This test demonstrates Singleflight protection
// With singleflight, only 1 request will hit the DB during cache miss

export const options = {
  scenarios: {
    // 1000 VUsが同時に1リクエストずつ送信 = 一斉スパイク
    singleflight_protection: {
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
  const res = http.get('http://app:8080/api/with-singleflight');

  check(res, {
    'status is 200': (r) => r.status === 200,
    'has response body': (r) => r.body.length > 0,
  });
}

export function teardown(data) {
  console.log('Test completed: with-singleflight');
  console.log('Check Grafana dashboard - DB calls should be ~1 instead of ~1000');
}
