# AGENTS.md — WinForge Agent Memory (Secondary Brain)

> Written by the agent for itself and successors. Everything here is either
> verified-by-execution or clearly marked as a plan. Re-verify before trusting;
> update this file when reality changes.

## 1. What this project is

WinForge Elite — all-in-one Windows optimization / debloat / privacy / repair /
power-user suite (inspired by Chris Titus Tech's Windows Utility). Two parts:

1. **Native engine (the product now)** — self-contained Go program, stdlib-only,
   builds a ~6.3 MB static `winforge.exe`. Full CLI + dashboard server + web UI.
   This is Phases 1–3 of the roadmap, already implemented.
2. **Next.js web simulation** — the original fullstack demo (PostgreSQL + Drizzle),
   catalog of record for tweaks/debloat/privacy/apps. Kept as the rich UI and the
   canonical catalog source.

History: repo went Go (PRs #3–#8) → Next.js simulation + WPF scaffold (PR #10) →
back to **Go-primary hybrid** (this line of work, merged 2026-08-16).

## 2. Architecture decision (final): 10-language hybrid, Go primary

`docs/LANGUAGE_SELECTION.md` v3 is the authority. Roles:

| Language | Role | Status |
|---|---|---|
| **Go 1.22** | PRIMARY core: engine, CLI, HTTP API, audit/undo, scheduler, updater, plugins | ✅ merged + CI-green |
| React/TS | UI (Next.js app + engine's bundled `web/` dashboard) | ✅ in-repo |
| PowerShell | Windows recipes (Appx/winget/DISM/schtasks/DNS) — engine models natively | recipes documented, engine-native where possible |
| Zig 0.16 | Companion: compile C deps (Lua, wasm3), Lite CLI fallback | scaffold in `native/` |
| C via zig cc | Vendored libs | verified capable |
| Lua 5.4 | Community-pack plugins | DLL build verified; **in-engine binding LANDED 2026-08-16** (Windows LazyDLL host; platform-independent logic tested on Linux via a fake host) |
| WebAssembly | Sandbox for hostile plugins | wasmtime DLL/spike verified; **re-scoped 2026-08-16** (no unverified binding — see `docs/WASM_REALSCOPE_2026-08-16.md`) |
| Python | Catalog tooling (`tools/`), build/test automation | ✅ in use |
| Bun/TS | UI dev server, packaging fallback | scaffold in `runtime/` |
| Node SEA | Packaging fallback | verified available |

**Archived**: WPF Phase 1 (`WinForge.Elite/`) — dormant due to BLK-1/7.
**Excluded with evidence**: Rust, Deno, Flutter, Java, FPC, Nim, Tcl, Electron (BLK-8).

## 3. Directory map

```
cmd/winforge/          engine main
internal/              Go engine (app, appmanager, audit, bloatware, cli, config,
                       engine, httpapi, isobuilder, maintenance, platform, plugin,
                       power, procout, registry, restorepoint, scheduler, service,
                       tweak, updater, winapi)
config/*.json          embedded catalogs: tweaks (219), debloat (102), apps (83),
                       playbooks, dns, protectedServices
web/                   engine dashboard (index.html, app.js, style.css)
src/                   Next.js app (catalog of record: src/db/seed-data.ts)
WinForge.Elite/        ARCHIVED WPF Phase 1 (reference only)
native/                Zig core scaffold + build.sh (secondary)
runtime/               Bun FFI bridge scaffold + test_core.ts (secondary)
tools/                 Python: catalog_parity.py, web_catalog_to_engine.py
docs/                  LANGUAGE_SELECTION.md, BLOCKED_ITEMS.md,
                       GO_TOOLCHAIN_BOOTSTRAP.md, CATALOG_PARITY.md,
                       GO_ENGINE_README.md, HANDOVER_PROMPT.md
ci.yml.fixed           drop-in CI (needs workflows permission)
ci/github-actions-ci.yml  the Go CI from the go-impl era (reference)
```

## 4. Environment constraints (verified 2026-08-16 — re-probe if something fails)

Sandbox is Debian Linux. **Reachable**: registry.npmjs.org, pypi.org (+ files),
github.com API/codeload (also tarballs). **Blocked**: dot.net/nuget/azureedge,
go.dev/dl.google/proxy.golang, nodejs.org, deb.debian.org (apt), electron
mirrors, rust/crates, deno, flutter, java/maven, freepascal, nim, tcl/lua.org,
and the GitHub Actions **log** blob host (results-receiver / azure blob) — you
cannot read CI failure logs, only job statuses.

**Crucial**: `/tmp` is wiped between sessions; `node_modules` is not snapshotted.
The bootstrapped Go toolchain lives in `/tmp/gobootstrap` and disappears —
rebuild it each new session (7 min, see below). Do NOT commit binaries.

## 5. Go toolchain bootstrap (REQUIRED at session start)

```bash
BASE=/tmp/gobootstrap && mkdir -p $BASE && cd $BASE
for tag in go1.4.3 go1.17.13 go1.20.14 go1.22.12; do
  curl -sSL "https://codeload.github.com/golang/go/tar.gz/refs/tags/${tag}" -o "${tag}.tar.gz" &
done; wait
cat > $BASE/cc-wrapper.sh <<'EOF'
#!/bin/sh
exec gcc -fcommon -no-pie "$@"
EOF
chmod +x $BASE/cc-wrapper.sh
tar xzf go1.4.3.tar.gz && mv go-go1.4.3 go1.4
(cd go1.4/src && CGO_ENABLED=0 CC_FOR_TARGET=$BASE/cc-wrapper.sh ./make.bash)
tar xzf go1.17.13.tar.gz && mv go-go1.17.13 go1.17
(cd go1.17/src && GOROOT_BOOTSTRAP=$BASE/go1.4 CGO_ENABLED=0 ./make.bash)
tar xzf go1.20.14.tar.gz && mv go-go1.20.14 go1.20
(cd go1.20/src && GOROOT_BOOTSTRAP=$BASE/go1.17 CGO_ENABLED=0 ./make.bash)
tar xzf go1.22.12.tar.gz && mv go-go1.22.12 go1.22
(cd go1.22/src && GOROOT_BOOTSTRAP=$BASE/go1.20 CGO_ENABLED=1 ./make.bash)
export PATH=$BASE/go1.22/bin:$PATH GOPROXY=off GOFLAGS=-mod=mod
go version   # expect go1.22.12
```
Full rationale: `docs/GO_TOOLCHAIN_BOOTSTRAP.md`.

## 6. Verification battery (run before every commit)

```bash
cd /home/user/winforge
export PATH=/tmp/gobootstrap/go1.22/bin:$PATH GOPROXY=off GOFLAGS=-mod=mod
gofmt -l .                          # must print nothing
go vet ./...                        # OK
go test ./...                       # 18/18 ok
go test -race ./...                 # 18/18 ok
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" \
    -o /tmp/wf.exe ./cmd/winforge && head -c 2 /tmp/wf.exe   # "MZ"
npm install --no-audit --no-fund && npm run typecheck && npm run lint
python3 tools/catalog_parity.py     # exit 0 = no gaps
git diff --check "$(git hash-object -t tree /dev/null)" HEAD   # no output
```

## 7. Catalog pipeline (Python tooling role)

- `src/db/seed-data.ts` is the **catalog of record** (64 tweaks, ~95 debloat, 64 apps, 40 privacy rules, presets). Note: the seed has **40** `priv-*` rules, not 41 as an earlier handover claimed — verified by regex over the file.
- `tools/web_catalog_to_engine.py --apply` — merges web tweaks into
  `config/tweaks.json` as native ops (idempotent; removes stale merges).
- `tools/catalog_parity.py --fix` — backfills atlas-* metadata by op-signature
  match; syncs `config/debloat.json` + `config/applications.json`; `--write-report`
  regenerates `docs/CATALOG_PARITY.md`. Also diffs the 40 web `privacySeed`
  rules against the engine catalog via curated triage tables (`PRIVACY_MAP` /
  `PRIVACY_GAPS`, each entry grounded in real engine ops): 20 equivalent,
  2 partial, 18 documented gaps; untriaged rules or dangling mappings exit 1.
- `tools/atlas_metadata.py --atlas-src <Atlas clone> [--apply]` — backfills
  atlas-* names/descriptions verbatim from the AtlasOS repo
  (`src/playbook/Configuration/tweaks/**/<slug>.yml` ↔ id `atlas-<slug>`).
  Guards on 100% apply-side op-signature overlap before writing (129/129
  matched 2026-08-15, dry-run exits 1 if anything is unverified).
- **Security boundary (do not violate)**: the elevated executor only runs
  allowlisted system executables (dism, w32tm, lodctr, winmgmt, rundll32,
  wevtutil, fsutil, setx, bcdedit, netsh). PowerShell/powercfg/wmic are
  DELIBERATELY refused (`TestRunCommandElevationBoundary`). The converter must
  never emit such commands. 3 web tweaks remain excluded for exactly these
  reasons (documented in CATALOG_PARITY.md); 4 former exclusions were retired
  2026-08-16 by native ops (power_hibernate, power_processor_state, netbios,
  registry_delete_key + default-value registry writes).
- Engine op types: registry_set_dword/_string/_qword, registry_delete,
  registry_delete_key (RegDeleteTreeW, loader requires depth ≥ 2), command,
  service_start/_stop/_start_mode, task_enable/_disable, power_scheme,
  power_hibernate (CallNtPowerInformation SystemReserveHiberFile),
  power_processor_state (PowerWrite/ReadACValueIndex + PowerSetActiveScheme),
  netbios (NetBT\Parameters\Interfaces NetbiosOptions). Registry ops with an
  empty name address the key's default value (documented RegSetValueExW
  semantics).
- Engine risk tiers: low/medium/high (no "expert" — maps to high).
- **Plugins** (`internal/plugin`): JSON plugins (`tweaks.json`) and, as of
  2026-08-16, **Lua packs** (`manifest.json` `"type":"lua"`, `pack.lua`). Lua
  is Windows-only: a cgo-free `syscall.LoadDLL` host binds a bundled
  `lua54.dll` (next to the exe or the data dir, never PATH). The whitelisted
  API (`winforge.registry.set/delete`, `winforge.service.set_start_mode`,
  `winforge.log`, `winforge.tweak{...}:commit()`, `winforge.revert`) builds
  `config.Operation`s through `config.ValidateOperationForPlugin`; a closed
  plugin op-type whitelist forbids command/appx/task/power/netbios/delete-key.
  `os/io/debug/package/loadfile/load/loadstring/require/dofile` are removed and
  a `LUA_MASKCOUNT` hook (budget 10M) aborts runaway scripts. On Linux (or with
  no DLL) Lua plugins are skipped best-effort; all platform-independent logic
  is unit-tested with a fake `ScriptHost`. Runtime behavior is BLK-6
  (checklist §11). Elevated processes ignore all plugins (UAC boundary). See
  `docs/LUA_PLUGIN_PLAN.md` and `examples/plugins/example-lua-pack/`.

## 8. Blocker register

`docs/BLOCKED_ITEMS.md` is the authority. Current status:
- **BLK-1** .NET toolchain unreachable → WPF archived.
- **BLK-2** Go toolchain → ✅ RESOLVED (source bootstrap).
- **BLK-3** GitHub App lacks `workflows` permission → **cannot modify
  `.github/workflows/*`**. CI (the stale Go workflow, which actually covers the
  engine well) runs on push; it was red on whitespace + Windows go test — both
  fixed in the mainline PR (2026-08-16), now expected green.
- **BLK-6** No Windows runtime — Windows-only behavior verified by cross-compile
  + reading; manual checklist pending on a real Windows box.
- CI failure logs are unreadable from the sandbox (log host blocked); debug by
  local reproduction instead.

## 9. Session state (as of last handover, 2026-08-16)

Done:
- WPF Phase 1 written + statically verified (archived).
- Go engine merged into mainline; catalog parity achieved (219/102/83).
- Windows `go test` failures fixed (Unix-assumption tests → self-binary helpers +
  `executor_unix_test.go`); repo-wide trailing-whitespace cleanup for CI.
- CI expected fully green; PR merged to main.

Done (2026-08-15, arena/01a0075d-winforge):
- Metadata pass complete: **all 129 atlas-* tweaks named** from the AtlasOS
  repo via new `tools/atlas_metadata.py` (100% apply-side op-overlap guard;
  0 suspects, 0 missing). Parity tool now reports "needing metadata: 0".
- Privacy parity added to `tools/catalog_parity.py`: 40 web `privacySeed`
  rules triaged → 20 equivalent, 2 partial, 18 documented gaps; 18 engine-only
  Privacy extras reported as info. `docs/CATALOG_PARITY.md` regenerated.
- Full verification battery re-run green on fresh sandbox (Go 1.22.12 source
  bootstrap, npm reinstall).

Done (2026-08-16, this session):
- Native ops shipped: `power_hibernate` (CallNtPowerInformation
  SystemReserveHiberFile + IsPwrHibernateAllowed), `power_processor_state`
  (PowerGetActiveScheme/PowerRead-/PowerWriteACValueIndex/PowerSetActiveScheme
  — powercfg fully replaced for processor state), `netbios` (NetbiosOptions on
  every NetBT\Parameters\Interfaces subkey — the documented registry backing
  of WMI SetTcpipNetbios), `registry_delete_key` (RegDeleteTreeW; loader
  refuses paths < 2 components). Default-value registry writes work via empty
  `name` (documented RegSetValueExW/RegGetValueW/RegDeleteValueW semantics).
- Converter upgraded: emits the new op types; `(Default)` → empty name;
  undo `-> removed` → registry_delete_key; TS `\"` unescaping fixed;
  idempotent (0 diffs on the 56 previously-merged tweaks).
- Catalog: 219 → **240** tweaks: 4 web merges (ui-classic-context,
  net-disable-netbios, pwr-hibernation, pwr-processor-mgmt) + **17 hand-written
  winforge-* Privacy tweaks** closing documented privacy gaps (8 ConsentStore
  Deny caps incl. chat; mdns EnableMDNS=0; NCSI NoActiveProbe=1; WCN
  DisableWcnUi/EnableRegistrars; PreventDeviceMetadataFromNetwork; Recall
  DisableAIDataAnalysis HKLM+HKCU; AllowCrossDeviceClipboard=0; Edge
  DiagnosticData=0; EnableSmartScreen=1; DisableDiagnosticDataViewer=1 —
  every value verified against learn.microsoft.com/policy CSP docs).
  Exclusions 7 → **3** (perf-pagefile, perf-memcompression, exp-classic-paint).
  Privacy triage: **38 equivalent · 0 partial · 2 gaps** (priv-dnt =
  browser-level; priv-microsoft-store-ads = no documented registry backing).
- Full battery re-verified green (fmt/vet linux+windows/18-pkg tests+race/
  PE cross-compile 6.67 MB/typecheck/lint/parity exit 0/whitespace).
- Backlog areas D–H covered with committed artifacts:
  - **D**: `docs/WINDOWS_SMOKE_CHECKLIST.md` — full BLK-6 checklist incl.
    first-validation items for the 4 new native ops + new privacy tweaks.
  - **E**: `ci.yml.fixed` rewritten — now the live Go CI + parity job
    (parity exit 0 + converter-idempotence SHA check) + winforge.exe artifact
    upload + npm typecheck/lint job (old version built the archived WPF).
  - **F**: `native/build-lua.sh` — Lua 5.4.7 via zig cc → liblua54.so (Linux,
    executed: 6*7=42 via ctypes) + lua54.dll (PE verified). Binding plan +
    the cgo tension documented in `docs/LUA_PLUGIN_PLAN.md` (Windows LazyDLL
    binding keeps stdlib-only; Linux dlopen would need cgo → deferred).
  - **G**: `docs/WASM_PLUGIN_SANDBOX.md` — design + executed spike (wat guest
    importing winforge.health_score host fn ran under wasmtime; win_amd64
    wheel's _wasmtime.dll re-verified as PE).
  - **H**: `docs/ADR-001-ui-engine-bridge.md` (option a: Next rewrites) +
    `/engine/:path*` proxy in `next.config.ts` +
    `src/components/EngineStatusCard.tsx` live engine card on the dashboard
    (polls /engine/api/status+health, graceful offline fallback).

Done (2026-08-16, this session — arena/01a00a47-winforge, W1 Lua binding):
- **Lua in-engine binding LANDED.** `internal/plugin/lua.go` (platform-
  independent API + strict validation + op whitelist), `lua_windows.go`
  (cgo-free syscall.LoadDLL/NewCallback host, absolute-path DLL lookup,
  removed dangerous globals, LUA_MASKCOUNT runaway hook), `lua_other.go`
  (non-Windows ErrLuaUnavailable stub). Manifest gains `"type":"lua"` +
  `"script":"pack.lua"`. Every proposed op runs through
  `config.ValidateOperationForPlugin`; scripts can only propose registry
  dword/string/qword sets, registry value deletes, and service start-mode
  changes (command/appx/task/power/netbios/delete-key are forbidden). Sample
  at `examples/plugins/example-lua-pack/`. 24 plugin tests (incl. all hostile
  cases) pass on Linux; windows `go vet` + `go test -c` + 6.74 MB PE clean.
  Windows runtime behavior is BLK-6 (new checklist §11). Elevated processes
  still ignore all plugins.

Next (prioritized):
1. Windows runtime smoke checklist (BLK-6) on a real machine — now also
   covers the four new native ops, the 17 new privacy tweaks, the Lua plugin
   runtime (§11), AND token-auth behavior.
2. CI modernization when `workflows` permission lands (copy ci.yml.fixed).
3. WASM sandbox: implement on a Windows-capable runner per
   docs/WASM_PLUGIN_SANDBOX.md + docs/WASM_REALSCOPE_2026-08-16.md (or
   formally accept Lua-only).
4. UI↔engine bridge phase 2: wire Next UI POST buttons through
   `src/lib/engine-client.ts` (the CSRF/session-token auth from ADR-002 is
   now in place; see ADR-001).

## 10. Gotchas & lessons (each cost time once)

- Tool output in this sandbox escapes backslashes — trust `od -c`, not eyeballs.
- Python regex edits via sed/python heredocs bite; when a patch "doesn't match",
  print the target line with `od -c` first.
- Dapper in the archived C# mapped JSON TEXT→List<string> via a type handler;
  the Go engine reads config directly — no such issue there.
- Engine `scan`/health excludes one-way ops by design (`UnverifiableTweaks`) —
  counts won't equal catalog size; not a bug.
- `git diff --check` flags trailing whitespace including intentional markdown
  double-spaces — repo must stay clean for the CI syntax job.
- The merge into mainline used `--allow-unrelated-histories` (go-impl was a
  separate root). Branch names from earlier work: `origin/go-impl` (=
  arena/019ffd90-winforge), arena/019ffd76-winforge, arena/019ffd90-winforge.
- PR #9 (winforge ECC bundle) is OPEN from another agent lineage — don't touch.

## 11. Operating protocol (user-mandated, non-negotiable)

1. ZERO fabrication — never claim a file/feature exists without verifying.
2. Verify by execution: build/test/parse/probe before asserting.
3. Honest docs — every exclusion/gap gets a written reason; no "TODO" placeholders.
4. Never modify `.github/workflows/*` (BLK-3) — push would be rejected.
5. Every service-level change: try/catch + user-friendly message (C# convention)
   or error-return with audit trail (Go convention).
6. Commit to the session branch; keep CI green; update AGENTS.md when reality changes.
