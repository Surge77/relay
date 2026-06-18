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
    return <div className="grid h-screen place-items-center text-neutral-400">Loading…</div>;
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
    <div className="grid h-screen place-items-center bg-neutral-950 text-neutral-100">
      <form onSubmit={submit} className="w-80 space-y-3 rounded-xl border border-neutral-800 p-6">
        <h1 className="text-lg font-semibold">
          {mode === "login" ? "Log in to Relay" : "Create your Relay account"}
        </h1>
        {mode === "signup" && (
          <input
            className="w-full rounded-lg bg-neutral-800 px-3 py-2 text-sm"
            placeholder="Display name"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            required
          />
        )}
        <input
          className="w-full rounded-lg bg-neutral-800 px-3 py-2 text-sm"
          type="email"
          placeholder="Email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
        />
        <input
          className="w-full rounded-lg bg-neutral-800 px-3 py-2 text-sm"
          type="password"
          placeholder="Password (min 8 chars)"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
        />
        {error && <p className="text-sm text-red-400">{error}</p>}
        <button
          type="submit"
          disabled={busy}
          className="w-full rounded-lg bg-blue-600 py-2 text-sm font-medium disabled:opacity-50"
        >
          {busy ? "…" : mode === "login" ? "Log in" : "Sign up"}
        </button>
        <button
          type="button"
          className="w-full text-xs text-neutral-400 hover:text-neutral-200"
          onClick={() => setMode(mode === "login" ? "signup" : "login")}
        >
          {mode === "login" ? "Need an account? Sign up" : "Have an account? Log in"}
        </button>
      </form>
    </div>
  );
}
