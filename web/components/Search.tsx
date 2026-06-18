"use client";

import { useSearch } from "@/lib/search";
import { useConvStore, convLabel } from "@/lib/conversations";

// Search runs a debounced full-text query and lists matching messages. Clicking
// a hit opens its conversation (scroll-to-seq is a later refinement) and clears
// the query.
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
    <div>
      <input
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Search messages"
        className="w-full rounded-lg bg-neutral-800 px-3 py-1.5 text-sm outline-none"
      />
      {open && (
        <div className="mt-1 max-h-72 space-y-1 overflow-y-auto rounded-lg border border-neutral-800 bg-neutral-950 p-1">
          {busy && <div className="px-2 py-1 text-xs text-neutral-500">Searching…</div>}
          {!busy && results.length === 0 && (
            <div className="px-2 py-1 text-xs text-neutral-600">No matches</div>
          )}
          {results.map((r) => (
            <button
              key={`${r.conversation_id}:${r.seq}`}
              onClick={() => {
                setActive(r.conversation_id);
                setQuery("");
              }}
              className="block w-full rounded-md px-2 py-1.5 text-left hover:bg-neutral-800"
            >
              <span className="block truncate text-xs text-neutral-400">{labelFor(r.conversation_id)}</span>
              <span className="block truncate text-sm">{r.body}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
