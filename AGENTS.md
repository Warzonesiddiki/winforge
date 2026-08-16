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

## 2. Architecture decision (final): 13-language hybrid, Go primary (v4: best-of-best, zero compromise)

`docs/LANGUAGE_SELECTION.md` v4 is the authority. Roles:

| Language | Role | Status |
|---|---|---|
| **Go 1.22** | PRIMARY core: engine, CLI, HTTP API, audit/undo, scheduler, updater, plugins | ✅ merged + CI-green |
| React/TS | UI (Next.js app + engine's bundled `web/` dashboard) | ✅ in-repo |
| PowerShell | Windows recipes (Appx/winget/DISM/schtasks/DNS) — engine models natively | recipes documented, engine-native where possible |
| Zig 0.16 | Companion: compile C deps (Lua) via `native/build-lua.sh` | `native/build-lua.sh` in use; the abandoned `core.zig`/Bun scaffold is archived |
| C via zig cc | Vendored libs (Lua 5.4.7) | verified capable; builds `lua54.dll` |
| Lua 5.4 | Community-pack plugins | DLL build verified; **in-engine binding LANDED 2026-08-16** (Windows LazyDLL host; platform-independent logic tested on Linux via a fake host) |
| WebAssembly | Sandbox for hostile plugins | wasmtime DLL verified; **platform-independent tier LANDED 2026-08-16** (`wasm.go` + fake host, 24 wasm tests; Windows C binding deferred per `WASM_REALSCOPE_2026-08-16.md`, DLL-search stub in `wasm_windows.go`) |
| Python | Catalog tooling (`tools/`), build/test automation | ✅ in use |
| Node SEA / Bun | Packaging fallback (abandoned) | scaffold archived in `archive/runtime/` |
| SQL (Drizzle) | Audit/history + Next.js catalog DB | ✅ in-repo — 18 tables, drizzle-kit |
| YAML/JSON | Playbooks & catalog source (Atlas) | ✅ 129/129 Atlas YAML verified |
| Inno Setup | Installer & signing flow | ✅ Phase 4 — isobuilder models it |

**Archived**: WPF Phase 1 (`archive/WinForge.Elite/`), Bun/Zig core scaffold (`archive/runtime/`, `archive/native/`), stale CI staging copy (`archive/ci/`). See `archive/README.md`.
**Excluded with evidence**: Rust, Deno, Flutter, Java, FPC, Nim, Tcl, Electron (BLK-8).

## 3. Directory map

```
cmd/winforge/          engine main
internal/              Go engine (app, appmanager, audit, bloatware, cli, config,
                       engine, httpapi, isobuilder, maintenance, platform, plugin,
                       power, procout, registry, restorepoint, scheduler, service,
                       tweak, updater, winapi)
config/*.json          embedded catalogs: tweaks (240), debloat (102), apps (83),
                       playbooks, dns, protectedServices
web/                   engine dashboard (index.html, app.js, style.css)
src/                   Next.js app (catalog of record: src/db/seed-data.ts)
native/                build-lua.sh only (builds lua54.dll for Lua plugins)
tools/                 Python: catalog_parity.py, web_catalog_to_engine.py
docs/                  LANGUAGE_SELECTION.md, BLOCKED_ITEMS.md,
                       GO_TOOLCHAIN_BOOTSTRAP.md, CATALOG_PARITY.md,
                       GO_ENGINE_README.md, HANDOVER_PROMPT.md
examples/plugins/      example-lua-pack, example-wasm-pack
archive/               dormant reference code (WPF scaffold, Bun/Zig core, stale CI)
ci.yml.fixed           drop-in CI (needs workflows permission); adds npm build+test
```

> **Go package scope**: always use `./cmd/... ./internal/... .` rather than `./...`.
> The latter walks `node_modules/`, where npm deps (e.g. `flatted`) ship `.go`
> files that are not part of this module and would break the build. The Makefile
> and `ci.yml.fixed` already scope correctly.

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
# Go packages are scoped to avoid node_modules/ (see §3).
gofmt -l cmd internal embed.go     # must print nothing
go vet ./cmd/... ./internal/... .  # OK
go test ./cmd/... ./internal/... . # 18/18 ok
go test -race ./cmd/... ./internal/... .  # 18/18 ok
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X winforge/internal/app.Version=$(git describe --tags --always --dirty)" \
    -o /tmp/wf.exe ./cmd/winforge && head -c 2 /tmp/wf.exe   # "MZ"
npm install --no-audit --no-fund && npm run typecheck && npm run lint && npm test
DATABASE_URL= npm run build          # production build must work with no DB
python3 tools/catalog_parity.py     # exit 0 = no gaps
git diff --check                    # no whitespace errors in your changes
```

Or simply `make verify` (runs all of the above).

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
- Engine risk tiers: low/medium/high. The loader accepts `"expert"` in
  `tweaks.json` and normalizes it to `high` (the web catalog uses four tiers;
  the engine collapses the top two). See `ValidateRisk` in
  `internal/config/models.go`.
- **Plugins** (`internal/plugin`): JSON plugins (`tweaks.json`) and, as of
  2026-08-16, **Lua packs** (`manifest.json` \"type\":\"lua\"`, `pack.lua`) and
  **WASM packs** (`manifest.json` \"type\":\"wasm\"`, `pack.wasm`). Lua is
  Windows-only: a cgo-free `syscall.LoadDLL` host binds a bundled
  `lua54.dll` (next to the exe or the data dir, never PATH). WASM is the
  strong-isolation tier: a `WasmHost` with fuel metering (10M) and bounded
  linear-memory strings; platform-independent proposal/validation (24 wasm
  tests) mirrors Lua and is tested on Linux via a fake `WasmHost`; the
  Windows wasmtime C binding is deferred per `WASM_REALSCOPE_2026-08-16.md`
  (DLL-search stub in `wasm_windows.go` returns `ErrWasmUnavailable` until
  verified on real Windows). Both tiers share the same whitelisted API
  (`registry.set/delete`, `service.set_start_mode`, `log`,
  `tweak{...}:commit()`, `revert`) which builds `config.Operation`s through
  `config.ValidateOperationForPlugin`; a closed plugin op-type whitelist
  forbids command/appx/task/power/netbios/delete-key. `os/io/debug/package/...`
  are removed for Lua and the WASM linear memory is bounded; runaway guests
  are terminated (Lua `LUA_MASKCOUNT` 10M hook, WASM fuel). On Linux (or with
  no DLL) both tiers are skipped best-effort; elevated processes ignore all
  plugins (UAC boundary). See `docs/LUA_PLUGIN_PLAN.md`,
  `docs/WASM_PLUGIN_SANDBOX.md`, `examples/plugins/example-lua-pack/` and
  `examples/plugins/example-wasm-pack/`.

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

Done (2026-08-16, W3 engine mutation auth — PR #16, ADR-002):
- **Session-token auth on every mutation.** Per-instance 32-byte crypto/rand
  base64url token; all POST/PUT/PATCH/DELETE require `X-WinForge-Token`
  (constant-time compare); GETs stay open; new `GET /api/session-token`.
  Enforced in `internal/httpapi/server.go` **after** the loopback-Host and
  same-origin checks and **before** body decode; token helpers live in
  `internal/httpapi/auth.go`. `web/app.js` fetches + sends the token;
  `src/lib/engine-client.ts` is the reusable Next-side client (caches the
  token, invalidates on 401). Tests: token required/wrong/accepted, reads
  open, endpoint serves + rotates, cross-origin/loopback still enforced.
  Windows verification is checklist §12.

Done (2026-08-16, W2 WASM re-scope — NOT shipped, evidence recorded):
- `docs/WASM_REALSCOPE_2026-08-16.md`: wasmtime 47's C API is ~980 symbols /
  ~28 MB .so; a minimal host subset is ~30–35 functions with manual handle
  ownership + C→Go callbacks; there is **no cgo-free Linux path to execute
  it**, and BLK-6 means no Windows runtime to verify a security-boundary
  binding. Design in `docs/WASM_PLUGIN_SANDBOX.md` is unchanged and lists
  exactly what is needed. **Do not ship an unverified binding** — Lua is the
  first scriptable tier.

Done (2026-08-16, W6 memcompression research CLOSED):
- `perf-memcompression` stays excluded, now **with evidence** (see
  `docs/CATALOG_PARITY.md`): the only documented mechanism is PowerShell
  `Disable-MMAgent -MemoryCompression` (refused by the executor allowlist);
  no documented registry value exists; the kernel path
  `NtSetSystemInformation(SystemMemoryCompressionInformation)` is
  undocumented and build-varying. Do not reopen without a citable source.

Done (2026-08-16, W3 phase 2 — Next UI wiring):
- `src/components/EngineTweaks.tsx` — live engine tweak panel (search,
  category filter, applied-only filter, apply/undo buttons) driven entirely
  through `src/lib/engine-client.ts`, so every mutation carries the ADR-002
  token. Mounted on the dashboard under the engine status card; renders
  nothing when the engine is offline, so the simulation is unaffected.

Done (2026-08-16, W2 platform-independent tier — arena/01a00ac0-winforge):
- **WASM proposal/validation LANDED** (Linux-verifiable, no Windows binding).
  `internal/plugin/wasm.go` (platform-independent `wasmAPI` + strict validation
  + op whitelist mirroring Lua), `wasm_windows.go` (DLL-search stub for
  `wasmtime.dll` by absolute path, deliberately returns `ErrWasmUnavailable`
  until the Windows C binding is verified on real hardware — see
  `WASM_REALSCOPE_2026-08-16.md`), `wasm_other.go` (non-Windows stub).
  Manifest gains \"type\":\"wasm\" + \"module\":\"pack.wasm\" (alias
  \"script\" accepted). Module validation: WASM magic `\\x00asm` + version 1,
  4 MiB cap. Same closed whitelist as Lua (registry dword/qword/string,
  registry delete, service start_mode; command/appx/task/power/netbios/delete-key
  forbidden). Fuel budget 10M (wasmtime metering). `internal/app/app.go`
  passes both `LuaDLLDirs` and `WasmDLLDirs` (exe dir + data dir). Sample at
  `examples/plugins/example-wasm-pack/` (8-byte minimal wasm + `pack.wat.example`
  spike). 24 wasmAPI tests + 6 discovery tests (bad hive/oversized/fractional/
  path-traversal/magic/bogus-tweaks) pass on Linux; windows `go vet` + `go test -c`
  + 6.5 MB PE clean. No unverified C binding ships — the security-boundary
  binding remains gated on Windows hardware.

Done (2026-08-16, arena/01a00b24-winforge — project analysis & fixes):
- **Fixed `npm run build`** — `src/db/index.ts` now constructs the pg Pool and
  drizzle instance lazily (via a transparent Proxy) so Next.js build-time page
  collection no longer requires `DATABASE_URL`. Production build passes with
  `DATABASE_URL=""`.
- **Excluded `node_modules/` from Go tooling** — Makefile, `ci.yml.fixed`, and
  AGENTS.md scope all Go commands to `./cmd/... ./internal/... .`; `gofmt`
  targets `cmd internal embed.go`. The `flatted` npm package ships `.go`
  files that `./...` would otherwise compile.
- **Added MIT LICENSE**, `CONTRIBUTING.md`, `SECURITY.md`, `QUICKSTART.md`,
  `.env.example`.
- **Fixed `package.json`** name (`nextjs-postgresql-template` → `winforge-web`);
  added version, description, license, `test`/`test:watch` scripts, vitest.
- **Removed `tsconfig.tsbuildinfo` from git**; expanded `.gitignore`
  (`*.tsbuildinfo`, `.next/`, `out/`, `next-env.d.ts`, Go build artifacts).
- **Graceful shutdown** for `winforge serve` — SIGINT/SIGTERM triggers
  `srv.Shutdown` with a 10s drain, preserving the `listenAndServe` test seam.
- **Risk "expert" accepted** by `ValidateRisk` and normalized to high (with test).
- **First web test suite**: 11 Vitest tests for `src/lib/health.ts` with `@/db`
  mocked; `npm test` runs in Makefile + CI.
- **Version stamping** via `-ldflags "-X winforge/internal/app.Version=..."`.
- **Documented** the intentionally-different Go vs web health algorithms.
- **Archived dead code**: `WinForge.Elite/`, `runtime/`, `native/core.zig`,
  `native/build.sh`, and `ci/` moved to `archive/` (with `archive/README.md`).
  `native/build-lua.sh` stays — it builds `lua54.dll`.
- **README rewritten** to lead with the Go engine, correct counts (18 tables),
  and document build/test/verify commands.
- Full battery re-verified green: fmt, vet (linux+windows), test+race (18 pkgs),
  PE cross-compile (6.6 MB), parity, converter idempotence, locales, npm
  typecheck/lint/test/build.

Next (prioritized):
1. Windows runtime smoke checklist (BLK-6) on a real machine — now also
   covers the four new native ops, the 17 new privacy tweaks, the Lua
   plugin runtime (§11), session-token auth incl. the Next UI (§12), AND the
   WASM DLL-search stub (§13 — WASM host unavailable until verified).
2. CI modernization when `workflows` permission lands (copy ci.yml.fixed).
3. WASM tier Windows binding: implement `wasmtime` C host (~30–35 funcs) on a
   Windows-capable runner per the two WASM docs, execute the §13 checklist,
   or formally accept Lua-only and close W2 with explicit evidence.
4. W6 housekeeping: watch for a documented `priv-microsoft-store-ads` policy
   (do NOT fabricate).
5. Add tests for `internal/platform` and `internal/scheduler` (zero-test packages).
6. Replace `alert()` in `web/app.js` with inline toast/notification UI.

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
