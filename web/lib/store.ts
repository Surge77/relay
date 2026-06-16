import { create } from "zustand";
import type { ChatMessage } from "./protocol";

export type Status = "connecting" | "connected" | "reconnecting" | "down";

interface ChatState {
  status: Status;
  // Messages per conversation, kept sorted by seq and deduped by seq.
  messages: Record<string, ChatMessage[]>;
  // Highest contiguous acked seq per conversation — the catch-up cursor.
  cursors: Record<string, number>;
  // userId → online flag.
  presence: Record<string, boolean>;
  typing: Record<string, boolean>; // userId → typing in current conversation

  setStatus: (s: Status) => void;
  addOptimistic: (conv: string, m: ChatMessage) => void;
  applyMessage: (conv: string, m: ChatMessage) => void;
  confirmAck: (conv: string, clientMsgId: string, seq: number) => void;
  setCursor: (conv: string, seq: number) => void;
  setPresence: (userId: string, online: boolean) => void;
  setTyping: (userId: string, typing: boolean) => void;
}

// upsert inserts or replaces a message by seq, keeping the slice sorted. A seq
// of 0 (optimistic, not yet acked) is keyed by clientMsgId instead.
function upsert(list: ChatMessage[], m: ChatMessage): ChatMessage[] {
  const next = list.filter((x) =>
    m.seq > 0 ? x.seq !== m.seq && x.clientMsgId !== m.clientMsgId : x.clientMsgId !== m.clientMsgId,
  );
  next.push(m);
  next.sort((a, b) => {
    if (a.seq === 0 || b.seq === 0) return a.ts - b.ts;
    return a.seq - b.seq;
  });
  return next;
}

export const useChatStore = create<ChatState>((set) => ({
  status: "connecting",
  messages: {},
  cursors: {},
  presence: {},
  typing: {},

  setStatus: (s) => set({ status: s }),

  addOptimistic: (conv, m) =>
    set((st) => ({ messages: { ...st.messages, [conv]: upsert(st.messages[conv] ?? [], m) } })),

  applyMessage: (conv, m) =>
    set((st) => {
      const merged = upsert(st.messages[conv] ?? [], m);
      const cursor = Math.max(st.cursors[conv] ?? 0, m.seq);
      return { messages: { ...st.messages, [conv]: merged }, cursors: { ...st.cursors, [conv]: cursor } };
    }),

  confirmAck: (conv, clientMsgId, seq) =>
    set((st) => {
      const list = st.messages[conv] ?? [];
      const cursor = Math.max(st.cursors[conv] ?? 0, seq);
      const target = list.find((m) => m.clientMsgId === clientMsgId);
      if (!target) {
        // The fanned-out copy may arrive before the ack; applyMessage handles it.
        return { cursors: { ...st.cursors, [conv]: cursor } };
      }
      const rest = list.filter((m) => m.clientMsgId !== clientMsgId && m.seq !== seq);
      return {
        messages: { ...st.messages, [conv]: upsert(rest, { ...target, seq, pending: false }) },
        cursors: { ...st.cursors, [conv]: cursor },
      };
    }),

  setCursor: (conv, seq) =>
    set((st) => ({ cursors: { ...st.cursors, [conv]: Math.max(st.cursors[conv] ?? 0, seq) } })),

  setPresence: (userId, online) =>
    set((st) => ({ presence: { ...st.presence, [userId]: online } })),

  setTyping: (userId, typing) =>
    set((st) => ({ typing: { ...st.typing, [userId]: typing } })),
}));
