import ws from 'k6/ws';
import { check } from 'k6';

export const options = { vus: Number(__ENV.VUS || 100), duration: '60s' };
const URL = __ENV.WS_URL || 'ws://localhost:8084/ws';

export default function () {
  const res = ws.connect(URL, {}, function (socket) {
    socket.on('message', function (data) {
      const u = JSON.parse(data);
      check(u, { 'has entries': (x) => Array.isArray(x.entries) });
    });
    socket.setTimeout(() => socket.close(), 30000);
  });
  check(res, { 'status 101': (r) => r && r.status === 101 });
}
