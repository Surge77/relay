"use client";

import { useState } from "react";
import { useConvStore } from "@/lib/conversations";
import { useUserSearch, type SearchUser } from "@/lib/users";
import { hueFor } from "@/lib/hue";

type Mode = "dm" | "group" | "channel";

// NewChat is the single entry point for starting a conversation: a direct
// message (find a person), a group (name + pick people), or a channel (name).
export function NewChat() {
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState<Mode>("dm");

  if (!open) {
    return (
      <button
        onClick={() => setOpen(true)}
        className="btn flex w-full items-center justify-center gap-2 rounded-lg bg-lime px-3 py-2.5 text-sm font-extrabold uppercase tracking-wide text-ink"
      >
        <span className="text-lg leading-none">+</span> New conversation
      </button>
    );
  }

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-ink/40 p-4 pt-24">
      <div className="card w-full max-w-md rounded-xl p-5" style={{ boxShadow: "var(--sh-lg)" }}>
        <div className="mb-4 flex items-center justify-between">
          <div className="ink-border flex gap-0 overflow-hidden rounded-lg">
            {(["dm", "group", "channel"] as const).map((m) => (
              <button
                key={m}
                onClick={() => setMode(m)}
                className={`border-r-2 border-ink px-3 py-1.5 text-xs font-bold uppercase tracking-wide last:border-r-0 ${
                  mode === m ? "bg-cobalt text-paper" : "bg-paper text-ink hover:bg-bone"
                }`}
              >
                {m === "dm" ? "Direct" : m}
              </button>
            ))}
          </div>
          <button
            onClick={() => setOpen(false)}
            className="btn grid h-8 w-8 place-items-center rounded-lg bg-paper text-ink hover:bg-tomato hover:text-paper"
          >
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
    <div className="space-y-2.5">
      <PeopleInput value={query} onChange={setQuery} busy={busy} placeholder="Search people by name or email" />
      {err && <ErrLine msg={err} />}
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
    <div className="space-y-2.5">
      <TextField value={name} onChange={setName} placeholder="Group name" autoFocus />
      {picked.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {picked.map((u) => (
            <button
              key={u.id}
              onClick={() => toggle(u)}
              className="ink-border rounded-md bg-lilac px-2 py-0.5 text-xs font-bold text-ink"
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
      {err && <ErrLine msg={err} />}
      <PrimaryButton
        onClick={submit}
        disabled={!name.trim() || saving}
        label={saving ? "Creating…" : `Create group${picked.length ? ` with ${picked.length}` : ""}`}
      />
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
    <div className="space-y-2.5">
      <TextField value={name} onChange={setName} placeholder="Channel name" autoFocus onEnter={submit} />
      {err && <ErrLine msg={err} />}
      <PrimaryButton
        onClick={submit}
        disabled={!name.trim() || saving}
        label={saving ? "Creating…" : "Create channel"}
      />
    </div>
  );
}

function ErrLine({ msg }: { msg: string }) {
  return <div className="ink-border rounded-md bg-tomato px-2 py-1 text-xs font-bold text-paper">{msg}</div>;
}

function TextField({
  value,
  onChange,
  placeholder,
  autoFocus,
  onEnter,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
  autoFocus?: boolean;
  onEnter?: () => void;
}) {
  return (
    <input
      autoFocus={autoFocus}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      onKeyDown={(e) => e.key === "Enter" && onEnter?.()}
      placeholder={placeholder}
      className="card-flat w-full rounded-lg px-3 py-2.5 text-sm text-ink outline-none placeholder:text-ash focus:shadow-[var(--sh-sm)]"
    />
  );
}

function PrimaryButton({ onClick, disabled, label }: { onClick: () => void; disabled: boolean; label: string }) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className="btn w-full rounded-lg bg-cobalt py-2.5 text-sm font-extrabold uppercase tracking-wide text-paper disabled:opacity-40"
    >
      {label}
    </button>
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
        className="card-flat w-full rounded-lg px-3 py-2.5 text-sm text-ink outline-none placeholder:text-ash focus:shadow-[var(--sh-sm)]"
      />
      {busy && <span className="absolute right-3 top-3 font-mono text-xs text-ash">…</span>}
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
    return <div className="px-1 py-2 text-xs font-medium text-ash">No people found</div>;
  }
  return (
    <ul className="max-h-56 space-y-1 overflow-y-auto">
      {results.map((u) => {
        const selected = selectedIds?.has(u.id) ?? false;
        return (
          <li key={u.id}>
            <button
              onClick={() => onPick(u)}
              className={`flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left text-sm ${
                selected ? "ink-border bg-lime" : "hover:bg-bone"
              }`}
            >
              <Avatar name={u.display_name} online={u.online} id={u.id} />
              <span className="min-w-0 flex-1 truncate font-medium text-ink">{u.display_name}</span>
              {selected && <span className="font-bold text-ink">✓</span>}
            </button>
          </li>
        );
      })}
    </ul>
  );
}

// Avatar — a colour-coded, ink-bordered tile with the person's initial and an
// online flag. The colour is derived from the id, so a person looks the same
// everywhere they appear.
export function Avatar({ name, online, id }: { name: string; online: boolean; id?: string }) {
  const hue = hueFor(id ?? name);
  return (
    <span
      className="relative inline-grid h-9 w-9 shrink-0 place-items-center rounded-lg text-sm font-extrabold ink-border"
      style={{ background: hue.fill, color: hue.on }}
    >
      {(name.trim()[0] ?? "?").toUpperCase()}
      <span
        className={`absolute -bottom-1 -right-1 h-3 w-3 rounded-full ink-border ${online ? "bg-mint" : "bg-bone"}`}
        title={online ? "online" : "offline"}
      />
    </span>
  );
}
