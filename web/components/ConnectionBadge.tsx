"use client";

import { useChatStore } from "@/lib/store";

const LABEL: Record<string, { dot: string; text: string }> = {
  connecting: { dot: "bg-yellow-400", text: "connecting" },
  connected: { dot: "bg-green-500", text: "connected" },
  reconnecting: { dot: "bg-yellow-400 animate-pulse", text: "reconnecting" },
  down: { dot: "bg-red-500", text: "down" },
};

export function ConnectionBadge() {
  const status = useChatStore((s) => s.status);
  const { dot, text } = LABEL[status] ?? LABEL.down;
  return (
    <div className="flex items-center gap-2 text-sm text-neutral-300">
      <span className={`inline-block h-2.5 w-2.5 rounded-full ${dot}`} aria-hidden />
      <span>{text}</span>
    </div>
  );
}
