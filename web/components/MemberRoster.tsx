"use client";

import { useEffect, useState } from "react";
import { authedGet } from "@/lib/api";
import { useChatStore } from "@/lib/store";

// Member mirrors the gateway's model.Member (no json tags → PascalCase).
interface Member {
  UserID: string;
  DisplayName: string;
  Role: string;
}

interface ConversationDetail {
  members: Member[] | null;
}

// MemberRoster shows each member of the active conversation with an online dot.
// Presence comes from the chat store, which the gateway seeds with a snapshot on
// subscribe and keeps live via presence frames — so dots reflect real state.
export function MemberRoster({ conversationId, me }: { conversationId: string; me: string }) {
  const [members, setMembers] = useState<Member[]>([]);
  const presence = useChatStore((s) => s.presence);

  useEffect(() => {
    let cancelled = false;
    authedGet<ConversationDetail>(`/conversations/${conversationId}`)
      .then((d) => {
        if (!cancelled) setMembers(d.members ?? []);
      })
      .catch(() => {
        if (!cancelled) setMembers([]);
      });
    return () => {
      cancelled = true;
    };
  }, [conversationId]);

  if (members.length === 0) return null;

  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-neutral-800 px-4 py-1.5 text-xs text-neutral-400">
      {members.map((m) => {
        const online = presence[m.UserID] ?? false;
        return (
          <span key={m.UserID} className="flex items-center gap-1.5">
            <span
              className={`h-2 w-2 rounded-full ${online ? "bg-green-500" : "bg-neutral-600"}`}
              title={online ? "online" : "offline"}
            />
            {m.DisplayName || m.UserID.slice(0, 8)}
            {m.UserID === me ? " (you)" : ""}
          </span>
        );
      })}
    </div>
  );
}
