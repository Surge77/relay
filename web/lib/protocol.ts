// Wire frames exchanged with the gateway. Mirrors gateway/internal/protocol.

export type ClientFrameType =
  | "send"
  | "subscribe"
  | "typing"
  | "read"
  | "ping"
  | "edit"
  | "delete"
  | "react"
  | "unreact";
export type ServerFrameType =
  | "ack"
  | "message"
  | "presence"
  | "receipt"
  | "caughtup"
  | "pong"
  | "error"
  | "conversation_created"
  | "conversation_updated"
  | "member_added"
  | "member_removed"
  | "message_edited"
  | "message_deleted"
  | "reaction_added"
  | "reaction_removed";

export interface Frame {
  type: ClientFrameType | ServerFrameType;
  client_msg_id?: string;
  conversation_id?: string;
  body?: string;
  seq?: number;
  last_acked_seq?: number;
  sender_id?: string;
  user_id?: string;
  state?: string;
  ts?: number;
  code?: string;
  message?: string;
  kind?: string;
  name?: string;
  actor_id?: string;
}

export interface ChatMessage {
  seq: number;
  senderId: string;
  clientMsgId: string;
  body: string;
  ts: number;
  pending: boolean; // optimistic, not yet acked
}
