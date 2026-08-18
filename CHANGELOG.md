# Changelog

All notable changes to this project are documented here. The format is based
on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Security

- Escaped dynamic values in HTML audit reports, added restrictive report CSP
  headers, and neutralized spreadsheet formulas in CSV history exports.
- Updated Next.js to 16.3.1, PostCSS to 8.5.26, and the matching Next.js ESLint
  configuration, clearing all known production dependency advisories.
- Added a token-bucket rate limiter (5 req/s sustained, burst 15) on all
  mutating HTTP API endpoints, returning 429 with `Retry-After` when
  exceeded. This is defense-in-depth behind the existing loopback bind,
  per-session token, and same-origin checks.

### Added

- MIT License.
- `CONTRIBUTING.md`, `SECURITY.md`, `QUICKSTART.md`, `.env.example`.
- GitHub issue templates (bug report, feature request) and a pull request
  template.
- Vitest test suite for the web app — 11 tests covering the health-scoring
  algorithm (`src/lib/health.ts`) with the database mocked.
- Tests for previously zero-test packages `internal/platform` (3 tests),
  `internal/scheduler` (8 tests), and `internal/winapi` (9 tests).
- Cross-platform `validateSystemFileName` extracted from the Windows-only
  winapi code so path-traversal / ADS / NUL rejection is tested on Linux CI.
- Cross-platform `validateRegister` extracted from the Windows scheduler
  code so absolute-path and unsafe-character checks run in CI.
- Graceful SIGINT/SIGTERM shutdown for `winforge serve` with a 10-second
  drain timeout.
- Non-blocking toast notifications in the embedded dashboard, replacing
  all `window.alert()` calls.
- Build version stamping via `-ldflags "-X winforge/internal/app.Version=..."`
  in the Makefile and CI.

### Fixed

- Windows CI no longer executes the non-Windows scheduler-stub assertion.
- Installer validation now works from a clean checkout without writing the
  ignored `dist/winforge.iss` artifact.
- `make verify` now uses the `go` binary from `PATH` by default instead of a
  sandbox-specific `/tmp` bootstrap path.
- **Next.js production build no longer requires `DATABASE_URL`** — the
  PostgreSQL pool and Drizzle instance are now constructed lazily, so
  `npm run build` succeeds in environments without a live database.
- **`go test ./...` no longer scans `node_modules/`** — all Go commands in
  the Makefile and CI are scoped to `./cmd/... ./internal/... .` (the
  `flatted` npm package ships `.go` files that `./...` would compile).
- The Go config loader now accepts `"expert"` as a risk level and
  normalizes it to `high`, matching the web catalog's four-tier enum.
- `package.json` name corrected from `nextjs-postgresql-template` to
  `winforge-web`; added version, description, license, and test scripts.
- Pre-existing trailing whitespace in `AUDIT_REPORT.md` removed.

### Changed

- README rewritten to lead with the Go engine as the product, with
  accurate catalog counts, build/verify instructions, and a user-facing
  PowerShell quick start.
- The intentionally different health algorithms (Go engine vs. web app)
  are now documented in both implementations with a note not to align
  them without updating both together.
- `AGENTS.md` updated to reflect the `archive/` layout, scoped Go
  package paths, and the expanded verification battery.

### Removed

- `tsconfig.tsbuildinfo` is no longer tracked in git (added to
  `.gitignore` along with `.next/`, `next-env.d.ts`, and Go build
  artifacts).
- Archived dormant reference code under `archive/`: the WPF/.NET Phase 1
  scaffold (`WinForge.Elite/`), the abandoned Bun/Zig core
  (`runtime/`, `native/core.zig`, `native/build.sh`), and the stale CI
  staging copy (`ci/`). `native/build-lua.sh` is retained — it builds
  `lua54.dll` for the Lua plugin tier.

### Deprecated

- `ci.yml.fixed` at the repo root is ready to be promoted to
  `.github/workflows/ci.yml` once the GitHub App is granted the
  `workflows` permission (BLK-3). Until then it is the canonical CI
  definition and is mirrored by `make verify`.

### Documentation

- `docs/TROUBLESHOOTING.md` covering SmartScreen, elevation, missing DLLs,
  restore points, dashboard connection, session tokens, 429s, and the
  DATABASE_URL build requirement.
- `docs/ADR-003-wasm-lua-only.md` recording the decision to ship WASM
  validation only and defer the Windows C host.
- GitHub issue templates (bug report, feature request) and a pull request
  template with the full verification checklist.
- Added a dated addendum to `AUDIT_REPORT.md` flagging resolved findings
  and noting that the competitive-analysis numbers are unverified.

### Tooling

- `tools/verify_binary.py`: static PE32+ verifier (architecture, embedded
  assets, stamped version) wired into the Makefile and CI. Replaces the
  two-byte MZ header check.
