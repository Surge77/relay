"use client";

import { useEffect, useMemo } from "react";
import { useChatSocket } from "@/lib/useChatSocket";
import { useChatStore } from "@/lib/store";
import { useAuthStore, logout } from "@/lib/auth";
import { ConnectionBadge } from "@/components/ConnectionBadge";
import { MessageList } from "@/components/MessageList";
import { Composer } from "@/components/Composer";
import { AuthGate } from "@/components/AuthGate";

// Phase 1 still uses the single seeded "general" channel; multi-conversation
// sidebar + real member/presence lists arrive in the conversations phase.
const CONVERSATION = "general";

export default function Home() {
  return (
    <AuthGate>
      <Chat />
    </AuthGate>
  );
}

function Chat() {
  const user = useAuthStore((s) => s.user)!;
  const { send, sendTyping, sendRead } = useChatSocket(user.id, CONVERSATION);
  const typing = useChatStore((s) => s.typing);
  const cursor = useChatStore((s) => s.cursors[CONVERSATION] ?? 0);

  useEffect(() => {
    if (cursor > 0) sendRead(cursor);
  }, [cursor, sendRead]);

  const typingUsers = useMemo(
    () => Object.entries(typing).filter(([u, t]) => t && u !== user.id).map(([u]) => u),
    [typing, user.id],
  );

  return (
    <div className="flex h-screen">
      <aside className="w-56 shrink-0 border-r border-neutral-800 p-4 flex flex-col gap-4">
        <div>
          <div className="text-xs text-neutral-400">Signed in as</div>
          <div className="text-sm font-medium">{user.display_name}</div>
          <div className="text-xs text-neutral-500 truncate">{user.email}</div>
        </div>

        <div>
          <div className="text-xs text-neutral-400 mb-2">Channels</div>
          <div className="rounded-lg bg-neutral-800 px-3 py-2 text-sm"># {CONVERSATION}</div>
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
        <header className="border-b border-neutral-800 px-4 py-3 text-sm font-medium"># {CONVERSATION}</header>
        <MessageList conversation={CONVERSATION} me={user.id} />
        <div className="h-5 px-4 text-xs text-neutral-500">
          {typingUsers.length > 0 ? `${typingUsers.join(", ")} typing…` : ""}
        </div>
        <Composer onSend={send} onTyping={sendTyping} />
      </main>
    </div>
  );
}
