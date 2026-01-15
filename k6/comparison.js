import http from 'k6/http';
import { check, sleep } from 'k6';

// This test compares both endpoints side by side
// Run this to see the difference in real-time on Grafana

export const options = {
  scenarios: {
    // Phase 1: 1000 VUs一斉スパイク（singleflight無し）
    without_sf: {
      executor: 'per-vu-iterations',
      vus: 1000,
      iterations: 1,
      maxDuration: '30s',
      exec: 'testWithoutSingleflight',
      startTime: '0s',
    },
    // Phase 2: 1000 VUs一斉スパイク（singleflight有り）
    with_sf: {
      executor: 'per-vu-iterations',
      vus: 1000,
      iterations: 1,
      maxDuration: '30s',
      exec: 'testWithSingleflight',
      startTime: '10s',
    },
  },
};

export function setup() {
  // 両フェーズ前にキャッシュクリア
  http.get('http://app:8080/api/clear-cache');
  console.log('='.repeat(60));
  console.log('Singleflight Comparison Test');
  console.log('='.repeat(60));
  console.log('Phase 1 (0s):  1000 VUs一斉スパイク WITHOUT singleflight');
  console.log('Phase 2 (10s): 1000 VUs一斉スパイク WITH singleflight');
  console.log('='.repeat(60));
  return {};
}

export function testWithoutSingleflight() {
  const res = http.get('http://app:8080/api/without-singleflight');
  check(res, {
    'without-sf: status 200': (r) => r.status === 200,
  });
}

export function testWithSingleflight() {
  // Phase 2開始前にキャッシュクリア（最初のVUのみ）
  if (__VU === 1) {
    http.get('http://app:8080/api/clear-cache');
  }
  const res = http.get('http://app:8080/api/with-singleflight');
  check(res, {
    'with-sf: status 200': (r) => r.status === 200,
  });
}

export function teardown(data) {
  console.log('='.repeat(60));
  console.log('Test completed!');
  console.log('Open Grafana at http://localhost:3000');
  console.log('Compare the "DB Calls Rate" panel between the two phases');
  console.log('='.repeat(60));
}
