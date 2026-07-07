// Every conversation (and person) owns a colour. It themes their avatar, their
// active row, and — for a DM — the bubbles you send them, so a chat is
// identifiable at a glance. Deterministic: the same id always maps to the same
// hue.

export interface Hue {
  /** Flat fill (CSS var reference). */
  fill: string;
  /** Foreground that reads on `fill`: ink or paper. */
  on: string;
}

const PALETTE: Hue[] = [
  { fill: "var(--color-cobalt)", on: "var(--color-paper)" },
  { fill: "var(--color-tomato)", on: "var(--color-paper)" },
  { fill: "var(--color-lime)", on: "var(--color-ink)" },
  { fill: "var(--color-butter)", on: "var(--color-ink)" },
  { fill: "var(--color-lilac)", on: "var(--color-ink)" },
  { fill: "var(--color-mint)", on: "var(--color-paper)" },
  { fill: "var(--color-sky)", on: "var(--color-ink)" },
];

export function hueFor(id: string): Hue {
  let h = 0;
  for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) >>> 0;
  return PALETTE[h % PALETTE.length];
}
