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

// authedGet calls a REST endpoint with the in-memory access token, refreshing
// once on a 401 (the token is short-lived; the httpOnly refresh cookie renews it).
async function authedGet<T>(path: string): Promise<T> {
  let token = useAuthStore.getState().accessToken;
  if (!token) token = await refreshSession();
  const res = await fetch(`/api${path}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    credentials: "include",
  });
  if (res.status !== 401) return unwrap<T>(res);

  const fresh = await refreshSession();
  const retry = await fetch(`/api${path}`, {
    headers: fresh ? { Authorization: `Bearer ${fresh}` } : {},
    credentials: "include",
  });
  return unwrap<T>(retry);
}

export const useConvStore = create<ConvState>((set, get) => ({
  conversations: [],
  activeId: null,

  load: async () => {
    const list = (await authedGet<ConversationSummary[] | null>("/conversations")) ?? [];
    set((st) => ({ conversations: list, activeId: st.activeId ?? list[0]?.ID ?? null }));
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
