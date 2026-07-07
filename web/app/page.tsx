"use client";

import { useEffect, useMemo } from "react";
import { useChatSocket } from "@/lib/useChatSocket";
import { useChatStore } from "@/lib/store";
import { useAuthStore, logout } from "@/lib/auth";
import { useConvStore, convLabel } from "@/lib/conversations";
import { hueFor } from "@/lib/hue";
import { ConnectionBadge } from "@/components/ConnectionBadge";
import { MessageList } from "@/components/MessageList";
import { Composer } from "@/components/Composer";
import { Sidebar } from "@/components/Sidebar";
import { Search } from "@/components/Search";
import { MemberRoster } from "@/components/MemberRoster";
import { AuthGate } from "@/components/AuthGate";
import { Avatar } from "@/components/NewChat";

export default function Home() {
  return (
    <AuthGate>
      <Chat />
    </AuthGate>
  );
}

function Chat() {
  const user = useAuthStore((s) => s.user)!;
  const load = useConvStore((s) => s.load);
  const activeId = useConvStore((s) => s.activeId);
  const active = useConvStore((s) => s.conversations.find((c) => c.ID === activeId));

  const { send, sendTyping, sendRead } = useChatSocket(user.id, activeId ?? "");
  const typing = useChatStore((s) => s.typing);
  const presence = useChatStore((s) => s.presence);
  const cursor = useChatStore((s) => (activeId ? (s.cursors[activeId] ?? 0) : 0));

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (activeId && cursor > 0) sendRead(cursor);
  }, [activeId, cursor, sendRead]);

  const typingUsers = useMemo(
    () => Object.entries(typing).filter(([u, t]) => t && u !== user.id).map(([u]) => u),
    [typing, user.id],
  );

  const hue = active ? hueFor(active.ID) : null;
  const back = () => useConvStore.setState({ activeId: null });

  return (
    <div className="flex h-screen text-ink">
      {/* Zone 1 — rail */}
      <nav className="hidden w-[72px] shrink-0 flex-col items-center gap-4 border-r-2 border-ink bg-bone py-5 lg:flex">
        <div
          className="grid h-12 w-12 place-items-center rounded-lg bg-cobalt text-paper ink-border"
          style={{ boxShadow: "var(--sh-sm)" }}
        >
          <BoltGlyph />
        </div>
        <div className="mt-1 h-0.5 w-8 bg-ink" />
        <div className="mt-auto flex flex-col items-center gap-4">
          <button
            onClick={logout}
            title="Log out"
            className="btn grid h-10 w-10 place-items-center rounded-lg bg-paper text-ink hover:bg-tomato hover:text-paper"
          >
            <LogoutGlyph />
          </button>
          <div title={`${user.display_name} · ${user.email}`}>
            <Avatar name={user.display_name} online id={user.id} />
          </div>
        </div>
      </nav>

      {/* Zone 2 — conversation panel */}
      <aside
        className={`w-full shrink-0 flex-col border-r-2 border-ink bg-paper lg:flex lg:w-[312px] ${
          activeId ? "hidden lg:flex" : "flex"
        }`}
      >
        <header className="border-b-2 border-ink px-4 pb-3 pt-4">
          <div className="flex items-center justify-between">
            <h1 className="font-display text-3xl font-extrabold uppercase leading-none tracking-tight">
              Relay
            </h1>
            <ConnectionBadge />
          </div>
          <div className="mt-1.5 flex items-center gap-2">
            <span className="h-2 w-10 bg-cobalt" />
            <span className="h-2 w-4 bg-tomato" />
            <span className="h-2 w-6 bg-lime" />
            <span className="font-mono text-[11px] text-ash">@{user.display_name}</span>
          </div>
        </header>
        <div className="border-b-2 border-ink p-3">
          <Search />
        </div>
        <div className="min-h-0 flex-1 overflow-hidden">
          <Sidebar />
        </div>
      </aside>

      {/* Zone 3 — thread */}
      <main className={`min-w-0 flex-1 flex-col ${activeId ? "flex" : "hidden lg:flex"}`}>
        {activeId && active && hue ? (
          <>
            <header className="flex items-center justify-between border-b-2 border-ink bg-paper px-4 py-3 sm:px-6">
              <div className="flex min-w-0 items-center gap-3">
                <button
                  onClick={back}
                  aria-label="Back to conversations"
                  className="btn -ml-1 grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-paper lg:hidden"
                >
                  <svg viewBox="0 0 24 24" fill="none" className="h-5 w-5" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M15 18l-6-6 6-6" /></svg>
                </button>
                <span
                  className="grid h-9 w-9 shrink-0 place-items-center rounded-lg text-sm font-extrabold ink-border"
                  style={{ background: hue.fill, color: hue.on, boxShadow: "var(--sh-sm)" }}
                >
                  {active.Kind === "channel" ? "#" : convLabel(active).replace(/^#\s*/, "").trim()[0]?.toUpperCase() ?? "?"}
                </span>
                <div className="min-w-0">
                  <div className="truncate font-display text-lg font-bold leading-none">
                    {convLabel(active)}
                  </div>
                  {active.Kind === "dm" && (
                    <div className="mt-1 flex items-center gap-1.5 font-mono text-[11px] uppercase tracking-wide text-ash">
                      <span
                        className={`inline-block h-2 w-2 rounded-full ink-border ${
                          (presence[active.PeerID] ?? active.PeerOnline) ? "bg-mint" : "bg-bone"
                        }`}
                      />
                      {(presence[active.PeerID] ?? active.PeerOnline)
                        ? "Online now"
                        : lastSeenLabel(active.PeerLastSeen)}
                    </div>
                  )}
                </div>
              </div>
              <MemberRoster conversationId={activeId} me={user.id} />
            </header>

            <MessageList conversation={activeId} me={user.id} />

            <div className="h-6 px-6">
              {typingUsers.length > 0 && (
                <div className="flex items-center gap-2 font-mono text-[11px] font-bold uppercase text-ink">
                  <span className="flex gap-1">
                    <span className="bob inline-block h-2 w-2 rounded-full bg-ink" />
                    <span className="bob inline-block h-2 w-2 rounded-full bg-ink" />
                    <span className="bob inline-block h-2 w-2 rounded-full bg-ink" />
                  </span>
                  {typingUsers.join(", ")} typing
                </div>
              )}
            </div>

            <Composer
              onSend={send}
              onTyping={sendTyping}
              placeholder={`Message ${convLabel(active)}`}
              hue={hue}
              focusKey={activeId}
            />
          </>
        ) : (
          <EmptyThread />
        )}
      </main>
    </div>
  );
}

function EmptyThread() {
  return (
    <div className="grid flex-1 place-items-center p-8">
      <div className="pop-in card max-w-sm rounded-xl p-8 text-center" style={{ boxShadow: "var(--sh-lg)" }}>
        <div
          className="mx-auto mb-5 grid h-16 w-16 -rotate-3 place-items-center rounded-xl bg-butter text-ink ink-border"
          style={{ boxShadow: "var(--sh)" }}
        >
          <BoltGlyph className="h-8 w-8" />
        </div>
        <h2 className="font-display text-2xl font-extrabold uppercase leading-tight">
          Nothing open
        </h2>
        <p className="mx-auto mt-2 max-w-xs text-sm text-ash">
          Pick a conversation on the left, or start a fresh one. Messages land in order, in real time.
        </p>
      </div>
    </div>
  );
}

function BoltGlyph({ className = "h-6 w-6" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" className={className} aria-hidden>
      <path d="M13.5 3 5 13.5h5l-1.5 7.5L18 10.5h-5L13.5 3Z" fill="currentColor" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
    </svg>
  );
}

function LogoutGlyph() {
  return (
    <svg viewBox="0 0 24 24" fill="none" className="h-5 w-5" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M15 4h3a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-3M10 17l5-5-5-5M15 12H3" />
    </svg>
  );
}

// lastSeenLabel renders a coarse "last seen …" string for an offline DM peer.
function lastSeenLabel(iso: string | null): string {
  if (!iso) return "offline";
  const secs = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (secs < 60) return "seen just now";
  if (secs < 3600) return `seen ${Math.floor(secs / 60)}m ago`;
  if (secs < 86400) return `seen ${Math.floor(secs / 3600)}h ago`;
  return `seen ${Math.floor(secs / 86400)}d ago`;
}
