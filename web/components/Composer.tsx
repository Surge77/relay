"use client";

import { useEffect, useRef, useState } from "react";
import type { Hue } from "@/lib/hue";

const MAX_ROWS_PX = 160;

export function Composer({
  onSend,
  onTyping,
  placeholder = "Message",
  hue,
  focusKey,
}: {
  onSend: (body: string) => void;
  onTyping: (start: boolean) => void;
  placeholder?: string;
  hue?: Hue;
  /** Changing this refocuses the field — e.g. when the open conversation changes. */
  focusKey?: string;
}) {
  const [text, setText] = useState("");
  const typingTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const ta = useRef<HTMLTextAreaElement | null>(null);

  // Auto-grow the textarea up to a cap, then scroll internally.
  const resize = () => {
    const el = ta.current;
    if (!el) return;
    el.style.height = "0px";
    el.style.height = `${Math.min(el.scrollHeight, MAX_ROWS_PX)}px`;
  };

  useEffect(() => {
    resize();
  }, [text]);

  // Refocus when the open conversation changes.
  useEffect(() => {
    ta.current?.focus();
  }, [focusKey]);

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
    requestAnimationFrame(resize);
  };

  const canSend = !!text.trim();
  const fill = hue?.fill ?? "var(--color-cobalt)";
  const on = hue?.on ?? "var(--color-paper)";

  return (
    <div className="px-4 pb-5 pt-1 sm:px-6">
      <div className="flex items-end gap-2.5">
        <div className="card-flat flex-1 rounded-lg focus-within:shadow-[var(--sh-sm)]">
          <textarea
            ref={ta}
            rows={1}
            className="block max-h-40 w-full resize-none bg-transparent px-4 py-3 text-[15px] leading-snug text-ink outline-none placeholder:text-ash"
            placeholder={placeholder}
            value={text}
            onChange={(e) => handleChange(e.target.value)}
            onKeyDown={(e) => {
              // Enter sends; Shift+Enter inserts a newline.
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                submit();
              }
            }}
          />
        </div>
        <button
          className="btn grid h-12 w-14 shrink-0 place-items-center rounded-lg disabled:cursor-not-allowed disabled:opacity-40"
          style={canSend ? { background: fill, color: on } : { background: "var(--color-bone)", color: "var(--color-ash)" }}
          onClick={submit}
          disabled={!canSend}
          aria-label="Send message"
          title="Send  ·  Enter"
        >
          <svg viewBox="0 0 24 24" fill="none" className="h-5 w-5" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M4 12l16-8-6 16-3.5-6L4 12Z" />
          </svg>
        </button>
      </div>
      <div className="mt-1.5 pl-1 font-mono text-[10px] text-ash">
        <span className="ink-border rounded bg-bone px-1 py-0.5">Enter</span> send
        <span className="mx-1.5">·</span>
        <span className="ink-border rounded bg-bone px-1 py-0.5">Shift</span>
        <span className="mx-0.5">+</span>
        <span className="ink-border rounded bg-bone px-1 py-0.5">Enter</span> newline
      </div>
    </div>
  );
}
