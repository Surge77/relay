"use client";

import { useConvStore, convLabel } from "@/lib/conversations";

// Sidebar lists the signed-in user's conversations with an unread badge and the
// latest message preview, newest activity first (the server already sorts them).
export function Sidebar() {
  const conversations = useConvStore((s) => s.conversations);
  const activeId = useConvStore((s) => s.activeId);
  const setActive = useConvStore((s) => s.setActive);

  return (
    <nav className="flex flex-col gap-1 overflow-y-auto">
      <div className="mb-1 text-xs text-neutral-400">Conversations</div>
      {conversations.length === 0 && (
        <div className="px-3 py-2 text-xs text-neutral-600">No conversations yet</div>
      )}
      {conversations.map((c) => {
        const active = c.ID === activeId;
        return (
          <button
            key={c.ID}
            onClick={() => setActive(c.ID)}
            className={`flex items-center justify-between rounded-lg px-3 py-2 text-left text-sm ${
              active ? "bg-neutral-800" : "hover:bg-neutral-900"
            }`}
          >
            <span className="min-w-0 flex-1">
              <span className="block truncate">{convLabel(c)}</span>
              {c.LastMessage && (
                <span className="block truncate text-xs text-neutral-500">{c.LastMessage.Body}</span>
              )}
            </span>
            {c.UnreadCount > 0 && (
              <span className="ml-2 shrink-0 rounded-full bg-blue-600 px-2 py-0.5 text-xs font-medium">
                {c.UnreadCount}
              </span>
            )}
          </button>
        );
      })}
    </nav>
  );
}
