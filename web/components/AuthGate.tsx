"use client";

import { useEffect, useState } from "react";
import { useAuthStore, login, signup, refreshSession } from "@/lib/auth";

// AuthGate blocks the app behind authentication. On mount it attempts a silent
// refresh (using the httpOnly cookie); if that fails it shows the login/signup
// form. Children render only once a user is present.
export function AuthGate({ children }: { children: React.ReactNode }) {
  const user = useAuthStore((s) => s.user);
  const ready = useAuthStore((s) => s.ready);
  const setReady = useAuthStore((s) => s.setReady);

  useEffect(() => {
    refreshSession().finally(() => setReady());
  }, [setReady]);

  if (!ready) {
    return (
      <div className="grid h-screen place-items-center">
        <div className="ink-border rounded-lg bg-paper px-4 py-2 font-mono text-sm font-bold uppercase">
          Loading…
        </div>
      </div>
    );
  }
  if (!user) return <AuthForm />;
  return <>{children}</>;
}

function AuthForm() {
  const [mode, setMode] = useState<"login" | "signup">("signup");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (mode === "login") await login(email, password);
      else await signup(email, password, displayName);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid h-screen place-items-center px-4 text-ink">
      <div className="w-full max-w-[400px]">
        {/* Brand block */}
        <div className="mb-6 flex items-end gap-3">
          <div
            className="grid h-14 w-14 -rotate-3 place-items-center rounded-xl bg-cobalt text-paper ink-border"
            style={{ boxShadow: "var(--sh)" }}
          >
            <svg viewBox="0 0 24 24" fill="none" className="h-8 w-8">
              <path d="M13.5 3 5 13.5h5l-1.5 7.5L18 10.5h-5L13.5 3Z" fill="currentColor" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
            </svg>
          </div>
          <div>
            <h1 className="font-display text-5xl font-extrabold uppercase leading-none tracking-tight">Relay</h1>
            <div className="mt-1.5 flex gap-1">
              <span className="h-2 w-8 bg-tomato" />
              <span className="h-2 w-4 bg-lime" />
              <span className="h-2 w-5 bg-butter" />
            </div>
          </div>
        </div>

        <div className="card rounded-xl p-6" style={{ boxShadow: "var(--sh-lg)" }}>
          <h2 className="mb-1 font-display text-2xl font-extrabold uppercase leading-none">
            {mode === "login" ? "Welcome back" : "Get in"}
          </h2>
          <p className="mb-5 text-sm text-ash">
            {mode === "login" ? "Pick up your conversations." : "Real-time messaging that keeps your place."}
          </p>

          <form onSubmit={submit} className="space-y-3">
            {mode === "signup" && <Field placeholder="Display name" value={displayName} onChange={setDisplayName} required />}
            <Field type="email" placeholder="Email" value={email} onChange={setEmail} required />
            <Field type="password" placeholder="Password (min 8 chars)" value={password} onChange={setPassword} required />
            {error && (
              <p className="ink-border rounded-md bg-tomato px-3 py-2 text-sm font-bold text-paper">{error}</p>
            )}
            <button
              type="submit"
              disabled={busy}
              className="btn w-full rounded-lg bg-cobalt py-3 text-sm font-extrabold uppercase tracking-wide text-paper disabled:opacity-60"
            >
              {busy ? "…" : mode === "login" ? "Log in" : "Create account"}
            </button>
          </form>
        </div>

        <button
          type="button"
          className="mt-5 w-full text-center text-sm font-medium text-ash hover:text-ink"
          onClick={() => setMode(mode === "login" ? "signup" : "login")}
        >
          {mode === "login" ? (
            <>Need an account? <span className="font-extrabold text-cobalt underline decoration-2 underline-offset-2">Sign up</span></>
          ) : (
            <>Have an account? <span className="font-extrabold text-cobalt underline decoration-2 underline-offset-2">Log in</span></>
          )}
        </button>
      </div>
    </div>
  );
}

function Field({
  type = "text",
  placeholder,
  value,
  onChange,
  required,
}: {
  type?: string;
  placeholder: string;
  value: string;
  onChange: (v: string) => void;
  required?: boolean;
}) {
  return (
    <input
      className="card-flat w-full rounded-lg px-3.5 py-2.5 text-sm font-medium text-ink outline-none placeholder:text-ash focus:shadow-[var(--sh-sm)]"
      type={type}
      placeholder={placeholder}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      required={required}
    />
  );
}
