"use client";

import { useRef, useState } from "react";

export function Composer({
  onSend,
  onTyping,
  placeholder = "Message",
}: {
  onSend: (body: string) => void;
  onTyping: (start: boolean) => void;
  placeholder?: string;
}) {
  const [text, setText] = useState("");
  const typingTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const handleChange = (v: string) => {
    setText(v);
    onTyping(true);
    if (typingTimer.current) clearTimeout(typingTimer.current);
    typingTimer.current = setTimeout(() => onTyping(false), 1500);
  };

  const submit = () => {
    const body = text.trim();
    if (!body) return;
    onSend(body);
    setText("");
    onTyping(false);
  };

  return (
    <div className="flex gap-2 border-t border-neutral-800 p-3">
      <input
        className="flex-1 rounded-lg bg-neutral-800 px-3 py-2 text-sm outline-none"
        placeholder={placeholder}
        value={text}
        onChange={(e) => handleChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") submit();
        }}
      />
      <button
        className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium disabled:opacity-40"
        onClick={submit}
        disabled={!text.trim()}
      >
        Send
      </button>
    </div>
  );
}
