"use client";

import { useEffect, useMemo } from "react";
import { useChatSocket } from "@/lib/useChatSocket";
import { useChatStore } from "@/lib/store";
import { useAuthStore, logout } from "@/lib/auth";
import { useConvStore, convLabel } from "@/lib/conversations";
import { ConnectionBadge } from "@/components/ConnectionBadge";
import { MessageList } from "@/components/MessageList";
import { Composer } from "@/components/Composer";
import { Sidebar } from "@/components/Sidebar";
import { Search } from "@/components/Search";
import { MemberRoster } from "@/components/MemberRoster";
import { AuthGate } from "@/components/AuthGate";

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

  return (
    <div className="flex h-screen">
      <aside className="flex w-64 shrink-0 flex-col gap-4 border-r border-neutral-800 p-4">
        <div>
          <div className="text-xs text-neutral-400">Signed in as</div>
          <div className="text-sm font-medium">{user.display_name}</div>
          <div className="truncate text-xs text-neutral-500">{user.email}</div>
        </div>

        <Search />

        <div className="min-h-0 flex-1 overflow-hidden">
          <Sidebar />
        </div>

        <div className="mt-auto space-y-3">
          <ConnectionBadge />
          <button
            onClick={logout}
            className="w-full rounded-lg border border-neutral-800 py-1.5 text-xs text-neutral-400 hover:text-neutral-200"
          >
            Log out
          </button>
        </div>
      </aside>

      <main className="flex flex-1 flex-col">
        {activeId && active ? (
          <>
            <header className="border-b border-neutral-800 px-4 py-3">
              <div className="text-sm font-medium">{convLabel(active)}</div>
              {active.Kind === "dm" && (
                <div className="text-xs text-neutral-500">
                  {(presence[active.PeerID] ?? active.PeerOnline)
                    ? "online"
                    : lastSeenLabel(active.PeerLastSeen)}
                </div>
              )}
            </header>
            <MemberRoster conversationId={activeId} me={user.id} />
            <MessageList conversation={activeId} me={user.id} />
            <div className="h-5 px-4 text-xs text-neutral-500">
              {typingUsers.length > 0 ? `${typingUsers.join(", ")} typing…` : ""}
            </div>
            <Composer onSend={send} onTyping={sendTyping} placeholder={`Message ${convLabel(active)}`} />
          </>
        ) : (
          <div className="flex flex-1 items-center justify-center text-sm text-neutral-600">
            Select a conversation
          </div>
        )}
      </main>
    </div>
  );
}

// lastSeenLabel renders a coarse "last seen …" string for an offline DM peer.
function lastSeenLabel(iso: string | null): string {
  if (!iso) return "offline";
  const secs = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (secs < 60) return "last seen just now";
  if (secs < 3600) return `last seen ${Math.floor(secs / 60)}m ago`;
  if (secs < 86400) return `last seen ${Math.floor(secs / 3600)}h ago`;
  return `last seen ${Math.floor(secs / 86400)}d ago`;
}
