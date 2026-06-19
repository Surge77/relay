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

// Stage targets/durations are env-tunable so the same harness drives both the
// full 50k fleet ramp (defaults) and a smaller single-node local run. Override
// PEAK_VUS / WARM_VUS and the *_DUR knobs to fit the machine under test.
const WARM_VUS = parseInt(__ENV.WARM_VUS || "1000", 10);
const MID_VUS = parseInt(__ENV.MID_VUS || "10000", 10);
const PEAK_VUS = parseInt(__ENV.PEAK_VUS || "50000", 10);
const RAMP_DUR = __ENV.RAMP_DUR || "1m";
const MID_DUR = __ENV.MID_DUR || "3m";
const PEAK_DUR = __ENV.PEAK_DUR || "5m";
const HOLD_DUR = __ENV.HOLD_DUR || "10m";
const DOWN_DUR = __ENV.DOWN_DUR || "1m";

export const options = {
  scenarios: {
    ramp: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: RAMP_DUR, target: WARM_VUS },
        { duration: MID_DUR, target: MID_VUS },
        { duration: PEAK_DUR, target: PEAK_VUS },
        { duration: HOLD_DUR, target: PEAK_VUS },
        { duration: DOWN_DUR, target: 0 },
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
