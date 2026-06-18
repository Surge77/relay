"use client";

import { useCallback, useEffect, useRef } from "react";
import type { Frame } from "./protocol";
import { useChatStore } from "./store";
import { useAuthStore, refreshSession } from "./auth";

const DEFAULT_GATEWAY_WS = process.env.NEXT_PUBLIC_GATEWAY_WS ?? "ws://localhost:8080/ws";
const MAX_BACKOFF_MS = 30_000;
const PING_INTERVAL_MS = 10_000;

// gatewayURL lets a single web origin drive any node: a ?gw=ws://host:port/ws
// query param overrides the default, so two browser tabs can connect to two
// different gateway nodes to demonstrate cross-node fan-out.
function gatewayURL(): string {
  if (typeof window !== "undefined") {
    const override = new URLSearchParams(window.location.search).get("gw");
    if (override) return override;
  }
  return DEFAULT_GATEWAY_WS;
}

interface UseChatSocket {
  send: (body: string) => void;
  sendTyping: (start: boolean) => void;
  sendRead: (seq: number) => void;
}

// useChatSocket owns the WebSocket lifecycle for one user + conversation: auth
// handshake, exponential-backoff reconnect (capped, guarded against the
// intentional-close race), and resume-by-cursor on every (re)open. It is the
// single source of socket truth; React reads chat state from the store.
export function useChatSocket(user: string, conversation: string): UseChatSocket {
  const ws = useRef<WebSocket | null>(null);
  const backoff = useRef(1000);
  const isClosed = useRef(false);
  const pingTimer = useRef<ReturnType<typeof setInterval> | null>(null);

  const store = useChatStore;

  const connect = useCallback(async () => {
    if (isClosed.current) return;
    store.getState().setStatus(ws.current ? "reconnecting" : "connecting");

    // Use the in-memory access token; if absent or stale, exchange the refresh
    // cookie for a fresh one. A null result means we are not authenticated.
    let token = useAuthStore.getState().accessToken;
    if (!token) token = await refreshSession();
    if (!token) {
      store.getState().setStatus("down");
      return;
    }
    if (isClosed.current) return; // unmounted while awaiting the token

    const socket = new WebSocket(`${gatewayURL()}?token=${token}`);
    ws.current = socket;
    // Ignore events from a socket that has since been superseded by a reconnect.
    const isCurrent = () => ws.current === socket;

    socket.onopen = () => {
      if (!isCurrent()) return;
      backoff.current = 1000;
      store.getState().setStatus("connected");
      const cursor = store.getState().cursors[conversation] ?? 0;
      sendFrame(socket, { type: "subscribe", conversation_id: conversation, last_acked_seq: cursor });
      if (pingTimer.current) clearInterval(pingTimer.current);
      pingTimer.current = setInterval(() => sendFrame(socket, { type: "ping" }), PING_INTERVAL_MS);
    };

    socket.onmessage = (ev) => {
      if (!isCurrent()) return;
      let frame: Frame;
      try {
        frame = JSON.parse(ev.data) as Frame;
      } catch {
        return; // ignore malformed frame
      }
      handleFrame(conversation, frame);
    };

    socket.onclose = () => {
      if (!isCurrent()) return;
      if (pingTimer.current) clearInterval(pingTimer.current);
      if (isClosed.current) return;
      store.getState().setStatus("reconnecting");
      scheduleReconnect();
    };

    socket.onerror = () => socket.close();
    // scheduleReconnect is intentionally omitted from deps to avoid a TDZ cycle;
    // it is referenced only at call time, by which point it is initialized.
  }, [user, conversation, store]);

  const scheduleReconnect = useCallback(() => {
    if (isClosed.current) return;
    backoff.current = Math.min(backoff.current * 2, MAX_BACKOFF_MS);
    setTimeout(connect, backoff.current);
  }, [connect]);

  useEffect(() => {
    isClosed.current = false;
    connect();
    return () => {
      isClosed.current = true;
      if (pingTimer.current) clearInterval(pingTimer.current);
      ws.current?.close();
    };
  }, [connect]);

  const send = useCallback(
    (body: string) => {
      const socket = ws.current;
      if (!socket || socket.readyState !== WebSocket.OPEN) return;
      const clientMsgId = crypto.randomUUID();
      store.getState().addOptimistic(conversation, {
        seq: 0,
        senderId: user,
        clientMsgId,
        body,
        ts: Date.now(),
        pending: true,
      });
      sendFrame(socket, { type: "send", conversation_id: conversation, client_msg_id: clientMsgId, body });
    },
    [conversation, user, store],
  );

  const sendTyping = useCallback(
    (start: boolean) => {
      const socket = ws.current;
      if (socket?.readyState === WebSocket.OPEN) {
        sendFrame(socket, { type: "typing", conversation_id: conversation, state: start ? "start" : "stop" });
      }
    },
    [conversation],
  );

  const sendRead = useCallback(
    (seq: number) => {
      const socket = ws.current;
      if (socket?.readyState === WebSocket.OPEN) {
        sendFrame(socket, { type: "read", conversation_id: conversation, seq });
      }
    },
    [conversation],
  );

  return { send, sendTyping, sendRead };
}

function sendFrame(socket: WebSocket, frame: Frame) {
  socket.send(JSON.stringify(frame));
}

function handleFrame(conversation: string, f: Frame) {
  const st = useChatStore.getState();
  switch (f.type) {
    case "message":
      st.applyMessage(conversation, {
        seq: f.seq ?? 0,
        senderId: f.sender_id ?? "",
        clientMsgId: f.client_msg_id ?? "",
        body: f.body ?? "",
        ts: f.ts ?? Date.now(),
        pending: false,
      });
      break;
    case "ack":
      if (f.client_msg_id && typeof f.seq === "number") {
        st.confirmAck(conversation, f.client_msg_id, f.seq);
      }
      break;
    case "caughtup":
      if (typeof f.seq === "number") st.setCursor(conversation, f.seq);
      break;
    case "presence":
      if (f.user_id) st.setPresence(f.user_id, f.state === "online");
      break;
    case "typing":
      if (f.user_id) st.setTyping(f.user_id, f.state === "start");
      break;
    case "receipt":
      if (f.user_id && typeof f.seq === "number") st.setReceipt(conversation, f.user_id, f.seq);
      break;
  }
}
