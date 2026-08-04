import { check, sleep } from 'k6';
import ws from 'k6/ws';

export const options = {
  vus: 100, // Reduced from 10k for local test
  duration: '10s',
};

export default function () {
  const url = 'ws://localhost:9001';
  const res = ws.connect(url, {}, function (socket) {
    socket.on('open', () => {
      // Send a dummy MQTT connect packet (hex format)
      socket.send(new Uint8Array([0x10, 0x0c, 0x00, 0x04, 0x4d, 0x51, 0x54, 0x54, 0x04, 0x02, 0x00, 0x3c, 0x00, 0x00]));
    });
    
    socket.on('message', (msg) => {
      // Expect CONNACK
    });

    socket.setTimeout(function () {
      socket.close();
    }, 5000);
  });

  check(res, { 'status is 101': (r) => r && r.status === 101 });
}
