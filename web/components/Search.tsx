"use client";

import { useSearch } from "@/lib/search";
import { useConvStore, convLabel } from "@/lib/conversations";

// Search runs a debounced full-text query and lists matching messages. Clicking
// a hit opens its conversation and clears the query.
export function Search() {
  const { query, setQuery, results, busy } = useSearch();
  const conversations = useConvStore((s) => s.conversations);
  const setActive = useConvStore((s) => s.setActive);

  const labelFor = (id: string) => {
    const c = conversations.find((x) => x.ID === id);
    return c ? convLabel(c) : id;
  };

  const open = query.trim().length > 0;

  return (
    <div className="relative">
      <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink">
        <svg viewBox="0 0 24 24" fill="none" className="h-4 w-4" stroke="currentColor" strokeWidth="2.4">
          <circle cx="11" cy="11" r="7" />
          <path d="m20 20-3.2-3.2" strokeLinecap="round" />
        </svg>
      </span>
      <input
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Search messages"
        className="card-flat w-full rounded-lg py-2.5 pl-9 pr-3 text-[13px] font-medium text-ink outline-none placeholder:text-ash focus:shadow-[var(--sh-sm)]"
      />
      {open && (
        <div className="card absolute left-0 right-0 top-full z-20 mt-2 max-h-72 space-y-1 overflow-y-auto rounded-lg p-2" style={{ boxShadow: "var(--sh)" }}>
          {busy && <div className="px-2 py-2 font-mono text-xs text-ash">Searching…</div>}
          {!busy && results.length === 0 && (
            <div className="px-2 py-2 font-mono text-xs text-ash">No matches</div>
          )}
          {results.map((r) => (
            <button
              key={`${r.conversation_id}:${r.seq}`}
              onClick={() => {
                setActive(r.conversation_id);
                setQuery("");
              }}
              className="block w-full rounded-md px-2.5 py-1.5 text-left hover:bg-bone"
            >
              <span className="block truncate font-mono text-[10px] font-bold uppercase tracking-wide text-cobalt">
                {labelFor(r.conversation_id)}
              </span>
              <span className="mt-0.5 block truncate text-[13px] font-medium text-ink">{r.body}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
