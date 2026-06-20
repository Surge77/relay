import { create } from "zustand";
import { authedGet, authedPost } from "./api";

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
  // Peer is the other participant in a DM (empty for channels/groups), so the
  // sidebar can label a DM with a person instead of an opaque id.
  PeerID: string;
  PeerName: string;
  PeerAvatar: string;
  PeerOnline: boolean;
  PeerLastSeen: string | null;
}

interface ConvState {
  conversations: ConversationSummary[];
  activeId: string | null;
  load: () => Promise<void>;
  create: (kind: "channel" | "group", name: string, members?: string[]) => Promise<void>;
  createDM: (userId: string) => Promise<void>;
  setActive: (id: string) => void;
  clearUnread: (id: string) => void;
}

// convLabel renders a conversation's display name: a DM shows the peer's name,
// channels get a leading '#', groups show their name.
export function convLabel(c: Pick<ConversationSummary, "Name" | "Kind" | "ID" | "PeerName">): string {
  if (c.Kind === "dm") return c.PeerName || "Direct message";
  if (c.Name) return c.Kind === "channel" ? `# ${c.Name}` : c.Name;
  return c.ID;
}

export const useConvStore = create<ConvState>((set, get) => ({
  conversations: [],
  activeId: null,

  load: async () => {
    const list = (await authedGet<ConversationSummary[] | null>("/conversations")) ?? [];
    set((st) => ({ conversations: list, activeId: st.activeId ?? list[0]?.ID ?? null }));
  },

  // create makes a channel/group (optionally with initial members), then refetches
  // the list (the create response is a bare conversation, not the summary shape
  // the sidebar renders) and opens the new conversation.
  create: async (kind, name, members = []) => {
    const conv = await authedPost<{ id: string }>("/conversations", { kind, name, members });
    await get().load();
    get().setActive(conv.id);
  },

  // createDM opens (or reuses) the direct message with another user, then opens it.
  createDM: async (userId) => {
    const conv = await authedPost<{ id: string }>("/dms", { user_id: userId });
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
