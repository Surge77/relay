"use client";

import { useState } from "react";
import { useConvStore, convLabel } from "@/lib/conversations";

// Sidebar lists the signed-in user's conversations with an unread badge and the
// latest message preview, newest activity first (the server already sorts them),
// plus a form to create a new channel or group.
export function Sidebar() {
  const conversations = useConvStore((s) => s.conversations);
  const activeId = useConvStore((s) => s.activeId);
  const setActive = useConvStore((s) => s.setActive);

  return (
    <nav className="flex h-full flex-col gap-1 overflow-y-auto">
      <div className="mb-1 flex items-center justify-between">
        <span className="text-xs text-neutral-400">Conversations</span>
      </div>
      <NewConversation />
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

// NewConversation is a collapsed "+ New" button that expands to a name + kind
// form. On submit it creates the conversation and opens it.
function NewConversation() {
  const create = useConvStore((s) => s.create);
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [kind, setKind] = useState<"channel" | "group">("channel");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const submit = async () => {
    const trimmed = name.trim();
    if (!trimmed || busy) return;
    setBusy(true);
    setErr(null);
    try {
      await create(kind, trimmed);
      setName("");
      setOpen(false);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "could not create");
    } finally {
      setBusy(false);
    }
  };

  if (!open) {
    return (
      <button
        onClick={() => setOpen(true)}
        className="mb-1 rounded-lg border border-dashed border-neutral-700 px-3 py-1.5 text-xs text-neutral-400 hover:text-neutral-200"
      >
        + New conversation
      </button>
    );
  }

  return (
    <div className="mb-2 space-y-2 rounded-lg border border-neutral-800 p-2">
      <input
        autoFocus
        value={name}
        onChange={(e) => setName(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") submit();
          if (e.key === "Escape") setOpen(false);
        }}
        placeholder="Name"
        className="w-full rounded-md bg-neutral-800 px-2 py-1.5 text-sm outline-none"
      />
      <div className="flex gap-1">
        {(["channel", "group"] as const).map((k) => (
          <button
            key={k}
            onClick={() => setKind(k)}
            className={`flex-1 rounded-md px-2 py-1 text-xs ${
              kind === k ? "bg-neutral-700" : "bg-neutral-900 text-neutral-400"
            }`}
          >
            {k}
          </button>
        ))}
      </div>
      {err && <div className="text-xs text-red-400">{err}</div>}
      <div className="flex gap-1">
        <button
          onClick={submit}
          disabled={!name.trim() || busy}
          className="flex-1 rounded-md bg-blue-600 px-2 py-1 text-xs font-medium disabled:opacity-40"
        >
          {busy ? "Creating…" : "Create"}
        </button>
        <button
          onClick={() => setOpen(false)}
          className="rounded-md border border-neutral-700 px-2 py-1 text-xs text-neutral-400"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}
