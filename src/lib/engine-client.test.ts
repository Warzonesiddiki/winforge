import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock global.fetch. Each test gets a fresh engine-client module via
// resetModules so the cached token promise does not leak between tests.
const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

async function freshClient() {
  vi.resetModules();
  vi.stubGlobal("fetch", mockFetch);
  return import("./engine-client");
}

describe("engine-client", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  function jsonResponse(body: unknown, init: ResponseInit = {}) {
    return new Response(JSON.stringify(body), {
      status: 200,
      headers: { "Content-Type": "application/json" },
      ...init,
    });
  }

  it("sends GET requests to /engine/* without a session token", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ ok: true }));
    const { engineClient } = await freshClient();
    const result = await engineClient.get<{ ok: boolean }>("/api/status");
    expect(result).toEqual({ ok: true });
    expect(mockFetch).toHaveBeenCalledTimes(1);
    const [url, init] = mockFetch.mock.calls[0];
    expect(url).toBe("/engine/api/status");
    // GET is the default; the client does not set X-WinForge-Token.
    expect(init.headers.get("X-WinForge-Token")).toBeNull();
    expect(init.cache).toBe("no-store");
  });

  it("fetches and attaches a session token for POST requests", async () => {
    mockFetch
      .mockResolvedValueOnce(jsonResponse({ token: "secret-token" }))
      .mockResolvedValueOnce(jsonResponse({ changed: true }));
    const { engineClient } = await freshClient();
    const result = await engineClient.post<{ changed: boolean }>("/api/tweaks/apply", { id: "x" });

    expect(result).toEqual({ changed: true });
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockFetch.mock.calls[0][0]).toBe("/engine/api/session-token");
    const [, postInit] = mockFetch.mock.calls[1];
    expect(postInit.method).toBe("POST");
    expect(postInit.headers.get("X-WinForge-Token")).toBe("secret-token");
    expect(postInit.body).toBe(JSON.stringify({ id: "x" }));
  });

  it("caches the token across multiple POST requests", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === "/engine/api/session-token") {
        return Promise.resolve(jsonResponse({ token: "cached" }));
      }
      return Promise.resolve(jsonResponse({ ok: true }));
    });
    const { engineClient } = await freshClient();
    await engineClient.post("/api/x", {});
    await engineClient.post("/api/y", {});

    const tokenCalls = mockFetch.mock.calls.filter((c) => c[0] === "/engine/api/session-token");
    expect(tokenCalls).toHaveLength(1);
  });

  it("throws EngineError with the server message on failure", async () => {
    mockFetch
      .mockResolvedValueOnce(jsonResponse({ token: "t" }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ error: "invalid token" }), {
          status: 401,
          headers: { "Content-Type": "application/json" },
        })
      );
    const { engineClient, EngineError } = await freshClient();
    const err = await engineClient.post("/api/x", {}).catch((e) => e);
    expect(err).toBeInstanceOf(EngineError);
    expect(err).toMatchObject({ message: "invalid token", status: 401 });
  });

  it("clears the cached token on 401 so the next request re-fetches", async () => {
    mockFetch
      // First attempt: token fetch succeeds, POST returns 401
      .mockResolvedValueOnce(jsonResponse({ token: "old" }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ error: "unauthorized" }), { status: 401 })
      )
      // After 401 the token promise is nulled; a fresh POST should re-fetch
      .mockResolvedValueOnce(jsonResponse({ token: "new" }))
      .mockResolvedValueOnce(jsonResponse({ ok: true }));
    const { engineClient } = await freshClient();
    await expect(engineClient.post("/api/x", {})).rejects.toThrow();
    await engineClient.post("/api/x", {});

    const tokenCalls = mockFetch.mock.calls.filter((c) => c[0] === "/engine/api/session-token");
    expect(tokenCalls).toHaveLength(2);
  });

  it("del sends DELETE with the token", async () => {
    mockFetch
      .mockResolvedValueOnce(jsonResponse({ token: "t" }))
      .mockResolvedValueOnce(jsonResponse({ ok: true }));
    const { engineClient } = await freshClient();
    await engineClient.del("/api/x");
    const [, delInit] = mockFetch.mock.calls[1];
    expect(delInit.method).toBe("DELETE");
    expect(delInit.headers.get("X-WinForge-Token")).toBe("t");
  });
});
