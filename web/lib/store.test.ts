import { describe, it, expect, beforeEach } from "vitest";

import { useChatStore } from "./store";
import type { ChatMessage } from "./protocol";

const CONV = "general";

function msg(over: Partial<ChatMessage>): ChatMessage {
  return { seq: 0, senderId: "alice", clientMsgId: "", body: "hi", ts: 0, pending: false, ...over };
}

// Reset the singleton store between tests so each runs in isolation.
const initial = useChatStore.getState();
beforeEach(() => {
  useChatStore.setState(
    { messages: {}, cursors: {}, presence: {}, typing: {}, receipts: {}, status: "connecting" },
    true,
  );
  // restore actions stripped by the replace above
  useChatStore.setState(initial);
  useChatStore.setState({ messages: {}, cursors: {}, presence: {}, typing: {}, receipts: {} });
});

describe("applyMessage ordering and dedupe", () => {
  it("keeps messages sorted by seq regardless of arrival order", () => {
    const { applyMessage } = useChatStore.getState();
    applyMessage(CONV, msg({ seq: 3, clientMsgId: "c" }));
    applyMessage(CONV, msg({ seq: 1, clientMsgId: "a" }));
    applyMessage(CONV, msg({ seq: 2, clientMsgId: "b" }));

    expect(useChatStore.getState().messages[CONV].map((m) => m.seq)).toEqual([1, 2, 3]);
  });

  it("dedupes a message redelivered with the same seq", () => {
    const { applyMessage } = useChatStore.getState();
    applyMessage(CONV, msg({ seq: 5, clientMsgId: "x" }));
    applyMessage(CONV, msg({ seq: 5, clientMsgId: "x" }));

    expect(useChatStore.getState().messages[CONV]).toHaveLength(1);
  });

  it("advances the cursor to the highest seq seen", () => {
    const { applyMessage } = useChatStore.getState();
    applyMessage(CONV, msg({ seq: 7, clientMsgId: "a" }));
    applyMessage(CONV, msg({ seq: 4, clientMsgId: "b" }));

    expect(useChatStore.getState().cursors[CONV]).toBe(7);
  });
});

describe("optimistic send then ack", () => {
  it("replaces the pending bubble in place and clears pending", () => {
    const { addOptimistic, confirmAck } = useChatStore.getState();
    addOptimistic(CONV, msg({ seq: 0, clientMsgId: "tmp1", pending: true, ts: 100 }));
    confirmAck(CONV, "tmp1", 9);

    const list = useChatStore.getState().messages[CONV];
    expect(list).toHaveLength(1);
    expect(list[0]).toMatchObject({ seq: 9, clientMsgId: "tmp1", pending: false });
    expect(useChatStore.getState().cursors[CONV]).toBe(9);
  });

  it("does not duplicate when the fanned-out copy already arrived before the ack", () => {
    const { addOptimistic, applyMessage, confirmAck } = useChatStore.getState();
    addOptimistic(CONV, msg({ seq: 0, clientMsgId: "tmp2", pending: true, ts: 50 }));
    // server fan-out lands first, carrying the real seq
    applyMessage(CONV, msg({ seq: 11, clientMsgId: "tmp2" }));
    // ack arrives afterwards
    confirmAck(CONV, "tmp2", 11);

    const list = useChatStore.getState().messages[CONV];
    expect(list).toHaveLength(1);
    expect(list[0].seq).toBe(11);
    expect(useChatStore.getState().cursors[CONV]).toBe(11);
  });

  it("advances the cursor on ack even when no local bubble matches", () => {
    const { confirmAck } = useChatStore.getState();
    confirmAck(CONV, "unknown", 20);

    expect(useChatStore.getState().cursors[CONV]).toBe(20);
    expect(useChatStore.getState().messages[CONV] ?? []).toHaveLength(0);
  });
});

describe("monotonic cursors and receipts", () => {
  it("never moves the cursor backwards", () => {
    const { setCursor } = useChatStore.getState();
    setCursor(CONV, 10);
    setCursor(CONV, 4);

    expect(useChatStore.getState().cursors[CONV]).toBe(10);
  });

  it("keeps the highest read seq per user", () => {
    const { setReceipt } = useChatStore.getState();
    setReceipt(CONV, "bob", 8);
    setReceipt(CONV, "bob", 3);

    expect(useChatStore.getState().receipts[CONV].bob).toBe(8);
  });
});
