import ws from 'k6/ws';
import { check } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const wsLatency = new Trend('ws_latency', true);

export const options = {
  stages: [
    { duration: '30s', target: 500 },   // ramp up
    { duration: '1m',  target: 2000 },  // sustained
    { duration: '30s', target: 5000 },  // peak
    { duration: '1m',  target: 10000 }, // max
    { duration: '30s', target: 0 },     // ramp down
  ],
  thresholds: {
    errors: ['rate<0.01'],
    ws_latency: ['p(95)<100'],
  },
};

export default function () {
  const url = 'ws://localhost:9999/ws';
  const start = Date.now();

  const res = ws.connect(url, {}, function (socket) {
    socket.on('open', function () {
      const msg = JSON.stringify({
        type: 'chat',
        text: `benchmark-${__VU}-${__ITER}`,
        from: `k6-vu-${__VU}`,
        ts: Math.floor(Date.now() / 1000),
      });
      socket.send(msg);
    });

    socket.on('message', function (msg) {
      wsLatency.add(Date.now() - start);
    });

    socket.setTimeout(function () {
      socket.close();
    }, 10000);
  });

  check(res, {
    'ws connected': (r) => r && r.status === 101,
  }) || errorRate.add(1);
}
