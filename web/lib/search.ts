import { useEffect, useState } from "react";
import { authedGet } from "./api";

// SearchResult mirrors the gateway's messageView (snake_case) returned by
// GET /search — a message hit within one of the caller's conversations.
export interface SearchResult {
  conversation_id: string;
  seq: number;
  sender_id: string;
  body: string;
  ts: number;
}

const DEBOUNCE_MS = 250;

// useSearch debounces the query and runs full-text search over the caller's
// messages. Each keystroke cancels the prior in-flight request via the cleanup
// flag so a slow earlier response can't overwrite a newer one.
export function useSearch() {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const q = query.trim();
    if (!q) {
      setResults([]);
      setBusy(false);
      return;
    }
    setBusy(true);
    let cancelled = false;
    const timer = setTimeout(async () => {
      try {
        const hits = (await authedGet<SearchResult[] | null>(`/search?q=${encodeURIComponent(q)}`)) ?? [];
        if (!cancelled) setResults(hits);
      } catch {
        if (!cancelled) setResults([]);
      } finally {
        if (!cancelled) setBusy(false);
      }
    }, DEBOUNCE_MS);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [query]);

  return { query, setQuery, results, busy };
}
