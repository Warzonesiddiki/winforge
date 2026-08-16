// Browser client for the Go engine's HTTP API, reached through the Next
// /engine/* rewrite (next.config.ts). ADR-002 requires a per-instance session
// token on every mutating request: this helper fetches it once from
// /engine/api/session-token (loopback + same-origin only) and echoes it in
// X-WinForge-Token. Read requests need no token.

const TOKEN_HEADER = "X-WinForge-Token";

let tokenPromise: Promise<string> | null = null;

async function getToken(): Promise<string> {
  if (!tokenPromise) {
    tokenPromise = fetch("/engine/api/session-token", { cache: "no-store" })
      .then(async (r) => {
        if (!r.ok) throw new Error(`engine session token: ${r.status}`);
        const b = (await r.json()) as { token: string };
        return b.token;
      })
      .catch((err) => {
        // Drop the cached promise so a later retry can re-fetch after the
        // engine restarts (which rotates the token).
        tokenPromise = null;
        throw err;
      });
  }
  return tokenPromise;
}

export class EngineError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = "EngineError";
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = (init.method ?? "GET").toUpperCase();
  const headers = new Headers(init.headers);
  headers.set("Content-Type", "application/json");
  if (method !== "GET" && method !== "HEAD") {
    headers.set(TOKEN_HEADER, await getToken());
  }
  const res = await fetch(`/engine${path}`, { ...init, headers, cache: "no-store" });
  if (!res.ok) {
    let msg = res.statusText;
    try {
      const b = await res.json();
      if (b?.error) msg = b.error;
    } catch {
      // ignore non-JSON error body
    }
    if (res.status === 401) {
      tokenPromise = null;
    }
    throw new EngineError(msg, res.status);
  }
  return res.json() as Promise<T>;
}

export const engineClient = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "POST", body: body ? JSON.stringify(body) : undefined }),
  del: <T>(path: string) => request<T>(path, { method: "DELETE" }),
};
