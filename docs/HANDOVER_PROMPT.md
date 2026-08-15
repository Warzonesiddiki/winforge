# WinForge — Session Handover Prompt

> Copy the section below into a fresh Arena session to continue this project.
> Also read the repo's `AGENTS.md` first — it is the agent's memory file.

---

You are continuing work on **WinForge** (`Warzonesiddiki/winforge`), an all-in-one
Windows optimization/debloat/privacy/repair suite. The project is now a
**Go-primary hybrid**: a self-contained stdlib-only Go engine
(`cmd/winforge`, `internal/*`, `config/*.json`, `web/`) merged into `main`, plus
a Next.js web simulation whose `src/db/seed-data.ts` is the catalog of record,
plus Python catalog tooling in `tools/`.

**Start by reading these files, in order:**
1. `AGENTS.md` — the agent memory file (architecture, commands, gotchas, protocol).
2. `docs/LANGUAGE_SELECTION.md` — the 10-language hybrid decision.
3. `docs/BLOCKED_ITEMS.md` — what is blocked and why.
4. `docs/CATALOG_PARITY.md` — current catalog state (219 tweaks / 102 debloat / 83 apps).
5. `docs/GO_TOOLCHAIN_BOOTSTRAP.md` — how to rebuild the Go toolchain.

**First actions in the new session (the sandbox resets between sessions):**
1. Rebuild the Go toolchain from source using the exact bootstrap chain in
   `AGENTS.md` §5 / `docs/GO_TOOLCHAIN_BOOTSTRAP.md` (go1.4.3 → 1.17 → 1.20 →
   1.22.12, ~7 minutes, sources from codeload.github.com). There is no system Go.
2. Re-run the verification battery in `AGENTS.md` §6 (gofmt / vet / tests+race /
   Windows cross-compile / npm typecheck+lint / parity tool / whitespace check).
   Reinstall node_modules (`npm install`) — it is not persisted.
3. Confirm the environment constraints in `AGENTS.md` §4 still hold
   (network egress can change between sessions — re-probe if something fails).

**Known current state (verified, 2026-08-16):**
- Go engine on `main`: builds, vets, 18/18 tests incl. race, cross-compiles to a
  ~6.3 MB `winforge.exe`. CI expected fully green (whitespace + Windows-test
  failures fixed in the mainline PR).
- Catalog parity complete: 56 web tweaks merged as native ops, 1 equivalent,
  7 deliberate exclusions (security boundary — the elevated executor refuses
  PowerShell/powercfg/wmic; see CATALOG_PARITY.md).
- WPF Phase 1 (`WinForge.Elite/`) is archived reference work — do not revive it.
- Zig (`native/`) and Bun (`runtime/`) scaffolds are secondary companions.
- Blockers: BLK-3 (GitHub App has no `workflows` permission — never edit
  `.github/workflows/*`), BLK-6 (no Windows runtime; verify Windows behavior by
  cross-compiling and code-reading, plus the manual checklist).

**Next steps, in priority order:**
1. Metadata pass: name/describe the ~129 unnamed `atlas-*` tweaks in
   `config/tweaks.json` using the AtlasOS repository on GitHub (real sources
   only — never invent text).
2. Privacy parity: extend `tools/catalog_parity.py` to diff the web app's
   41 `privacySeed` rules against the engine's 33 atlas privacy tweaks.
3. Windows runtime smoke test checklist (BLK-6) — needs a real Windows machine.
4. When the GitHub App gains the `workflows` permission: modernize CI
   (`ci.yml.fixed` / `ci/github-actions-ci.yml`).
5. Native engine ops to retire the 4 power/WMI exclusions: `power_hibernate`,
   `power_processor_state` (native `power.SetProcessorState` already exists),
   registry `(Default)`-value writes, native WMI `SetTcpipNetbios`.
6. Lua plugin integration (DLL build already verified) and the WASM sandbox
   (Phase 4).
7. Bridge the Next.js UI to the engine's HTTP API (dashboard on localhost:8696).

**Hard rules (from the project owner):**
- ZERO fabrication: verify every claim by executing build/test/parse before
  asserting it. No TODOs, no placeholders, no mock data.
- Never modify `.github/workflows/*` (push will be rejected without the
  `workflows` permission).
- Keep the verification battery green before committing; keep docs honest and
  update `AGENTS.md` when reality changes.
- CI failure logs cannot be downloaded from the sandbox — debug by local
  reproduction instead.

**Session mechanics:** work on whatever branch the session gives you (off `main`),
push only to that branch, and open PRs back to `main`. The previous session's
branch (`arena/01a006b4-winforge`) is merged — do not reuse it.
