"use client";

import { useEffect, useRef } from "react";
import { useChatStore } from "@/lib/store";
import type { ChatMessage } from "@/lib/protocol";

// Stable empty reference so the selector never returns a fresh array (which would
// make useSyncExternalStore see a new snapshot every render → infinite loop).
const EMPTY: ChatMessage[] = [];

// MessageList renders strictly by seq (not arrival order) and dedupes by seq —
// the visible proof that ordering holds regardless of how messages raced across
// nodes. Optimistic (pending) messages render greyed until their ack arrives.
export function MessageList({ conversation, me }: { conversation: string; me: string }) {
  const messages = useChatStore((s) => s.messages[conversation] ?? EMPTY);
  const bottom = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    bottom.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages.length]);

  return (
    <div className="flex-1 overflow-y-auto p-4 space-y-2">
      {messages.map((m) => (
        <div
          key={m.clientMsgId || m.seq}
          className={`flex flex-col ${m.senderId === me ? "items-end" : "items-start"}`}
        >
          <div
            className={`max-w-[70%] rounded-2xl px-3 py-2 text-sm ${
              m.senderId === me ? "bg-blue-600" : "bg-neutral-700"
            } ${m.pending ? "opacity-50" : ""}`}
          >
            <span className="block text-[10px] text-neutral-300">
              {m.senderId}
              {m.seq > 0 ? ` · #${m.seq}` : " · sending…"}
            </span>
            {m.body}
          </div>
        </div>
      ))}
      <div ref={bottom} />
    </div>
  );
}
