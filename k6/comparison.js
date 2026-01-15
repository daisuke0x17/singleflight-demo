import http from 'k6/http';
import { check, sleep } from 'k6';

// This test compares both endpoints side by side
// Run this to see the difference in real-time on Grafana

export const options = {
  scenarios: {
    // Phase 1: Test without singleflight
    without_sf: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '5s', target: 50 },   // Ramp up
        { duration: '20s', target: 50 },  // Stay at 50 VUs
        { duration: '5s', target: 0 },    // Ramp down
      ],
      exec: 'testWithoutSingleflight',
      startTime: '0s',
    },
    // Phase 2: Test with singleflight (starts after phase 1)
    with_sf: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '5s', target: 50 },   // Ramp up
        { duration: '20s', target: 50 },  // Stay at 50 VUs
        { duration: '5s', target: 0 },    // Ramp down
      ],
      exec: 'testWithSingleflight',
      startTime: '35s',
    },
  },
};

export function setup() {
  console.log('='.repeat(60));
  console.log('Singleflight Comparison Test');
  console.log('='.repeat(60));
  console.log('Phase 1 (0-30s): WITHOUT singleflight - watch DB calls spike');
  console.log('Phase 2 (35-65s): WITH singleflight - watch DB calls stay low');
  console.log('='.repeat(60));
  return {};
}

export function testWithoutSingleflight() {
  // Clear cache periodically to trigger stampede
  if (__ITER % 50 === 0) {
    http.get('http://app:8080/api/clear-cache');
  }

  const res = http.get('http://app:8080/api/without-singleflight');
  check(res, {
    'without-sf: status 200': (r) => r.status === 200,
  });

  sleep(0.1);
}

export function testWithSingleflight() {
  // Clear cache periodically to trigger (protected) stampede
  if (__ITER % 50 === 0) {
    http.get('http://app:8080/api/clear-cache');
  }

  const res = http.get('http://app:8080/api/with-singleflight');
  check(res, {
    'with-sf: status 200': (r) => r.status === 200,
  });

  sleep(0.1);
}

export function teardown(data) {
  console.log('='.repeat(60));
  console.log('Test completed!');
  console.log('Open Grafana at http://localhost:3000');
  console.log('Compare the "DB Calls Rate" panel between the two phases');
  console.log('='.repeat(60));
}
