"use client";

import { useMemo, useState } from "react";
import { useChatSocket } from "@/lib/useChatSocket";
import { useChatStore } from "@/lib/store";
import { ConnectionBadge } from "@/components/ConnectionBadge";
import { MessageList } from "@/components/MessageList";
import { Composer } from "@/components/Composer";

const TEST_USERS = ["alice", "bob", "carol"];
const CONVERSATION = "general";

export default function Home() {
  const [me, setMe] = useState("alice");
  // Re-key the socket on user switch so the hook tears down and reconnects.
  return <Chat key={me} me={me} setMe={setMe} />;
}

function Chat({ me, setMe }: { me: string; setMe: (u: string) => void }) {
  const { send, sendTyping } = useChatSocket(me, CONVERSATION);
  const presence = useChatStore((s) => s.presence);
  const typing = useChatStore((s) => s.typing);

  const typingUsers = useMemo(
    () => Object.entries(typing).filter(([u, t]) => t && u !== me).map(([u]) => u),
    [typing, me],
  );

  return (
    <div className="flex h-screen">
      <aside className="w-56 shrink-0 border-r border-neutral-800 p-4 flex flex-col gap-4">
        <div>
          <label className="block text-xs text-neutral-400 mb-1">Log in as</label>
          <select
            className="w-full rounded-lg bg-neutral-800 px-2 py-1.5 text-sm"
            value={me}
            onChange={(e) => setMe(e.target.value)}
          >
            {TEST_USERS.map((u) => (
              <option key={u} value={u}>
                {u}
              </option>
            ))}
          </select>
        </div>

        <div>
          <div className="text-xs text-neutral-400 mb-2">Channels</div>
          <div className="rounded-lg bg-neutral-800 px-3 py-2 text-sm"># {CONVERSATION}</div>
        </div>

        <div>
          <div className="text-xs text-neutral-400 mb-2">Members</div>
          <ul className="space-y-1 text-sm">
            {TEST_USERS.map((u) => (
              <li key={u} className="flex items-center gap-2">
                <span
                  className={`inline-block h-2 w-2 rounded-full ${
                    presence[u] ? "bg-green-500" : "bg-neutral-600"
                  }`}
                />
                {u}
              </li>
            ))}
          </ul>
        </div>

        <div className="mt-auto">
          <ConnectionBadge />
        </div>
      </aside>

      <main className="flex flex-1 flex-col">
        <header className="border-b border-neutral-800 px-4 py-3 text-sm font-medium"># {CONVERSATION}</header>
        <MessageList conversation={CONVERSATION} me={me} />
        <div className="h-5 px-4 text-xs text-neutral-500">
          {typingUsers.length > 0 ? `${typingUsers.join(", ")} typing…` : ""}
        </div>
        <Composer onSend={send} onTyping={sendTyping} />
      </main>
    </div>
  );
}
