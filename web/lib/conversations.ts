import { create } from "zustand";
import { useAuthStore, refreshSession } from "./auth";

// Wire shape from GET /conversations. The list endpoint serializes the Go model
// structs directly (no json tags), so these fields are PascalCase — unlike the
// snake_case the hand-written view structs use elsewhere in the API.
export interface LastMessage {
  Seq: number;
  SenderID: string;
  Body: string;
  TS: number;
}

export interface ConversationSummary {
  ID: string;
  Kind: string; // channel | dm | group
  Name: string;
  CreatedBy: string;
  UnreadCount: number;
  LastMessage: LastMessage | null;
}

interface ConvState {
  conversations: ConversationSummary[];
  activeId: string | null;
  load: () => Promise<void>;
  create: (kind: "channel" | "group", name: string) => Promise<void>;
  setActive: (id: string) => void;
  clearUnread: (id: string) => void;
}

// convLabel renders a conversation's display name. DMs carry no name (peer-name
// labelling is a later step); channels get a leading '#'.
export function convLabel(c: Pick<ConversationSummary, "Name" | "Kind" | "ID">): string {
  if (c.Name) return c.Kind === "channel" ? `# ${c.Name}` : c.Name;
  return c.Kind === "dm" ? "Direct message" : c.ID;
}

async function unwrap<T>(res: Response): Promise<T> {
  const json = await res.json();
  if (!json.success) throw new Error(json.error?.message ?? "request failed");
  return json.data as T;
}

// authedFetch attaches the in-memory access token and retries once on a 401,
// refreshing the token from the httpOnly cookie (access tokens are short-lived).
async function authedFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const call = (token: string | null) =>
    fetch(`/api${path}`, {
      ...init,
      credentials: "include",
      headers: { ...init.headers, ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    });

  let token = useAuthStore.getState().accessToken;
  if (!token) token = await refreshSession();
  const res = await call(token);
  if (res.status !== 401) return unwrap<T>(res);
  return unwrap<T>(await call(await refreshSession()));
}

function authedGet<T>(path: string): Promise<T> {
  return authedFetch<T>(path);
}

function authedPost<T>(path: string, body: unknown): Promise<T> {
  return authedFetch<T>(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export const useConvStore = create<ConvState>((set, get) => ({
  conversations: [],
  activeId: null,

  load: async () => {
    const list = (await authedGet<ConversationSummary[] | null>("/conversations")) ?? [];
    set((st) => ({ conversations: list, activeId: st.activeId ?? list[0]?.ID ?? null }));
  },

  // create makes a channel/group, then refetches the list (the create response
  // is a bare conversation, not the summary shape the sidebar renders) and opens
  // the new conversation.
  create: async (kind, name) => {
    const conv = await authedPost<{ id: string }>("/conversations", { kind, name, members: [] });
    await get().load();
    get().setActive(conv.id);
  },

  setActive: (id) => {
    set({ activeId: id });
    get().clearUnread(id);
  },

  clearUnread: (id) =>
    set((st) => ({
      conversations: st.conversations.map((c) => (c.ID === id ? { ...c, UnreadCount: 0 } : c)),
    })),
}));
