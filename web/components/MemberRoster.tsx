"use client";

import { useEffect, useState } from "react";
import { authedGet } from "@/lib/api";
import { useChatStore } from "@/lib/store";
import { hueFor } from "@/lib/hue";

// Member mirrors the gateway's model.Member (no json tags → PascalCase).
interface Member {
  UserID: string;
  DisplayName: string;
  Role: string;
}

interface ConversationDetail {
  members: Member[] | null;
}

// MemberRoster shows the active conversation's members as a row of colour-coded,
// ink-outlined tiles with online flags; presence comes from the chat store.
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

  const shown = members.slice(0, 4);
  const extra = members.length - shown.length;
  const onlineCount = members.filter((m) => presence[m.UserID] ?? false).length;

  return (
    <div className="flex items-center gap-2.5">
      <div className="flex -space-x-1.5">
        {shown.map((m) => {
          const online = presence[m.UserID] ?? false;
          const label = m.DisplayName || m.UserID.slice(0, 8);
          const hue = hueFor(m.UserID);
          return (
            <span
              key={m.UserID}
              title={`${label}${m.UserID === me ? " (you)" : ""}${online ? " · online" : ""}`}
              className="relative grid h-8 w-8 place-items-center rounded-lg text-[12px] font-extrabold ink-border"
              style={{ background: hue.fill, color: hue.on }}
            >
              {(label.trim()[0] ?? "?").toUpperCase()}
              {online && <span className="absolute -bottom-1 -right-1 h-2.5 w-2.5 rounded-full bg-mint ink-border" />}
            </span>
          );
        })}
        {extra > 0 && (
          <span className="grid h-8 w-8 place-items-center rounded-lg bg-bone text-[11px] font-extrabold ink-border">
            +{extra}
          </span>
        )}
      </div>
      <span className="hidden font-mono text-[10px] font-bold uppercase text-ash sm:inline">
        {members.length} in · {onlineCount} on
      </span>
    </div>
  );
}
