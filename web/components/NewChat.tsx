"use client";

import { useState } from "react";
import { useConvStore } from "@/lib/conversations";
import { useUserSearch, type SearchUser } from "@/lib/users";

type Mode = "dm" | "group" | "channel";

// NewChat is the single entry point for starting a conversation: a direct
// message (find a person), a group (name + pick people), or a channel (name).
// People are found by name/email so no raw user id is ever needed.
export function NewChat() {
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState<Mode>("dm");

  if (!open) {
    return (
      <button
        onClick={() => setOpen(true)}
        className="mb-1 rounded-lg border border-dashed border-neutral-700 px-3 py-1.5 text-xs text-neutral-400 hover:text-neutral-200"
      >
        + New chat
      </button>
    );
  }

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/50 p-4 pt-24">
      <div className="w-full max-w-md rounded-xl border border-neutral-800 bg-neutral-950 p-4 shadow-xl">
        <div className="mb-3 flex items-center justify-between">
          <div className="flex gap-1">
            {(["dm", "group", "channel"] as const).map((m) => (
              <button
                key={m}
                onClick={() => setMode(m)}
                className={`rounded-md px-2.5 py-1 text-xs capitalize ${
                  mode === m ? "bg-neutral-700" : "bg-neutral-900 text-neutral-400"
                }`}
              >
                {m === "dm" ? "Direct" : m}
              </button>
            ))}
          </div>
          <button onClick={() => setOpen(false)} className="text-neutral-500 hover:text-neutral-200">
            ✕
          </button>
        </div>

        {mode === "dm" && <NewDM onDone={() => setOpen(false)} />}
        {mode === "group" && <NewGroup onDone={() => setOpen(false)} />}
        {mode === "channel" && <NewChannel onDone={() => setOpen(false)} />}
      </div>
    </div>
  );
}

function NewDM({ onDone }: { onDone: () => void }) {
  const createDM = useConvStore((s) => s.createDM);
  const { query, setQuery, results, busy } = useUserSearch();
  const [err, setErr] = useState<string | null>(null);

  const start = async (u: SearchUser) => {
    setErr(null);
    try {
      await createDM(u.id);
      onDone();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "could not start chat");
    }
  };

  return (
    <div className="space-y-2">
      <PeopleInput value={query} onChange={setQuery} busy={busy} placeholder="Search people by name or email" />
      {err && <div className="text-xs text-red-400">{err}</div>}
      <PeopleResults results={results} query={query} busy={busy} onPick={start} />
    </div>
  );
}

function NewGroup({ onDone }: { onDone: () => void }) {
  const create = useConvStore((s) => s.create);
  const { query, setQuery, results, busy } = useUserSearch();
  const [name, setName] = useState("");
  const [picked, setPicked] = useState<SearchUser[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const toggle = (u: SearchUser) =>
    setPicked((p) => (p.some((x) => x.id === u.id) ? p.filter((x) => x.id !== u.id) : [...p, u]));

  const submit = async () => {
    const trimmed = name.trim();
    if (!trimmed || saving) return;
    setSaving(true);
    setErr(null);
    try {
      await create("group", trimmed, picked.map((u) => u.id));
      onDone();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "could not create group");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-2">
      <input
        autoFocus
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="Group name"
        className="w-full rounded-md bg-neutral-800 px-2 py-1.5 text-sm outline-none"
      />
      {picked.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {picked.map((u) => (
            <button
              key={u.id}
              onClick={() => toggle(u)}
              className="rounded-full bg-neutral-700 px-2 py-0.5 text-xs hover:bg-neutral-600"
            >
              {u.display_name} ✕
            </button>
          ))}
        </div>
      )}
      <PeopleInput value={query} onChange={setQuery} busy={busy} placeholder="Add people by name or email" />
      <PeopleResults
        results={results}
        query={query}
        busy={busy}
        selectedIds={new Set(picked.map((u) => u.id))}
        onPick={toggle}
      />
      {err && <div className="text-xs text-red-400">{err}</div>}
      <button
        onClick={submit}
        disabled={!name.trim() || saving}
        className="w-full rounded-md bg-blue-600 px-2 py-1.5 text-sm font-medium disabled:opacity-40"
      >
        {saving ? "Creating…" : `Create group${picked.length ? ` with ${picked.length}` : ""}`}
      </button>
    </div>
  );
}

function NewChannel({ onDone }: { onDone: () => void }) {
  const create = useConvStore((s) => s.create);
  const [name, setName] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const submit = async () => {
    const trimmed = name.trim();
    if (!trimmed || saving) return;
    setSaving(true);
    setErr(null);
    try {
      await create("channel", trimmed);
      onDone();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "could not create channel");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-2">
      <input
        autoFocus
        value={name}
        onChange={(e) => setName(e.target.value)}
        onKeyDown={(e) => e.key === "Enter" && submit()}
        placeholder="Channel name"
        className="w-full rounded-md bg-neutral-800 px-2 py-1.5 text-sm outline-none"
      />
      {err && <div className="text-xs text-red-400">{err}</div>}
      <button
        onClick={submit}
        disabled={!name.trim() || saving}
        className="w-full rounded-md bg-blue-600 px-2 py-1.5 text-sm font-medium disabled:opacity-40"
      >
        {saving ? "Creating…" : "Create channel"}
      </button>
    </div>
  );
}

function PeopleInput({
  value,
  onChange,
  busy,
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  busy: boolean;
  placeholder: string;
}) {
  return (
    <div className="relative">
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full rounded-md bg-neutral-800 px-2 py-1.5 text-sm outline-none"
      />
      {busy && <span className="absolute right-2 top-2 text-xs text-neutral-500">…</span>}
    </div>
  );
}

function PeopleResults({
  results,
  query,
  busy,
  selectedIds,
  onPick,
}: {
  results: SearchUser[];
  query: string;
  busy: boolean;
  selectedIds?: Set<string>;
  onPick: (u: SearchUser) => void;
}) {
  if (query.trim().length < 2) return null;
  if (!busy && results.length === 0) {
    return <div className="px-2 py-2 text-xs text-neutral-600">No people found</div>;
  }
  return (
    <ul className="max-h-56 overflow-y-auto">
      {results.map((u) => {
        const selected = selectedIds?.has(u.id) ?? false;
        return (
          <li key={u.id}>
            <button
              onClick={() => onPick(u)}
              className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-neutral-800 ${
                selected ? "bg-neutral-800" : ""
              }`}
            >
              <Avatar name={u.display_name} online={u.online} />
              <span className="min-w-0 flex-1 truncate">{u.display_name}</span>
              {selected && <span className="text-xs text-blue-400">✓</span>}
            </button>
          </li>
        );
      })}
    </ul>
  );
}

// Avatar renders a name initial in a circle with an online dot overlay.
export function Avatar({ name, online }: { name: string; online: boolean }) {
  return (
    <span className="relative inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-neutral-700 text-xs">
      {(name.trim()[0] ?? "?").toUpperCase()}
      <span
        className={`absolute -bottom-0.5 -right-0.5 h-2 w-2 rounded-full ring-2 ring-neutral-950 ${
          online ? "bg-green-500" : "bg-neutral-600"
        }`}
        title={online ? "online" : "offline"}
      />
    </span>
  );
}
