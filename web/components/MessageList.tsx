"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useChatStore } from "@/lib/store";
import { hueFor } from "@/lib/hue";
import type { ChatMessage } from "@/lib/protocol";

// Stable empty references so the selectors never return a fresh value (which
// would make useSyncExternalStore see a new snapshot every render → infinite
// loop).
const EMPTY: ChatMessage[] = [];
const EMPTY_RECEIPTS: Record<string, number> = {};

const GROUP_GAP_MS = 5 * 60 * 1000;
const NEAR_BOTTOM_PX = 120;

// MessageList renders strictly by seq (not arrival order) and dedupes by seq —
// the visible proof that ordering holds regardless of how messages raced across
// nodes. Own messages are filled with the conversation's colour; others are ink
// -outlined paper cards. Consecutive messages from one sender are grouped.
export function MessageList({ conversation, me }: { conversation: string; me: string }) {
  const messages = useChatStore((s) => s.messages[conversation] ?? EMPTY);
  const receipts = useChatStore((s) => s.receipts[conversation] ?? EMPTY_RECEIPTS);
  const hue = hueFor(conversation);

  const scroller = useRef<HTMLDivElement | null>(null);
  const nearBottom = useRef(true);
  const prevConv = useRef<string | null>(null);
  const prevLen = useRef(0);
  const [showJump, setShowJump] = useState(false);

  const scrollToBottom = useCallback((smooth: boolean) => {
    const el = scroller.current;
    if (!el) return;
    el.scrollTo({ top: el.scrollHeight, behavior: smooth ? "smooth" : "auto" });
  }, []);

  // Conversation switch → jump straight to the latest, no smooth yank.
  useEffect(() => {
    prevConv.current = conversation;
    prevLen.current = messages.length;
    nearBottom.current = true;
    setShowJump(false);
    requestAnimationFrame(() => scrollToBottom(false));
  }, [conversation, scrollToBottom]);

  // New messages → follow only if the reader is already near the bottom;
  // otherwise surface a "jump to latest" affordance instead of yanking them down.
  useEffect(() => {
    if (conversation !== prevConv.current) return;
    if (messages.length > prevLen.current) {
      if (nearBottom.current) {
        scrollToBottom(true);
        setShowJump(false);
      } else {
        setShowJump(true);
      }
    }
    prevLen.current = messages.length;
  }, [messages.length, conversation, scrollToBottom]);

  const onScroll = () => {
    const el = scroller.current;
    if (!el) return;
    const nb = el.scrollHeight - el.scrollTop - el.clientHeight < NEAR_BOTTOM_PX;
    nearBottom.current = nb;
    if (nb) setShowJump(false);
  };

  const maxSeenByOthers = Object.entries(receipts)
    .filter(([userId]) => userId !== me)
    .reduce((acc, [, seq]) => Math.max(acc, seq), 0);
  const lastSeenOwnSeq = messages
    .filter((m) => m.senderId === me && m.seq > 0 && m.seq <= maxSeenByOthers)
    .reduce((acc, m) => Math.max(acc, m.seq), 0);

  return (
    <div className="relative min-h-0 flex-1">
      <div ref={scroller} onScroll={onScroll} className="h-full overflow-y-auto px-4 py-5 sm:px-8">
        {messages.map((m, i) => {
          const prev = messages[i - 1];
          const next = messages[i + 1];
          const mine = m.senderId === me;
          const newDay = !prev || !sameDay(prev.ts, m.ts);
          const grouped = !newDay && !!prev && prev.senderId === m.senderId && m.ts - prev.ts < GROUP_GAP_MS;
          const lastOfGroup =
            !next || next.senderId !== m.senderId || !sameDay(next.ts, m.ts) || next.ts - m.ts >= GROUP_GAP_MS;
          const isSeen = mine && m.seq === lastSeenOwnSeq && lastSeenOwnSeq > 0;

          return (
            <div key={m.clientMsgId || m.seq}>
              {newDay && <DateDivider ts={m.ts} />}
              <div className={`pop-in group flex items-end gap-2.5 ${grouped ? "mt-1" : "mt-4"} ${mine ? "flex-row-reverse" : "flex-row"}`}>
                {!mine &&
                  (grouped ? <span className="w-9 shrink-0" /> : <SenderTile id={m.senderId} />)}

                <div className={`flex max-w-[78%] flex-col sm:max-w-[66%] ${mine ? "items-end" : "items-start"}`}>
                  {!grouped && !mine && (
                    <span className="mb-1 ml-0.5 font-mono text-[11px] font-bold uppercase tracking-wide text-ash">
                      {short(m.senderId)}
                    </span>
                  )}
                  <div
                    className={`ink-border rounded-xl px-3.5 py-2 text-[14px] font-medium leading-relaxed [overflow-wrap:anywhere] whitespace-pre-wrap ${
                      grouped ? (mine ? "rounded-tr-md" : "rounded-tl-md") : ""
                    } ${m.pending ? "opacity-55" : ""}`}
                    style={
                      mine
                        ? { background: hue.fill, color: hue.on, boxShadow: "var(--sh-sm)" }
                        : { background: "var(--color-paper)", color: "var(--color-ink)", boxShadow: "var(--sh-sm)" }
                    }
                  >
                    {m.body}
                  </div>
                  {/* Time shows once per group; on hover for the rest. Seen is always shown. */}
                  <span
                    className={`mt-1 px-0.5 font-mono text-[10px] text-ash transition-opacity ${
                      lastOfGroup || isSeen ? "opacity-100" : "opacity-0 group-hover:opacity-100"
                    }`}
                  >
                    {m.seq > 0 ? clock(m.ts) : "sending…"}
                    {isSeen && <span className="ml-1.5 font-bold text-ink">· SEEN ✓</span>}
                  </span>
                </div>
              </div>
            </div>
          );
        })}
      </div>

      {showJump && (
        <button
          onClick={() => scrollToBottom(true)}
          className="btn stamp-in absolute bottom-3 left-1/2 -translate-x-1/2 rounded-full bg-cobalt px-4 py-1.5 font-mono text-[11px] font-bold uppercase tracking-wide text-paper"
        >
          ↓ New messages
        </button>
      )}
    </div>
  );
}

function DateDivider({ ts }: { ts: number }) {
  return (
    <div className="my-5 flex justify-center">
      <span className="ink-border -rotate-1 rounded-md bg-butter px-3 py-0.5 font-mono text-[10px] font-bold uppercase tracking-widest text-ink" style={{ boxShadow: "var(--sh-sm)" }}>
        {dayLabel(ts)}
      </span>
    </div>
  );
}

function SenderTile({ id }: { id: string }) {
  const hue = hueFor(id);
  return (
    <span className="grid h-9 w-9 shrink-0 place-items-center rounded-lg text-sm font-extrabold ink-border" style={{ background: hue.fill, color: hue.on }}>
      {(id.trim()[0] ?? "?").toUpperCase()}
    </span>
  );
}

const short = (id: string) => (id.length > 12 ? `${id.slice(0, 12)}…` : id);

function sameDay(a: number, b: number): boolean {
  return new Date(a).toDateString() === new Date(b).toDateString();
}

function dayLabel(ts: number): string {
  const d = new Date(ts);
  const today = new Date();
  const yest = new Date();
  yest.setDate(today.getDate() - 1);
  if (d.toDateString() === today.toDateString()) return "Today";
  if (d.toDateString() === yest.toDateString()) return "Yesterday";
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

function clock(ts: number): string {
  return new Date(ts).toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" });
}
