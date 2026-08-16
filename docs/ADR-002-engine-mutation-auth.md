# ADR-002 — Engine HTTP mutation authentication (loopback session token)

- **Status:** Accepted (2026-08-16)
- **Deciders:** WinForge agent session `arena/01a00a47-winforge`
- **Context files:** `internal/httpapi/server.go`, `web/` (embedded dashboard),
  `next.config.ts`, `src/components/EngineStatusCard.tsx`,
  `docs/ADR-001-ui-engine-bridge.md`

## Context

The engine dashboard server binds **loopback only** and already enforces, in
`Server.ServeHTTP`:

1. **Loopback Host check** — any `Host` that is not `localhost` or a loopback
   IP is rejected (`isLoopbackAuthority`), defeating DNS rebinding.
2. **Same-origin check on mutations** — `POST`/`PUT`/`PATCH`/`DELETE` require
   a matching `Origin`/`Host`/scheme/port; `Sec-Fetch-Site: cross-site` is
   rejected (`isSameOrigin`). Browser-driven CSRF is therefore already
   blocked.
3. Standard hardening headers (`X-Frame-Options: DENY`,
   `frame-ancestors 'none'`, `X-Content-Type-Options: nosniff`,
   `Referrer-Policy: no-referrer`, a strict CSP).

ADR-001 flagged the remaining gap explicitly: *"the engine currently trusts
loopback callers"* and phase-2 UI bridge work (POSTs through `/engine/*`)
*"needs a deliberate CSRF/auth story in the engine first."* The same-origin
check stops a **browser** from forging a mutation, but it does NOT stop a
**different local process** from POSTing to `127.0.0.1:8696` — a non-browser
HTTP client sends no `Origin`, and `isSameOrigin` treats a missing `Origin` as
allowed (deliberately, so the CLI/curl work). Any process running as the same
user can apply tweaks, install apps, or run maintenance by hitting the
loopback API. Because some of those mutations prompt for elevation or touch
HKLM, that is a real local-privilege-surface worth closing before the Next UI
gains write access.

## Options considered

**(a) Do nothing.** Rely on loopback + same-origin. Rejected: phase-2 POSTs
widen the trusted-caller assumption from the embedded dashboard to a browser
tab that also loads other sites; defense in depth is warranted.

**(b) Per-request shared secret passed as a header, generated at startup and
printed / written to a user-readable file.** The UI reads it and echoes it on
every mutation. Simple, but a static secret for the process lifetime is
replayable by any local process that can read the same file or scrape the
console, and rotating it is awkward.

**(c) Loopback session-token handshake (chosen).** When `winforge serve`
starts, it generates a high-entropy token. The token is:

- served to the **embedded dashboard** by injecting it into the dashboard
  HTML/JS at load time (same-origin, loopback only — a cross-origin reader
  cannot fetch it because of the same-origin check + CSP), and exposed via a
  new `GET /api/session-token` endpoint that is itself protected by the
  existing same-origin/loopback rules;
- required on every **mutation** in the `X-WinForge-Token` header. A missing
  or mismatched token → `401`.
- generated with `crypto/rand` (32 bytes, base64url), per server instance;
  restarting the engine rotates it, so a stolen token does not outlive the
  process.

This is the standard double-submit/CSRF-token shape adapted for a loopback
control plane. It does NOT attempt user authentication (the process already
runs as the user; elevation is a separate UAC boundary handled by the
engine). It closes the "any local process can POST" gap because a process
that is not same-origin cannot read the token, while the embedded dashboard
and the proxied Next UI (which fetches it server→loopback through the Next
rewrite) can.

**(d) Mutual TLS / bearer tokens from a config file.** Overkill for a
loopback dev/dashboard surface and adds key-management UX burden. Rejected.

## Decision

Option **(c)**, implemented as:

- `Server` gains a per-instance `sessionToken string` (32 random bytes,
  base64url), generated in `New`/`newServer`.
- New route `GET /api/session-token` returns `{ "token": "..." }`. It is a
  read endpoint; it is only reachable from loopback and (when called from a
  browser) same-origin. The embedded dashboard reads it on load.
- All mutations (`isMutation`) additionally require
  `X-WinForge-Token: <token>`; a missing/wrong token returns
  `401 Unauthorized` **before** any body decode or handler runs, after the
  existing loopback and same-origin checks. The error is JSON
  (`{"error":"invalid or missing session token"}`).
- The token is injected into the embedded dashboard by serving a tiny
  generated `web/session-token.js` (virtual file) that sets
  `window.WINFORGE_SESSION_TOKEN`; `web/index.html` already loads `app.js`,
  and we add the script before it. A non-browser client (curl) first calls
  `GET /api/session-token` and reuses the value.
- The Next.js bridge (`next.config.ts` `/engine/:path*` proxy) already
  forwards all headers, so the UI can fetch the token from
  `/engine/api/session-token` and send `X-WinForge-Token` on its POSTs. No
  CORS change: browser → Next origin, Next server → loopback.

Backward compatibility: this is a **breaking** change for any non-browser
caller that POSTed without a token (there are none shipped; the CLI invokes
the engine in-process, not over HTTP). The phase-2 UI work in W3 is the first
external mutation client and will use the handshake. Read endpoints
(`GET /api/*`) remain token-free so the status/health card and public dashboard
load work unchanged.

## Consequences

- Local processes can no longer mutate system state through the loopback API
  without first reading the token, which requires same-origin access to the
  engine (browsers enforce this; native processes would have to scrape the
  dashboard or the in-memory token, a substantially higher bar).
- The embedded dashboard must fetch/include the token before any POST. The
  engine's own `web/app.js` is updated to read `window.WINFORGE_SESSION_TOKEN`
  (or fetch `/api/session-token`) and attach the header.
- Elevation is unaffected: the token gates the HTTP surface only; the UAC /
  "elevated ignores plugins and user-writable overrides" boundary in
  `internal/app` is unchanged.
- The token is not a secret against an attacker who already controls the user
  account (they can read process memory); it is a CSRF / ambient-authority
  boundary, not a substitute for OS security. This is documented in the
  response and in the dashboard.
- Tests cover: token required on mutations, token accepted, wrong token
  rejected, read endpoints do not require it, `GET /api/session-token` works
  loopback, the injected virtual script serves the token, and cross-origin
  still cannot reach it (existing same-origin tests continue to hold).
