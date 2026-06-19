// @vitest-environment jsdom
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";

// Token is present synchronously, so connect() never suspends on refresh — the
// socket is created within the same tick, which keeps fake-timer tests simple.
vi.mock("./auth", () => ({
  useAuthStore: { getState: () => ({ accessToken: "tok" }) },
  refreshSession: vi.fn(async () => "tok"),
}));

import { useChatSocket } from "./useChatSocket";
import { useChatStore } from "./store";

// Minimal WebSocket double: records sent frames and lets tests drive lifecycle
// events. Every instance is captured so tests can assert reconnect creates new
// sockets.
class MockWebSocket {
  static OPEN = 1;
  static instances: MockWebSocket[] = [];
  readyState = 0;
  sent: string[] = [];
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(public url: string) {
    MockWebSocket.instances.push(this);
  }
  send(data: string) {
    this.sent.push(data);
  }
  close() {
    this.readyState = 3;
    this.onclose?.();
  }
  open() {
    this.readyState = MockWebSocket.OPEN;
    this.onopen?.();
  }
}

beforeEach(() => {
  vi.useFakeTimers();
  MockWebSocket.instances = [];
  vi.stubGlobal("WebSocket", MockWebSocket as unknown as typeof WebSocket);
  useChatStore.setState({ status: "connecting", cursors: {} });
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

const latest = () => MockWebSocket.instances[MockWebSocket.instances.length - 1];

describe("useChatSocket connection lifecycle", () => {
  it("marks connected and sends a subscribe frame on open", () => {
    renderHook(() => useChatSocket("alice", "general"));
    act(() => latest().open());

    expect(useChatStore.getState().status).toBe("connected");
    const subscribe = latest().sent.map((s) => JSON.parse(s)).find((f) => f.type === "subscribe");
    expect(subscribe).toMatchObject({ conversation_id: "general", last_acked_seq: 0 });
  });

  it("subscribes from the stored cursor for catch-up", () => {
    useChatStore.setState({ cursors: { general: 42 } });
    renderHook(() => useChatSocket("alice", "general"));
    act(() => latest().open());

    const subscribe = latest().sent.map((s) => JSON.parse(s)).find((f) => f.type === "subscribe");
    expect(subscribe.last_acked_seq).toBe(42);
  });
});

describe("exponential backoff reconnect", () => {
  it("reconnects with a doubling, capped delay and resets after a good open", () => {
    renderHook(() => useChatSocket("alice", "general"));
    expect(MockWebSocket.instances).toHaveLength(1);

    // First drop: backoff 1000*2 = 2000ms.
    act(() => latest().close());
    act(() => vi.advanceTimersByTime(1999));
    expect(MockWebSocket.instances).toHaveLength(1);
    act(() => vi.advanceTimersByTime(1));
    expect(MockWebSocket.instances).toHaveLength(2);

    // Second drop without a successful open: 2000*2 = 4000ms.
    act(() => latest().close());
    act(() => vi.advanceTimersByTime(4000));
    expect(MockWebSocket.instances).toHaveLength(3);

    // A successful open resets backoff back to the 2000ms first step.
    act(() => latest().open());
    act(() => latest().close());
    act(() => vi.advanceTimersByTime(2000));
    expect(MockWebSocket.instances).toHaveLength(4);
  });

  it("does not reconnect after intentional unmount", () => {
    const { unmount } = renderHook(() => useChatSocket("alice", "general"));
    act(() => latest().open());
    unmount();

    act(() => vi.advanceTimersByTime(60_000));
    expect(MockWebSocket.instances).toHaveLength(1);
    expect(useChatStore.getState().status).not.toBe("reconnecting");
  });
});
