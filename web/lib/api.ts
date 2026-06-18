import { useAuthStore, refreshSession } from "./auth";

async function unwrap<T>(res: Response): Promise<T> {
  const json = await res.json();
  if (!json.success) throw new Error(json.error?.message ?? "request failed");
  return json.data as T;
}

// authedFetch attaches the in-memory access token and retries once on a 401,
// refreshing the token from the httpOnly cookie (access tokens are short-lived).
async function authedFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const call = (token: string | null) =>
    fetch(`/api${path}`, {
      ...init,
      credentials: "include",
      headers: { ...init.headers, ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    });

  let token = useAuthStore.getState().accessToken;
  if (!token) token = await refreshSession();
  const res = await call(token);
  if (res.status !== 401) return unwrap<T>(res);
  return unwrap<T>(await call(await refreshSession()));
}

export function authedGet<T>(path: string): Promise<T> {
  return authedFetch<T>(path);
}

export function authedPost<T>(path: string, body: unknown): Promise<T> {
  return authedFetch<T>(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}
