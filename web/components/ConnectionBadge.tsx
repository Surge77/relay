"use client";

import { useChatStore } from "@/lib/store";

const LABEL: Record<string, { dot: string; text: string; blink?: boolean }> = {
  connecting: { dot: "bg-butter", text: "Connecting" },
  connected: { dot: "bg-mint", text: "Live" },
  reconnecting: { dot: "bg-butter", text: "Retrying", blink: true },
  down: { dot: "bg-tomato", text: "Offline" },
};

export function ConnectionBadge() {
  const status = useChatStore((s) => s.status);
  const { dot, text, blink } = LABEL[status] ?? LABEL.down;
  return (
    <div className="ink-border flex items-center gap-1.5 rounded-md bg-bone px-2 py-1 font-mono text-[10px] font-bold uppercase tracking-wide">
      <span className={`inline-block h-2.5 w-2.5 rounded-full ink-border ${dot} ${blink ? "blink" : ""}`} aria-hidden />
      {text}
    </div>
  );
}
