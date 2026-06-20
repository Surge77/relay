import { useEffect, useState } from "react";
import { authedGet } from "./api";

// SearchUser mirrors the gateway's profileView (snake_case) from
// GET /users/search — a public profile, never the email.
export interface SearchUser {
  id: string;
  display_name: string;
  avatar_url?: string;
  status_text?: string;
  online: boolean;
  last_seen_at?: string | null;
}

const DEBOUNCE_MS = 250;
const MIN_QUERY = 2; // matches the gateway's minimum query length

// useUserSearch debounces the query and searches people by name/email so the
// caller can start a DM without knowing a raw user id. Each keystroke cancels the
// prior in-flight request so a slow earlier response can't overwrite a newer one.
export function useUserSearch() {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchUser[]>([]);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const q = query.trim();
    if (q.length < MIN_QUERY) {
      setResults([]);
      setBusy(false);
      return;
    }
    setBusy(true);
    let cancelled = false;
    const timer = setTimeout(async () => {
      try {
        const hits = (await authedGet<SearchUser[] | null>(`/users/search?q=${encodeURIComponent(q)}`)) ?? [];
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
