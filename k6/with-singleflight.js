import http from 'k6/http';
import { check, sleep } from 'k6';

// This test demonstrates Singleflight protection
// With singleflight, only 1 request will hit the DB during cache miss

export const options = {
  scenarios: {
    // Scenario 1: Clear cache and send burst of requests
    singleflight_protection: {
      executor: 'shared-iterations',
      vus: 100,           // 100 concurrent users
      iterations: 100,    // Total 100 requests
      maxDuration: '30s',
      startTime: '0s',
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
  console.log('Check Grafana dashboard - DB calls should be ~1 instead of ~100');
}
