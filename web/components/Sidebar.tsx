"use client";

import { useConvStore, convLabel, type ConversationSummary } from "@/lib/conversations";
import { useChatStore } from "@/lib/store";
import { hueFor } from "@/lib/hue";
import { NewChat, Avatar } from "@/components/NewChat";

// Sidebar lists conversations split into Direct Messages and Rooms. Every row is
// tagged with the conversation's own colour so it's identifiable at a glance.
export function Sidebar() {
  const conversations = useConvStore((s) => s.conversations);
  const dms = conversations.filter((c) => c.Kind === "dm");
  const rooms = conversations.filter((c) => c.Kind !== "dm");

  return (
    <nav className="flex h-full flex-col overflow-y-auto p-3">
      <NewChat />

      {conversations.length === 0 && (
        <div className="mt-4 text-center font-mono text-xs text-ash">
          No conversations yet.
        </div>
      )}

      <Section title="Direct" bar="bg-cobalt" items={dms} />
      <Section title="Rooms" bar="bg-tomato" items={rooms} />
    </nav>
  );
}

function Section({ title, bar, items }: { title: string; bar: string; items: ConversationSummary[] }) {
  if (items.length === 0) return null;
  return (
    <div className="mt-4">
      <div className="mb-2 flex items-center gap-2">
        <span className={`h-3 w-3 ink-border ${bar}`} />
        <span className="font-mono text-[11px] font-bold uppercase tracking-[0.16em]">{title}</span>
      </div>
      <div className="space-y-2">
        {items.map((c) => (
          <Row key={c.ID} c={c} />
        ))}
      </div>
    </div>
  );
}

function Row({ c }: { c: ConversationSummary }) {
  const activeId = useConvStore((s) => s.activeId);
  const setActive = useConvStore((s) => s.setActive);
  const presence = useChatStore((s) => s.presence);

  const active = c.ID === activeId;
  const isDM = c.Kind === "dm";
  const online = isDM ? (presence[c.PeerID] ?? c.PeerOnline) : false;
  const hue = hueFor(c.ID);

  return (
    <button
      onClick={() => setActive(c.ID)}
      className={`flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left transition-transform ${
        active ? "ink-border bg-bone" : "hover:-translate-x-0.5 hover:-translate-y-0.5 hover:bg-bone/60"
      }`}
      style={active ? { boxShadow: "var(--sh-sm)" } : undefined}
    >
      {isDM ? (
        <Avatar name={c.PeerName || "?"} online={online} id={c.PeerID || c.ID} />
      ) : (
        <span
          className="grid h-9 w-9 shrink-0 place-items-center rounded-lg text-sm font-extrabold ink-border"
          style={{ background: hue.fill, color: hue.on }}
        >
          {c.Kind === "channel" ? "#" : (c.Name.trim()[0] ?? "?").toUpperCase()}
        </span>
      )}

      <span className="min-w-0 flex-1">
        <span className="flex items-center justify-between gap-2">
          <span className={`truncate text-[14px] text-ink ${c.UnreadCount > 0 ? "font-extrabold" : "font-bold"}`}>
            {convLabel(c)}
          </span>
          {c.LastMessage && (
            <span className={`shrink-0 font-mono text-[10px] ${c.UnreadCount > 0 ? "font-bold text-ink" : "text-ash"}`}>
              {relTime(c.LastMessage.TS)}
            </span>
          )}
        </span>
        {c.LastMessage && (
          <span className={`mt-0.5 block truncate text-[12px] ${c.UnreadCount > 0 ? "font-semibold text-ink" : "text-ash"}`}>
            {c.LastMessage.Body}
          </span>
        )}
      </span>

      {c.UnreadCount > 0 && (
        <span className="ink-border grid h-6 min-w-[24px] shrink-0 place-items-center rounded-md bg-tomato px-1 font-mono text-[11px] font-bold text-paper">
          {c.UnreadCount > 99 ? "99" : c.UnreadCount}
        </span>
      )}
    </button>
  );
}

// relTime renders a compact relative timestamp for the conversation list.
function relTime(tsMs: number): string {
  const secs = Math.max(0, (Date.now() - tsMs) / 1000);
  if (secs < 60) return "now";
  if (secs < 3600) return `${Math.floor(secs / 60)}m`;
  if (secs < 86400) return `${Math.floor(secs / 3600)}h`;
  if (secs < 604800) return `${Math.floor(secs / 86400)}d`;
  return `${Math.floor(secs / 604800)}w`;
}
