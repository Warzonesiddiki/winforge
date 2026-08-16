# WinForge — Session Handover Prompt (90% → 100% Final Sprint — Zero Compromises)
> Copy the section below into a fresh Arena session to continue this project.
> Also read the repo's `AGENTS.md` first — it is the agent's memory file.
---
You are continuing work on **WinForge** (`Warzonesiddiki/winforge`), an all-in-one
Windows optimization/debloat/privacy/repair suite. The project is a
**Go-primary hybrid (v4: 13 languages, 0 blocked toolchains)**: a self-contained
stdlib-only Go engine (`cmd/winforge`, `internal/*`, `config/*.json`, `web/`)
merged into `main` (PR #19, `3df143e`), plus a Next.js web simulation whose
`src/db/seed-data.ts` is the catalog of record, plus Python catalog tooling
in `tools/`, plus Zig (`native/`) and Bun (`runtime/`) companion scaffolds.
**Goal for this session: push the project from 90% sandbox-verifiable → 100% complete,
zero compromises. The sandbox can only prove 90%; the final 10% requires a real
Windows 10 22H2/11 machine + EV cert + `workflows` permission. Make this the
session that closes the project or explicitly documents why it cannot be closed.**

**Start by reading these files, in order:**
1. `AGENTS.md` — the agent memory file (v4: 13-lang, Go primary, WASM platform tier landed, 90% breadth blast through PR #19).
2. `docs/LANGUAGE_SELECTION.md` v4 — the 13-language best-of-best decision (Go primary, re-verified blockers today, why adding Rust/Nim/Tcl would be a compromise).
3. `docs/BLOCKED_ITEMS.md` — what is blocked and why (official hosts 000 today, GitHub/pypi/npm mirrors 200).
4. `docs/CATALOG_PARITY.md` — catalog state (240 tweaks / 102 debloat / 83 apps / 40 privacy: 38 equivalent · 0 partial · 2 documented gaps, 18 engine-only extras).
5. `docs/GO_TOOLCHAIN_BOOTSTRAP.md` — how to rebuild the Go toolchain.
6. `docs/WASM_REALSCOPE_2026-08-16.md` + `docs/WASM_PLUGIN_SANDBOX.md` + `docs/LUA_PLUGIN_PLAN.md` — plugin tiers (Lua landed, WASM platform-independent landed, Windows C host deferred).
7. `docs/WINDOWS_SMOKE_CHECKLIST.md` — BLK-6 manual checklist (§11 Lua, §12 auth, §13 WASM stub, §14 ISO, §15 new isobuilder/updater/locale smoke).
8. `docs/ADR-002-engine-mutation-auth.md` — session-token auth (42s).
9. `Makefile` — `make verify` = `ci.yml.fixed` (the de facto CI since BLK-3).

**First actions in the new session (the sandbox resets between sessions):**
1. Rebuild the Go toolchain from source using the exact bootstrap chain in `AGENTS.md` §5 (go1.4.3 → go1.17.13 → go1.20.14 → go1.22.12, ~7 minutes, sources from codeload.github.com; no system Go exists).
2. Reinstall node_modules (`npm install --no-audit --no-fund`) and re-run the verification battery (`make verify` or `AGENTS.md` §6: `gofmt`, `go vet` linux+windows, `go test` + `-race` 18/18 packages — isobuilder 15 tests + updater 14 tests + config fuzz + engine hardening, `GOOS=windows go build` → 6.5 MB PE, `npm typecheck` + `lint`, `python3 tools/catalog_parity.py` exit 0, `python3 tools/extract_locales.py` PASSED, `python3 tools/generate_iss.py --check` PASSED, `git diff --check` clean).
3. Skim `git log origin/main --oneline -5` to see where the last session left off (should be `3df143e Merge PR #19`).

**Known current state (verified 2026-08-16, main `3df143e` after PR #19, session `arena/01a00ada-winforge`):**
- Engine: 18/18 packages green incl. race; **isobuilder 15 tests (6 wimconfig + 9 existing), updater 14 tests (9 github + 5 existing), config fuzz + engine hardening**; `winforge.exe` 6.5 MB PE (linux `go vet` + windows `go vet` + `go test -c` plugin both GOOS clean).
- **Catalog is 240 tweaks, all named** (129 Atlas YAML 100% op-overlap, 60 web merges, 17 winforge-* privacy, 12 backfills). 3 exclusions remain with evidence: `perf-pagefile` (REG_MULTI_SZ), `perf-memcompression` (PowerShell-only, research CLOSED), `exp-classic-paint` (narrative). **Privacy 38/40** (2 deliberate gaps: `priv-dnt` browser, `priv-microsoft-store-ads` no registry backing — do not fabricate).
- **Hybrid v4: 13 languages, 0 blocked** — Go primary, React/TS, PowerShell, Zig 0.16 (97.9 MB wheel), C via `zig cc`, Lua 5.4 (483 KB `lua54.dll`), WASM (+ WASI, `_wasmtime.dll` PE), Python, Bun/TS, Node SEA, **SQL (Drizzle 11 tables + `drizzle/0001_language_settings.sql` idempotent)**, **YAML/JSON**, **Inno Setup** (Phase 4 dry-run). Every toolchain verified; Rust/Nim/Tcl/Deno/Flutter/Java/FPC remain excluded with evidence (official hosts 000, GitHub mirrors 200 but heavy source builds would break stdlib-only).
- **Plugins:** JSON + Lua W1 (Windows `syscall.LoadDLL`, 10M `LUA_MASKCOUNT`) + **WASM platform-independent W2**: `internal/plugin/wasm.go` (whitelisted `wasmAPI` + `validateWasmModule` magic `\x00asm` + version 1 + 4 MiB cap, fuel 10M), `wasm_windows.go` stub `wasmtime.dll` absolute path `ErrWasmUnavailable` until verified, 30 WASM tests + 33 Lua = 63 plugin tests + new hardening. Windows C host (~30–35 funcs) still gated on BLK-6 (§13).
- **W3 Session-token auth (ADR-002) DONE both phases:** per-instance 32B `crypto/rand` base64url, `GET /api/session-token`, `X-WinForge-Token` on POST/PUT/PATCH/DELETE (401 before decode, after loopback+same-origin), `web/app.js` + `src/lib/engine-client.ts` + `EngineTweaks.tsx` (search/category/applied). Verified 401 without token, 403 cross-origin, 200 with, rotation invalidates, GETs open.
- **90% sprint PR #19 breadth blast (4 tracks, 2344 lines):**
  - **A Localization:** `web/locales/{en,es,fr,de,zh}.json` + `en-US` aliases (56 keys each, sorted, `tools/extract_locales.py` sync), `src/lib/i18n.ts` 28→56 keys (typed `t(key)`), `src/components/LanguageSelector.tsx` (reusable), `drizzle/0001_language_settings.sql` idempotent.
  - **B Installer:** `tools/generate_iss.py` → `dist/winforge.iss` (Inno DSL, 240/83/102, Python parser validates sections/AppVersion, iscc via wine if available), `internal/isobuilder/wimconfig.go` (`GenerateUnattendXML`/`ValidateUnattendXML`/`WriteUnattendFile`/`GenerateWimConfig`, Go `encoding/xml` + Python `xml.etree` no ADK), `internal/updater/github.go` (`CheckGitHubRelease` + `CheckForUpdate`, httptest mocked, 1 MiB cap, `GOPROXY=off`).
  - **C Hardening:** `internal/config/limits_fuzz_test.go` (16k fuzz ±10 + multi-byte rune), `internal/engine/executor_hardening_test.go` (30+ extra negatives: `net use`, `sc`, `reg`, `wmic`, `powercfg`, `schtasks` + PowerShell aliases), isobuilder/updater 18 new tests.
  - **D CI:** `Makefile` (`SHELL=bash`, `make verify` = 7 CI checks: gofmt/vet/test/race/vet-windows/build-windows PE/parity+converter SHA+locales+ISS+Autounattend/python xml/web typecheck+lint+node --check/JSON+JS syntax+git diff --check), fixes `HANDOVER_PROMPT.md` blank line.
- **Blockers:** BLK-3 `workflows` permission still blocked (push to `.github/workflows/ci.yml` → `refusing to allow a GitHub App`), BLK-6 no Windows runtime (checklist §§11-15), BLK-1 .NET archived. `ci.yml.fixed` (parity + artifact + typecheck/lint) ready to copy when BLK-3 clears. `make verify` is de facto CI.
- **Branch:** `main` at `3df143e` (PR #19 merged: `9f95da3` 90% breadth blast). `git diff --check` clean, `make verify` ALL GREEN.

**Retrospective — why the 90% session was a breadth blast (and why 100% needs a Windows machine):**
| Session | What landed | Lines / tests | Why it matters vs 100% gap |
|---|---|---|---|
| **Hybrid switch** `01a00795` | Native ops (4 Win32 APIs), converter, 240 tweaks (219→240), checklist D, `ci.yml.fixed` E, `build-lua.sh` F, `WASM_PLUGIN_SANDBOX` G, `ADR-001` + proxy H | ~2k lines, 4 gaps retired | Pushed 50%→70% via engine+cat+CI+docs |
| `01a00a47` W1 Lua | `lua.go`+`lua_windows.go`+`lua_other.go`, 24 tests | ~600 lines | Single tier, deep but narrow |
| `01a00ac0` W2 WASM platform | `wasm.go`+`wasm_other/windows.go`+30 tests, v4 docs | ~900 lines | Platform tier, honest stub (no unverified C host) |
| **90% blast** `01a00ada` | **4 tracks:** `web/locales` 10 JSON + `extract_locales.py`, `generate_iss.py`, `wimconfig.go`+`github.go`, `limits_fuzz`+`executor_hardening`, `Makefile` make verify | **2344 lines, 18 new tests** | **Sandbox-verifiable max 90%:** every track was Go/Python/TS that runs on Linux — no Windows exec, no `workflows` permission needed. The last 10% *cannot* be sandbox-verified; it is `wasmtime` C host (~35 funcs) + `lua54.dll` load + `iscc` + EV cert + real `winforge serve` on Win10/11 |

**Pattern:** 70%→90% was *second breadth* (A+B+C+D parallel). 90%→100% must be *depth on real hardware* — a single Windows run that proves what Linux fake hosts only model.

**100% = Zero Compromise Checklist (all must be green before claiming 100%):**
*Measured from `PROJECT_ROADMAP.md` Phases 1-5. 90% = sandbox max; 100% = Windows smoke PASS + signing cert + workflows permission.*

- [ ] **Engine core (Phases 1-3):** 240 tweaks, 102 debloat, 83 apps, parity exit 0, converter SHA, `go vet` linux+windows + `go test -race` 18/18 + `GOOS=windows go test -c` plugin both tiers + 6.5 MB PE all green (already ✅ — keep green)
- [ ] **Plugins (Phase 2):** Lua W1 + WASM W2 **Windows C host verified** (or explicit *Lua-only* decision with evidence): `lua54.dll` absolute-path load (§11.1-11.9 PASS inc. 11.8 longjmp stability), `wasmtime.dll` absolute-path + fuel 10M + linear-memory bounds (§13.1-13.4 PASS) OR written ADR closing W2 as Lua-only — no fabricated C binding
- [ ] **Security (Phase 2-3):** Session-token auth (§12.1-12.9 PASS on `http://localhost:8696`: 401 without, 403 cross-origin, 200 with, rotation invalidates `EngineTweaks.tsx`), loopback+same-origin, protected services, service name validation, command allowlist, `ValidateOperationForPlugin` on every plugin op, elevation boundary — all tests + smoke green
- [ ] **UI bridge (Phase 2.5):** `/engine/*` proxy + `EngineStatusCard` + `EngineTweaks.tsx` (search/category/applied, token on POST) + `web/app.js` token fetch — `npm typecheck` + `lint` green, e2e smoke §12.6-12.7 PASS
- [ ] **Catalog (Phase 2):** 129 Atlas YAML 100% overlap, `catalog_parity.py` 38/40, `web_catalog_to_engine.py --apply` idempotent — no invented backing
- [ ] **Installer & Updater (Phase 4):** `internal/isobuilder` + `internal/updater` + **`tools/generate_iss.py` → `iscc` PASS via `wine` on real Windows** + `GenerateWimConfig` writes `Autounattend.xml` (python `xml.etree` PASS) + `CheckGitHubRelease` against live GitHub API (`GOPROXY=off`); **EV cert signs `winforge.exe` → SmartScreen Verified Publisher** (§11 signing)
- [ ] **Localization (Phase 5, parallel):** 5-language scaffolding (56 keys each) + `LanguageSelector.tsx` + Drizzle migration idempotent — **human-reviewed translations** for tweak names if required (mark `TODO` if still TODO — do not LLM-fabricate 240 names), `npm typecheck` green, snapshot test per locale
- [ ] **CI & Docs (Phase 4-5):** `ci.yml.fixed` copied to `.github/workflows/ci.yml` after BLK-3 clears (parity + artifact + typecheck/lint) — `make verify` == CI, `git diff --check` clean, `AGENTS.md` updated, every gap has written reason
- [ ] **Smoke docs:** `WINDOWS_SMOKE_CHECKLIST.md` §§1-15 fully written **and EXECUTED** (PASS/FAIL per item, console logs, SHA-256) — the checklist itself is 90%, the *execution* is 100%

**Next session plan — 3 parallel tracks to hit 100% (final mile, needs Windows):**
**Track A — Windows Smoke on Real Hardware (BLK-6, Phase 1-3 + §11-15) — 50% of 100%:**
1. Provision a **disposable Win10 22H2 or Win11 VM** (snapshot BEFORE any `winforge` run). Build `winforge.exe` on that VM via `go build` (or copy 6.5 MB PE from CI artifact) + place `lua54.dll` (from `native/build-lua.sh`) and optionally `_wasmtime.dll` (from `wasmtime 47` wheel) next to exe.
2. Execute `docs/WINDOWS_SMOKE_CHECKLIST.md` §§1-15 in order, recording PASS/FAIL + `> logs\NN.txt 2>&1` per item: §1 basics (240 tweaks list/scan), §2 elevation (HKLM tweak fails non-elevated, succeeds elevated, elevated ignores plugins), §3 restore point, §4 apply/verify/undo roundtrip (`tel-disable-telemetry`), §5 four native ops (`pwr-hibernation`, `pwr-processor-mgmt`, `net-disable-netbios`, `ui-classic-context` default-value + `registry_delete_key`), §6 privacy spotchecks, §7 guards (protected `WinDefend`, malformed `WinDefend `, `powershell` not allowlisted), §8 dashboard `serve` + §12 auth 401/200/403/rotation, §11 Lua happy/hostile/runaway/bypass (11.8 longjmp stability), §13 WASM no-DLL + bad magic/oversized + elevation, §14 ISO (if media available), §15 **new:** isobuilder `GenerateWimConfig` writes `Autounattend.xml` (validate `python -c 'import xml.etree'` on Windows Python) + `tools/generate_iss.py` → `iscc` compiles `dist/winforge.iss` (need `iscc` via Inno Setup on Windows) + updater `CheckGitHubRelease` hits live API (not mocked).
3. File issues for any FAIL; do not silently patch — this is the verification gate the last 6 sessions deferred.

**Track B — Signing + WASM Host Closure (Phase 4 strong isolation) — 30%:**
1. **Signing:** purchase EV cert (DigiCert/GlobalSign) → `signtool sign /f cert.pfx /p password /tr http://timestamp.digicert.com /td sha256 /fd sha256 winforge.exe` → `signtool verify /pa winforge.exe` + SmartScreen Verified Publisher on test VM.
2. **WASM C host decision:** EITHER (a) implement `wasmtime` C host `wasm_windows.go` (~35 funcs: `wasmtime_config_new`, `wasmtime_context_set_fuel`, `wasmtime_linker_define_func`, `wasmtime_call`, `wasmtime_trap_code`, etc.) with manual handle ownership + `syscall.NewCallback` for host imports (`health_score`, `tweak_is_applied`, `propose_registry_set`, `log`), fuel 10M, bounded linear-memory strings via `limits.go` — then execute §13.4-13.5 and prove out-of-fuel termination OR (b) write `docs/ADR-003-wasm-lua-only.md` closing W2 as Lua-only with evidence (why 30-func unverified binding in a security boundary is a compromise, Lua already covers trusted community packs). Do not ship an unverified binding — that violates the zero-fabrication rule.
3. Update `docs/LANGUAGE_SELECTION.md` if WASM decision changes the 13-lang count (keep 0 compromise narrative) and `docs/WASM_REALSCOPE_2026-08-16.md` with the outcome.

**Track C — CI Workflows Permission + Release (Phase 4, unblocks the last 10%) — 20%:**
1. Repo owner grants GitHub App `workflows: write` (Settings → Permissions → Workflows: Read and write → re-install for `Warzonesiddiki/winforge`).
2. Copy `ci.yml.fixed` → `.github/workflows/ci.yml` (`cp ci.yml.fixed .github/workflows/ci.yml`), push, verify `gh pr checks` shows **7 jobs**: `Test (Linux)`, `Test (Windows)`, `Cross-compile` (3 arch), `Catalog parity` (exit 0 + converter SHA), `Web app checks` (typecheck/lint), `JSON and JavaScript syntax` + artifact `winforge-windows-amd64`.
3. Tag `v0.1.0` (or current `app.Version`), publish GitHub Release (artifacts from CI), invite 20-50 beta testers — `internal/updater` `CheckGitHubRelease` now has a live endpoint to poll.

**Hard rules (from the project owner):**
- ZERO fabrication: verify every claim by executing build/test/parse before asserting it. No TODOs, no placeholders, no mock data. Every new registry value must cite a `learn.microsoft.com` policy/CSP doc or an Atlas YAML with 100% op overlap.
- Never modify `.github/workflows/*` until BLK-3 clears — keep `ci.yml.fixed` as the drop-in; use `make verify` as de facto CI.
- Keep the verification battery green before committing; keep docs honest; run `python3 tools/catalog_parity.py --write-report docs/CATALOG_PARITY.md` after any catalog change (then re-append the manual sections from "## State of the catalog"); update `AGENTS.md` when reality changes.
- Security invariants in `AGENTS.md` §7 override feature pressure. The elevated executor never runs PowerShell/powercfg/wmic; new capabilities are native ops, not allowlist additions. Plugins share the same closed whitelist.
- CI failure logs cannot be downloaded from the sandbox — debug locally. `make verify` must equal CI. WASM binding without Windows execution is a security-boundary bug — do not ship.

**Session mechanics & gotchas:** work only on the branch the session assigns (off current `main` `3df143e`), push only there, open PRs back to `main` with `gh pr create --head <branch>`. Sandbox quirks: shell output escapes backslashes (trust `od -c`), `/tmp` and `node_modules` vanish between sessions, `git diff --check` must stay clean (CI enforces it). Use `GOPROXY=off GOFLAGS=-mod=mod` everywhere. Commit early, push often, keep PR description with a **Verification** code block showing `gofmt`/`go vet`/`go test -race`/`GOOS=windows go build`/`parity`/`typecheck`/`lint` all green + `git log origin/main --oneline -5` + `make verify` ALL GREEN.

**How to claim 100%:** at PR description, paste the **8-track checklist** above with `[x]` for every item, plus `3df143e..HEAD` diff stat, plus the verification block, plus **Windows smoke sign-off table** (build SHA-256, date/operator, 15 sections PASS/FAIL/N/A, logs linked). The only honest way to claim 100% is a Windows VM execution; sandbox-only can stay at 90% and document BLK-6/3/1 explicitly.
