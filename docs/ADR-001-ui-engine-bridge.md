# ADR-001 — Bridging the Next.js UI to the Go engine HTTP API

- **Status:** Accepted (2026-08-16)
- **Deciders:** WinForge agent session `arena/01a00795-winforge`
- **Context files:** `internal/httpapi/server.go` (engine API),
  `next.config.ts` (web app), `src/app/dashboard/` (web dashboard)

## Context

WinForge has two UIs:

1. The **engine dashboard** (`web/`, embedded in `winforge.exe`, served by
   `winforge serve` on `127.0.0.1:8696`) — the real control surface. The
   server deliberately binds loopback only and validates `Host`/`Origin`
   (it is a system-control surface; see the comment at the top of
   `internal/httpapi/server.go`).
2. The **Next.js web simulation** (`src/`) — the catalog of record and a rich
   demo UI backed by PostgreSQL/Drizzle. It simulates applying tweaks; it does
   not touch a real machine.

The question: how should the Next.js app talk to a *running engine* so users
who have both get live data instead of simulation?

## Options considered

**(a) Next.js dev rewrites proxying `/engine/*` → `http://127.0.0.1:8696/*`.**
The Next server (which runs on the same machine as the browser in the
dev/demo scenario) forwards requests server-side, so the browser only ever
talks to the Next origin. No CORS changes in the engine, no engine code
changes at all, works with the engine's loopback-only bind and Host checks
(the proxy sets the target host). Cost: the engine must be running for live
data; when it is not, the UI must degrade gracefully.

**(b) Embed a static export of the Next app in the engine binary.** The Next
app currently uses server components, a database, and server actions —
`next export` is impossible without a major rewrite, and embedding a Node
runtime contradicts the single-static-binary goal. Rejected for now.

**(c) Keep them fully separate.** The status quo. Zero risk, zero benefit.

## Decision

**Option (a)**, implemented minimally and additively:

- `next.config.ts` gains a `rewrites()` entry: `/engine/:path*` →
  `http://127.0.0.1:8696/:path*`. The engine URL can be overridden with the
  `WINFORGE_ENGINE_URL` environment variable (still server-side only).
- A client component `src/components/EngineStatusCard.tsx` polls
  `/engine/api/status` + `/engine/api/health` and renders a live card on the
  dashboard when an engine is reachable. When the fetch fails (engine not
  running — the common case in the hosted demo), the card renders a compact
  "engine offline — simulation mode" note. Failures are silent-by-design:
  the simulation remains fully functional without the engine.
- Nothing in the engine changes. The security posture (loopback bind,
  Host/Origin validation, elevated-mode plugin refusal) is untouched.

## Consequences

- Dev/demo machines running `winforge serve` see live engine state inside the
  Next dashboard — the first real UI↔engine integration.
- The hosted demo (no engine) is unaffected: one failed fetch, then the
  offline note.
- Full UI parity (applying tweaks from the Next UI through the engine) stays
  future work; it would route POSTs through the same `/engine/*` proxy and
  needs a deliberate CSRF/auth story in the engine first (the engine currently
  trusts loopback callers). That design is explicitly out of scope here.
