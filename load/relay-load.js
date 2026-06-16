// k6 load harness for Relay. Ramps WebSocket connections, has each VU subscribe
// to a conversation and periodically send a message, and records send→ack and
// send→delivery latency.
//
// Usage:
//   k6 run load/relay-load.js \
//     -e WS_ADDR=ws://localhost:8080/ws \
//     -e TOKEN=<a dev JWT> \
//     -e CONV=general
//
// Mint a token with: cd gateway && go run ./cmd/relayctl -user load -conv general
// -secret $JWT_SECRET  (it prints nothing useful for tokens directly — instead
// issue one in a tiny Go snippet or expose a dev /token endpoint). For the 50k
// run, generate per-VU tokens and pass via a data file.

import ws from "k6/ws";
import { check } from "k6";
import { Trend } from "k6/metrics";

const ackLatency = new Trend("relay_ack_ms", true);

const WS_ADDR = __ENV.WS_ADDR || "ws://localhost:8080/ws";
const TOKEN = __ENV.TOKEN || "";
const CONV = __ENV.CONV || "general";

export const options = {
  scenarios: {
    ramp: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "1m", target: 1000 },
        { duration: "3m", target: 10000 },
        { duration: "5m", target: 50000 },
        { duration: "10m", target: 50000 },
        { duration: "1m", target: 0 },
      ],
      gracefulStop: "30s",
    },
  },
};

export default function () {
  const url = `${WS_ADDR}?token=${TOKEN}`;
  const sent = {};

  ws.connect(url, {}, function (socket) {
    socket.on("open", function () {
      socket.send(JSON.stringify({ type: "subscribe", conversation_id: CONV, last_acked_seq: 0 }));
      socket.setInterval(function () {
        const id = `${__VU}-${Date.now()}`;
        sent[id] = Date.now();
        socket.send(JSON.stringify({ type: "send", conversation_id: CONV, client_msg_id: id, body: "load" }));
      }, 1000);
    });

    socket.on("message", function (raw) {
      const f = JSON.parse(raw);
      if (f.type === "ack" && sent[f.client_msg_id]) {
        ackLatency.add(Date.now() - sent[f.client_msg_id]);
        delete sent[f.client_msg_id];
      }
    });

    socket.setTimeout(function () {
      socket.close();
    }, 60000);
  });

  check(null, { connected: () => true });
}
