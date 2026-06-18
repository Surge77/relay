import { create } from "zustand";

export interface AuthUser {
  id: string;
  email: string;
  display_name: string;
}

interface AuthState {
  user: AuthUser | null;
  accessToken: string | null;
  ready: boolean; // initial silent-refresh attempt finished
  setSession: (user: AuthUser, accessToken: string) => void;
  clear: () => void;
  setReady: () => void;
}

// Access token lives in memory only (never localStorage) to limit XSS blast
// radius; the refresh token is an httpOnly cookie the browser never exposes to JS.
export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  accessToken: null,
  ready: false,
  setSession: (user, accessToken) => set({ user, accessToken }),
  clear: () => set({ user: null, accessToken: null }),
  setReady: () => set({ ready: true }),
}));

interface SessionData {
  user: AuthUser;
  access_token: string;
}

async function postSession(path: string, body: unknown): Promise<SessionData> {
  const res = await fetch(`/api${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(body),
  });
  const json = await res.json();
  if (!json.success) throw new Error(json.error?.message ?? "request failed");
  const data = json.data as SessionData;
  useAuthStore.getState().setSession(data.user, data.access_token);
  return data;
}

export async function login(email: string, password: string): Promise<void> {
  await postSession("/auth/login", { email, password });
}

export async function signup(email: string, password: string, displayName: string): Promise<void> {
  await postSession("/auth/signup", { email, password, display_name: displayName });
}

export async function logout(): Promise<void> {
  await fetch("/api/auth/logout", { method: "POST", credentials: "include" }).catch(() => {});
  useAuthStore.getState().clear();
}

// refreshSession exchanges the httpOnly refresh cookie for a new access token.
// Used on page load (silent login) and when the socket is rejected for an
// expired token. Returns the new access token, or null if not authenticated.
export async function refreshSession(): Promise<string | null> {
  try {
    const data = await postSession("/auth/refresh", {});
    return data.access_token;
  } catch {
    useAuthStore.getState().clear();
    return null;
  }
}
