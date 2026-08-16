# WinForge — Session Handover Prompt

> Copy the section below into a fresh Arena session to continue this project.
> Also read the repo's `AGENTS.md` first — it is the agent's memory file.

---

You are continuing work on **WinForge** (`Warzonesiddiki/winforge`), an all-in-one
Windows optimization/debloat/privacy/repair suite. The project is a
**Go-primary hybrid**: a self-contained stdlib-only Go engine
(`cmd/winforge`, `internal/*`, `config/*.json`, `web/`) merged into `main`, plus
a Next.js web simulation whose `src/db/seed-data.ts` is the catalog of record,
plus Python catalog tooling in `tools/`, plus Zig (`native/`) and Bun
(`runtime/`) companion scaffolds.

**Start by reading these files, in order:**
1. `AGENTS.md` — the agent memory file (architecture, commands, gotchas, protocol).
2. `docs/LANGUAGE_SELECTION.md` — the 10-language hybrid decision.
3. `docs/BLOCKED_ITEMS.md` — what is blocked and why.
4. `docs/CATALOG_PARITY.md` — catalog state (240 tweaks / 102 debloat / 83 apps /
   40 privacy rules: 38 equivalent · 0 partial · 2 documented gaps).
5. `docs/GO_TOOLCHAIN_BOOTSTRAP.md` — how to rebuild the Go toolchain.

**First actions in the new session (the sandbox resets between sessions):**
1. Rebuild the Go toolchain from source using the exact bootstrap chain in
   `AGENTS.md` §5 (go1.4.3 → go1.17.13 → go1.20.14 → go1.22.12, ~7 minutes,
   sources from codeload.github.com; no system Go exists).
2. Reinstall node_modules (`npm install`) and re-run the verification battery
   in `AGENTS.md` §6.
3. Skim `git log origin/main --oneline -5` to see where the last session left off.

**Known current state (verified 2026-08-16, session arena/01a00795-winforge):**
- Engine: 18/18 test packages green incl. race; ~6.7 MB winforge.exe.
- **Catalog is 240 tweaks, all named.** New native op types (2026-08-16):
  `power_hibernate` (CallNtPowerInformation SystemReserveHiberFile),
  `power_processor_state` (PowerWrite/ReadACValueIndex + PowerSetActiveScheme),
  `netbios` (NetBT Interfaces NetbiosOptions), `registry_delete_key`
  (RegDeleteTreeW, min depth 2), plus default-value registry writes (empty
  `name`). These retired 4 of the 7 web-tweak exclusions — only 3 remain
  (perf-pagefile REG_MULTI_SZ, perf-memcompression PowerShell-only,
  exp-classic-paint narrative).
- **Privacy parity: 38 equivalent · 0 partial · 2 documented gaps** out of 40
  web rules. 17 winforge-* privacy tweaks added (ConsentStore Deny ×8, mDNS,
  NCSI, WCN, device metadata, Recall, cloud clipboard, Edge DiagnosticData,
  SmartScreen enforce, DDV disable) — every value grounded in
  learn.microsoft.com / policy-CSP docs. Remaining gaps are deliberate:
  priv-dnt (browser-level), priv-microsoft-store-ads (no documented registry
  backing — do not fabricate one).
- Backlog area artifacts on the branch: `docs/WINDOWS_SMOKE_CHECKLIST.md`
  (Area D), rewritten `ci.yml.fixed` with parity job + artifact upload
  (Area E), `native/build-lua.sh` + `docs/LUA_PLUGIN_PLAN.md` (Area F — .so
  executed 6*7=42; DLL PE-verified; in-engine binding deferred, see the cgo
  tension in the plan), `docs/WASM_PLUGIN_SANDBOX.md` with executed spike
  (Area G), `docs/ADR-001-ui-engine-bridge.md` + `/engine/*` Next rewrite +
  `EngineStatusCard` on the dashboard (Area H).
- Blockers: BLK-3 (no `workflows` permission — never edit
  `.github/workflows/*`; copy `ci.yml.fixed` when it clears), BLK-6 (no
  Windows runtime; run `docs/WINDOWS_SMOKE_CHECKLIST.md` on a real machine).

**Next steps, in priority order:**
1. Windows smoke checklist execution (BLK-6, needs real hardware).
2. `ci.yml.fixed` → `.github/workflows/ci.yml` the moment BLK-3 clears.
3. Lua in-engine binding (Windows LazyDLL path, `docs/LUA_PLUGIN_PLAN.md`).
4. WASM plugin tier implementation (`docs/WASM_PLUGIN_SANDBOX.md`).
5. UI↔engine bridge phase 2 (POSTs through /engine/*; requires engine
   CSRF/auth design first — ADR-001).

**Hard rules (from the project owner):**
- ZERO fabrication: verify every claim by executing build/test/parse before
  asserting it. No TODOs, no placeholders, no mock data.
- Never modify `.github/workflows/*` (push is rejected without the `workflows`
  permission).
- Keep the verification battery green before committing; keep docs honest; run
  `python3 tools/catalog_parity.py --write-report docs/CATALOG_PARITY.md` after
  any catalog change (then re-append the manual sections from "## State of the
  catalog" — the tool only writes the generated part); update `AGENTS.md` when
  reality changes.
- Security invariants in AGENTS.md §7 override feature pressure. The elevated
  executor never runs PowerShell/powercfg/wmic; new capabilities are native
  ops, not allowlist additions.
- CI failure logs cannot be downloaded from the sandbox — debug locally.

**Session mechanics & gotchas:** work only on the branch the session assigns
(off current `main`), push only there, open PRs back to `main` with
`gh pr create --head <branch>`. Merged branches `arena/01a006b4-winforge`
(PR #11) and `arena/01a0075d-winforge` (PR #12) must not be reused; PR #13
(ECC bundle, other agent lineage) is open — do not touch it. Sandbox quirks:
shell output escapes backslashes (trust `od -c`), `/tmp` and node_modules
vanish between sessions, `git diff --check` must stay clean (CI enforces it).
