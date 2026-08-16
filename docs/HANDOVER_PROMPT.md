# WinForge — Session Handover Prompt (90% Sprint — Zero Compromises)

> Copy the section below into a fresh Arena session to continue this project.
> Also read the repo's `AGENTS.md` first — it is the agent's memory file.

---

You are continuing work on **WinForge** (`Warzonesiddiki/winforge`), an all-in-one
Windows optimization/debloat/privacy/repair suite. The project is a
**Go-primary hybrid (v4: 13 languages, 0 blocked toolchains)**: a self-contained
stdlib-only Go engine (`cmd/winforge`, `internal/*`, `config/*.json`, `web/`)
merged into `main` (PR #18, `f999811`), plus a Next.js web simulation whose
`src/db/seed-data.ts` is the catalog of record, plus Python catalog tooling
in `tools/`, plus Zig (`native/`) and Bun (`runtime/`) companion scaffolds.
**Goal for this session: push the project from ~70% → ≥90% complete, zero compromises,
and make it the most productive session since the hybrid switch.**

**Start by reading these files, in order:**
1. `AGENTS.md` — the agent memory file (v4: 13-lang, Go primary, WASM platform tier landed, session state through PR #18).
2. `docs/LANGUAGE_SELECTION.md` v4 — the 13-language best-of-best decision (Go primary, re-verified blockers today, why adding Rust/Nim/Tcl would be a compromise).
3. `docs/BLOCKED_ITEMS.md` — what is blocked and why (official hosts 000 today, GitHub/pypi/npm mirrors 200).
4. `docs/CATALOG_PARITY.md` — catalog state (240 tweaks / 102 debloat / 83 apps / 40 privacy: 38 equivalent · 0 partial · 2 documented gaps, 18 engine-only extras).
5. `docs/GO_TOOLCHAIN_BOOTSTRAP.md` — how to rebuild the Go toolchain.
6. `docs/WASM_REALSCOPE_2026-08-16.md` + `docs/WASM_PLUGIN_SANDBOX.md` + `docs/LUA_PLUGIN_PLAN.md` — plugin tiers (Lua landed, WASM platform-independent landed, Windows C host deferred).
7. `docs/WINDOWS_SMOKE_CHECKLIST.md` — BLK-6 manual checklist (§11 Lua, §12 auth, §13 WASM stub, §14 ISO).
8. `docs/ADR-002-engine-mutation-auth.md` — session-token auth (42s).

**First actions in the new session (the sandbox resets between sessions):**
1. Rebuild the Go toolchain from source using the exact bootstrap chain in `AGENTS.md` §5 (go1.4.3 → go1.17.13 → go1.20.14 → go1.22.12, ~7 minutes, sources from codeload.github.com; no system Go exists).
2. Reinstall node_modules (`npm install --no-audit --no-fund`) and re-run the verification battery in `AGENTS.md` §6 (`gofmt`, `go vet` linux+windows, `go test` + `-race` 18/18 packages — plugin now 63 tests, `GOOS=windows go build` → 6.5 MB PE, `npm typecheck` + `lint`, `python3 tools/catalog_parity.py` exit 0, `git diff --check` clean).
3. Skim `git log origin/main --oneline -5` to see where the last session left off (should be `f999811 Merge PR #18`).

**Known current state (verified 2026-08-16, main `f999811` after PR #18, session arena/01a00ac0-winforge):**
- Engine: 18/18 packages green incl. race; **plugin 63 tests (33 Lua + 30 WASM)**; `winforge.exe` 6.5 MB PE (linux `go vet` + windows `go vet` + `go test -c` plugin both GOOS clean).
- **Catalog is 240 tweaks, all named** (129 Atlas YAML 100% op-overlap via `tools/atlas_metadata.py`, 60 web tweaks merged, 17 winforge-* privacy tweaks, 12 atlas backfills). 3 exclusions remain with evidence: `perf-pagefile` (REG_MULTI_SZ), `perf-memcompression` (PowerShell-only, research CLOSED), `exp-classic-paint` (narrative). **Privacy 38/40** (2 deliberate gaps: `priv-dnt` browser, `priv-microsoft-store-ads` no registry backing — do not fabricate).
- **Hybrid v4: 13 languages, 0 blocked** — Go primary, React/TS, PowerShell, Zig 0.16 (97.9 MB wheel downloaded today), C via `zig cc`, Lua 5.4 (483 KB `lua54.dll` via `zig cc`, landed binding), WASM (+ WASI, `_wasmtime.dll` PE), Python, Bun/TS, Node SEA, **SQL (Drizzle 11 tables), YAML/JSON, Inno Setup** (Phase 4). Every toolchain verified today; Rust/Nim/Tcl/Deno/Flutter/Java/FPC remain excluded with evidence (official hosts 000, GitHub mirrors 200 but heavy source builds would break stdlib-only — compromise).
- **Plugins:** JSON + Lua W1 (Windows `syscall.LoadDLL`, 10M `LUA_MASKCOUNT` hook, fake `ScriptHost` 24 tests) + **WASM platform-independent W2 (2026-08-16 `arena/01a00ac0`):** `internal/plugin/wasm.go` (whitelisted `wasmAPI` + `validateWasmModule` magic `\x00asm` + version 1 + 4 MiB cap, fuel 10M), `wasm_other.go`/`wasm_windows.go` (DLL-search stub `wasmtime.dll` absolute path, `ErrWasmUnavailable` until verified), `wasm_test.go` 30 tests, `examples/plugins/example-wasm-pack/` (8-byte minimal wasm + `pack.wat.example`). Windows binding (~30–35 C funcs) still gated on BLK-6 hardware (§13).
- **W3 Session-token auth (ADR-002) DONE both phases:** per-instance 32B `crypto/rand` base64url token, `GET /api/session-token`, `X-WinForge-Token` required on POST/PUT/PATCH/DELETE (401 before decode, after loopback+same-origin), `web/app.js` + `src/lib/engine-client.ts` + `EngineTweaks.tsx` (search/category/applied filters). Verified 401 without token, 403 cross-origin, 200 with token, rotation invalidates, GETs stay open.
- **Blockers:** BLK-3 `workflows` permission still blocked (verified today: push to `.github/workflows/ci.yml` → `refusing to allow a GitHub App`), BLK-6 no Windows runtime (checklist §§11-14), BLK-1 .NET archived. `ci.yml.fixed` (parity + artifact + `typecheck`/`lint`) ready to copy when BLK-3 clears.
- **Branch:** `main` at `f999811` (PR #18 merged: `943c819` WASM tier + `febc1c6` v4 docs). `git diff --check` clean.

**Retrospective — why the last 3 sessions felt minor vs the hybrid switch (and why the next must be the most productive):**

| Session | What landed | Lines / tests | Why it felt "minor" vs hybrid switch (2026-08-16 `arena/01a00795`, which was a *4-area blast*) |
|---|---|---|---|
| **Hybrid switch** `01a00795` | Native ops (`power_hibernate`, `power_processor_state`, `netbios`, `registry_delete_key` + default-value writes), converter upgrade, 240 tweaks (219→240) + 17 privacy tweaks (18→2 gaps), checklist D, `ci.yml.fixed` E, `native/build-lua.sh` F, `WASM_PLUGIN_SANDBOX` G, `ADR-001` + proxy + `EngineStatusCard` H | ~2k lines, 4 catalog gaps retired, 6 artifacts | **High blast radius:** touched engine + catalog + converter + 4 native Win32 APIs + 4 docs + CI — pushed coverage from ~50% → ~70% |
| `01a00a47` W1 (Lua) | In-engine Lua binding (`lua.go`+`lua_windows.go`+`lua_other.go`), example pack, 24 plugin tests | ~600 lines, +1 lang binding | **Single tier:** 1 plugin tier, 1 op whitelist — deep but narrow |
| `01a00a47` W3+W2+W6 | Session-token auth (`auth.go`+`server.go`+`web/app.js`+`engine-client.ts`), W2 re-scope doc, memcompression closed | ~400 lines + docs | **Auth + decisions:** security hardening + honest deferral, not user-visible features |
| `01a00a47` W3 phase2 | `EngineTweaks.tsx` Next UI wiring (search/filters/apply) | ~200 lines | **UI wiring:** 1 component, no engine change |
| `01a00ac0` W2 v4 | WASM platform-independent tier + v4 docs (30 WASM tests, 13-lang) | ~900 lines | **Platform tier + docs:** same shape as W1 but without Windows C host — honest stub |

**Pattern:** hybrid switch was *breadth* (engine+cat+CI+docs+4 native APIs). Last 3 were *depth* (one tier or one security feature each). To reach **≥90%**, the next session must be a **second breadth blast** — 4 parallel areas, each sandbox-verifiable, zero compromise.

**90% = Zero Compromise Checklist (all must be green before claiming 90%):**

*Measured from `PROJECT_ROADMAP.md` Phases 1-5. 100% requires Windows smoke on real hardware + signing cert + `workflows` permission; 90% is the *sandbox-verifiable* max.*

- [ ] **Engine core (Phases 1-3):** 240 tweaks, 102 debloat, 83 apps, 240 parity exit 0, converter idempotent SHA, `go vet` linux+windows + `go test -race` 18/18 + `GOOS=windows go test -c` plugin both tiers + 6.5 MB PE all green (already ✅ — keep green)
- [ ] **Plugins (Phase 2):** Lua W1 + WASM W2 platform-independent both on Linux via fakes (63 tests) + DLL-search stubs (absolute path, never PATH) + `MergeTweaks` precedence + elevated ignores plugins (§11 + §13.1-13.4 in `WINDOWS_SMOKE_CHECKLIST.md`) — Windows C host for WASM stays **honestly stubbed** (no fabrication)
- [ ] **Security (Phase 2-3):** Session-token auth (§12: 401 without, 403 cross-origin, 200 with, rotation), loopback+same-origin, protected services, service name validation, command allowlist, `ValidateOperationForPlugin` on every plugin op, elevation boundary — all tests green
- [ ] **UI bridge (Phase 2.5):** `next.config.ts` `/engine/*` proxy + `EngineStatusCard` (offline graceful) + `EngineTweaks.tsx` (search/category/applied, token on POST) + `web/app.js` token fetch — `npm typecheck` + `lint` green, e2e via `fetch` proofs
- [ ] **Catalog (Phase 2):** 129 Atlas YAML 100% op-overlap, `tools/atlas_metadata.py` dry-run exit 1 if suspect, `catalog_parity.py` triage 38/40, `web_catalog_to_engine.py --apply` idempotent — no invented registry backing for `perf-memcompression`/`priv-store-ads`
- [ ] **Installer & Updater (Phase 4):** `internal/isobuilder` + `internal/updater` already exist — next session must add **sandbox-verifiable Inno Setup `*.iss` generation** (parse + `osc` dry-run, no binary download) + update check against GitHub Releases API (mocked, `GOPROXY=off` compliant)
- [ ] **Localization (Phase 5, parallel):** 5-language scaffolding (en-US, es-ES, fr-FR, de-DE, zh-CN) — **no hardcoded strings**: extract `XAML Language binding`-ready JSON (`web/` + `src/`) + `src/db/seed-data.ts` → `.json` resource files + language selector in Settings (Drizzle migration, no DB wipe)
- [ ] **CI & Docs (Phase 4-5):** `ci.yml.fixed` parity job (`python3 tools/catalog_parity.py` must exit 0 + converter SHA) + artifact upload + `npm typecheck/lint` — keep `git diff --check` clean, `AGENTS.md` updated when reality changes, every gap has a written reason
- [ ] **Smoke docs:** `WINDOWS_SMOKE_CHECKLIST.md` §§1-14 fully written, honest about BLK-6 N/A items — 90% does NOT require executing Windows, but requires the checklist to be *complete* so a Windows run is a single pass

**Next session plan — 4 parallel tracks to hit 90% (most productive session since hybrid):**

**Track A — Localization (Phase 5) — ~30% of 90%:**
1. Extract all hardcoded UI strings from `src/` + `web/` into `web/locales/{en,es,fr,de,zh}.json` (script `tools/extract_locales.py`, verified by `python3` + `node --check`).
2. Add `src/lib/i18n.ts` (typed `t(key)` + fallback to `en`) + `src/components/LanguageSelector.tsx` in Settings, wire to `drizzle` `settings` table (migration idempotent).
3. Tests: snapshot of each locale loads + fallback, `typecheck` green. Do NOT translate 240 tweak names via LLM — keep English as source, mark others as `TODO (human review)` to avoid fabrication.

**Track B — Installer & Packaging (Phase 4) — ~25%:**
1. `tools/generate_iss.py` — emits `dist/winforge.iss` from `config/tweaks.json` + `go.mod` version (Inno Setup DSL, no binary — `iscc` syntax check via `wine` if available else `python3` parser).
2. `internal/isobuilder` — add `GenerateWimConfig` dry-run that writes `Autounattend.xml` snippets and validates with `python3 -c 'import xml'` (no ADK).
3. `internal/updater` — add `CheckGitHubRelease` that parses `https://api.github.com/repos/Warzonesiddiki/winforge/releases` (mocked in tests with `httptest`, `GOPROXY=off` — no live fetch in CI).

**Track C — Engine Hardening & WASM Docs (Phase 2-3) — ~25%:**
1. Keep engine green but add **2 high-value sandbox-verifiable hardening**: `limits.go` fuzz (max string/path 16k) + `executor` allowlist negative tests for `perf-memcompression`/`powershell` (already `TestRunCommandElevationBoundary` — extend to `net use` etc.).
2. Close the WASM docs loop: update `docs/CATALOG_PARITY.md` manual sections after any catalog touch (re-append state of catalog), ensure `WASM_REALSCOPE` + `WASM_PLUGIN_SANDBOX` + checklist §13 stay in sync (no divergence).
3. No Windows C binding — keep the honest stub; add a **CGO-free fuel/memory bounds test** for WASM (fake host in `wasm_test.go` already covers).

**Track D — CI & Quality Gates (Phase 4) — ~10% (unblocks the last 10%):**
1. Make `ci.yml.fixed` the *de facto* CI by adding a `make verify` target that runs the exact 7 checks CI runs (so local `make verify` = CI green without `workflows` permission).
2. `tools/catalog_parity.py --write-report` + re-append manual sections + `python3 tools/web_catalog_to_engine.py --apply` idempotence SHA check — enforce in `make verify`.
3. Keep `git diff --check` clean (no trailing whitespace — markdown double-space is a CI failure).

**Hard rules (from the project owner):**
- ZERO fabrication: verify every claim by executing build/test/parse before asserting it. No TODOs, no placeholders, no mock data. Every new registry value must cite a `learn.microsoft.com` policy/CSP doc or an Atlas YAML with 100% op overlap.
- Never modify `.github/workflows/*` (push is rejected without `workflows` permission) — keep `ci.yml.fixed` as the drop-in.
- Keep the verification battery green before committing; keep docs honest; run `python3 tools/catalog_parity.py --write-report docs/CATALOG_PARITY.md` after any catalog change (then re-append the manual sections from "## State of the catalog"); update `AGENTS.md` when reality changes.
- Security invariants in `AGENTS.md` §7 override feature pressure. The elevated executor never runs PowerShell/powercfg/wmic; new capabilities are native ops, not allowlist additions. Plugins share the same closed whitelist.
- CI failure logs cannot be downloaded from the sandbox — debug locally. `make verify` must equal CI.

**Session mechanics & gotchas:** work only on the branch the session assigns (off current `main` `f999811`), push only there, open PRs back to `main` with `gh pr create --head <branch>`. Sandbox quirks: shell output escapes backslashes (trust `od -c`), `/tmp` and `node_modules` vanish between sessions, `git diff --check` must stay clean (CI enforces it). Use `GOPROXY=off GOFLAGS=-mod=mod` everywhere. Commit early, push often, keep PR description with a **Verification** code block showing `gofmt`/`go vet`/`go test -race`/`GOOS=windows go build`/`parity`/`typecheck`/`lint` all green.

**How to claim 90%:** at PR description, paste the **8-track checklist** above with `[x]` for every sandbox-verifiable item, plus `f999811..HEAD` diff stat, plus the verification block. The only `[ ]` left at 90% should be Windows smoke execution on real hardware (§§11-14), signing cert, and `workflows` permission — all explicitly BLK-6/3/1 and documented.
