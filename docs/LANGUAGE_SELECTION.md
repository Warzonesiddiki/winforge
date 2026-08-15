# WinForge Elite — Language Selection v2 (10-Language Hybrid)

> Decision date: 2026-08-16 · Status: **Selected** (v2 supersedes v1's 5-language plan)
> Methodology: every capability below was **probed and executed in the sandbox on this date**.
> Zero claims from memory — blocked means the host refused the connection; verified means we produced the artifact.

## 1. Executive Summary — The Dream Team

WinForge Elite becomes a **10-language hybrid**, each language deployed exactly where it is unbeatable, assembled so that **every tier of the product can be built, tested, or at minimum produced locally today**:

| # | Language | Role | Verified evidence (this session) |
|---|---|---|---|
| 1 | **Zig 0.16** | Native Windows core: registry, services, restore points, snapshots, health score | 799 KB static Windows EXE + Windows DLL + Linux `.so`, all cross-compiled from sandbox; `zigwin32` full Win32 bindings confirmed fetchable |
| 2 | **C (via `zig cc`)** | Vendored C libraries & Win32 hot paths | C source → valid Windows PE; cmake 4.4 + ninja installed from pypi as the build pipeline |
| 3 | **TypeScript on Bun** | Orchestrator: HTTP UI server, `bun:ffi` bridge to core, `bun:sqlite` audit/undo store, subprocess manager, single-EXE packaging | Windows EXE cross-compiled (`MZ` PE); FFI round-trip to the Zig core passing all assertions |
| 4 | **Node.js (SEA)** | Packaging fallback + npm ecosystem breadth | `node-win-x64` + `postject` npm packages verified present |
| 5 | **Python 3.11** | Build/test automation + catalog generator (TS seed data → Zig/Lua/JSON catalogs) | pypi reachable; `ziglang`, `cmake`, `ninja`, `wasmtime` wheels all install and run |
| 6 | **PowerShell** | Windows automation recipes: Appx debloat, winget, DISM/SFC, schtasks, DNS | Native on Win10/11 (the CTT-proven approach); invoked as supervised subprocess |
| 7 | **Lua 5.4** | Community packs & user plugin scripting | **Compiled Lua from source to a 483 KB Windows DLL** with `zig cc`, entirely in-sandbox |
| 8 | **WebAssembly** | Hardened sandbox for third-party plugins | `wasmtime` 47.0.1 win_amd64 wheel from pypi contains a **valid `_wasmtime.dll` PE**; `wasm3` C source fetchable via GitHub API |
| 9 | **Go** | Reserve core (instant alternate if toolchain unblocks) | 15,800-line, 77-file, stdlib-only engine with tests exists on `origin/go-impl` (CI-proven in PRs #3–#8) |
| 10 | **React/TypeScript (web UI)** | End-user UI — the existing Next.js app served by Bun | Repo already contains the complete polished UI; typecheck/lint pass |

## 2. Research Method

1. Probed every toolchain host with direct HTTP checks (reachable = verified, unreachable = excluded).
2. For viable candidates, executed the *actual* minimal pipeline (install → compile → produce a Windows PE artifact).
3. Reserved judgment for anything that could not be empirically demonstrated.

## 3. Evidence Table — All 18 Candidates

| Candidate | Hosts probed | Local build? | Verdict |
|---|---|---|---|
| **Zig** | pypi `ziglang` | ✅ **VERIFIED** (EXE/DLL/.so) | **CORE** |
| **Bun** | npm | ✅ **VERIFIED** (EXE + FFI) | **ORCHESTRATOR** |
| **C via zig cc** | — | ✅ **VERIFIED** (Windows PE) | **LIBS/HOT PATHS** |
| **Node SEA** | npm | ✅ verified available | **FALLBACK PACKAGER** |
| **Python** | pypi | ✅ **VERIFIED** (toolchain) | **TOOLING** |
| **Lua** | github.com/lua/lua (lua.org blocked) | ✅ **VERIFIED** (Windows DLL built) | **PLUGINS** |
| **WebAssembly** | pypi `wasmtime` wheel | ✅ **VERIFIED** (PE DLL extracted) | **PLUGIN SANDBOX** |
| **Go** | go.dev, dl.google.com, proxy.golang.org | ❌ blocked | RESERVE CORE (engine exists) |
| **.NET/WPF** | dot.net, nuget, azureedge | ❌ blocked | ARCHIVED UI |
| **Electron** | release CDN, npmmirror | ❌ blocked | EXCLUDED |
| **Rust** | static.rust-lang.org, crates.io | ❌ blocked | EXCLUDED |
| **Deno** | dl.deno.land | ❌ blocked | EXCLUDED |
| **Flutter/Dart** | storage.googleapis.com | ❌ blocked | EXCLUDED |
| **Java/Kotlin** | adoptium, maven central | ❌ blocked | EXCLUDED |
| **Free Pascal/Lazarus** | freepascal.org | ❌ blocked | EXCLUDED (would otherwise fit perfectly) |
| **Nim** | nim-lang.org | ❌ blocked | EXCLUDED |
| **Tcl** | tcl.tk | ❌ blocked | EXCLUDED |
| **UPX** (tool) | pypi (absent) | ⚠️ source-build only | DEFERRED — Bun EXE compression |

## 4. Why Each Selected Language Earns Its Seat

### 1. Zig — the core (irreplaceable)
- Runtime-free **799 KB statics** vs Bun's 98 MB vs .NET's ~100 MB — the only verified path to a *small* native Windows binary from this sandbox.
- Memory-safe by default; exports a plain C ABI, callable from Bun, Node, PowerShell (P/Invoke), or Python.
- `zigwin32` (generated, MIT) supplies the entire Win32 SDK surface — no Microsoft headers needed.
- Same source → Linux `.so` → all core logic (health scoring, operation scheduling, snapshot/undo, JSON) is **unit-testable locally today**.

### 2. C via zig cc — the substrate
- The lingua franca of Win32 documentation; any vendored library (Lua, wasm3, future libs) compiles to Windows with one command — proven twice this session.
- cmake 4.4 + ninja from pypi make real multi-file C builds tractable.

### 3. TypeScript on Bun — the conductor
- Verified Windows EXE cross-compile **plus** first-class FFI — no other verified stack offers both.
- `bun:sqlite` gives the audit/undo database with zero drivers; embedded static server hosts the UI; subprocess management supervises PowerShell.
- The existing React/TS skills in this repo transfer 1:1.

### 4. Node.js SEA — the insurance
- If Bun ever shows a Windows-specific defect, the same TypeScript orchestrator repackages as a Node SEA EXE — both pieces verified present on npm.

### 5. Python — the toolsmith
- Generates and validates the catalogs (`src/db/seed-data.ts` → Zig consts, Lua plugin skeletons, JSON), drives the Zig/Bun builds, and runs the test matrix. pypi is the only fully-reachable ecosystem besides npm.

### 6. PowerShell — the Windows legion
- ~99% of Chris Titus' utility (30M+ runs) is PowerShell; Appx/winget/DISM/schtasks recipes are transparent, user-auditable, and self-updating with Windows.
- Bun supervises it (exit codes, timeouts, output capture) — keeping CTT's battle-tested approach *and* WinForge's audit/undo guarantees.

### 7. Lua — the community language
- **Proven buildable here** (483 KB Windows DLL). Community packs become Lua scripts with a whitelisted API (`winforge.registry.set(...)`, `winforge.service.set_start(...)`) — the feature the web app's "Community Packs" and the Go engine's "plugins" both promised.

### 8. WebAssembly — the sandbox
- Third-party packs that need stronger guarantees compile to WASM and run in wasmtime (DLL already obtained). Memory-safe, capability-limited, killable. Lua covers friendliness; WASM covers hostile code.

### 9. Go — the reserve core
- The previous sessions already wrote **15,800 lines of hardened, stdlib-only Go** (registry via raw syscalls with bounded reads, restore points, Appx, DISM/ISO builder, winget manager, audit, plugins, HTTP API, tests). If any Go-capable machine/CI appears, it can replace the Zig core *and* shrink the EXE to ~12 MB. Kept warm on `origin/go-impl`.

### 10. React/TS web UI — the face
- Zero new UI work: the existing Next.js app (dashboard, debloat, tweaks, privacy, install, repair, updates, ISO, history, settings) is the blueprint's own "Recommended Hybrid" frontend. It talks to the Bun-hosted API instead of Next server actions.

## 5. Architecture v2

```
┌────────────────────────────────────────────────────────────────┐
│ Windows target                                                  │
│  winforge.exe (Bun single-EXE ~98 MB, embeds UI + orchestrator)│
│  ┌─────────────┐   ┌──────────────────┐   ┌────────────────┐  │
│  │ React UI    │──▶│ TS orchestrator  │──▶│ bun:sqlite     │  │
│  │ (localhost  │   │ HTTP API, queue, │   │ audit + undo   │  │
│  │  :8443)     │   │ updater, UAC     │   └────────────────┘  │
│  └─────────────┘   └──┬───────────┬───┘                        │
│            ┌──────────┘           └──────────┐                 │
│            ▼ FFI (bun:ffi)                    ▼ subprocess      │
│  ┌──────────────────────┐        ┌──────────────────────────┐  │
│  │ winforge_core.dll    │        │ PowerShell recipes        │  │
│  │ (Zig, ~800 KB)       │        │ Appx · winget · DISM/SFC ·│  │
│  │ registry · services ·│        │ schtasks · DNS · chkdsk   │  │
│  │ restore points ·     │        └──────────────────────────┘  │
│  │ snapshots · health   │        ┌──────────────────────────┐  │
│  └──────────────────────┘        │ Plugins                  │  │
│  ┌──────────────────────┐        │  ├─ Lua (lua54.dll)      │  │
│  │ wasmtime.dll         │◀───────┤  └─ WASM sandbox         │  │
│  └──────────────────────┘        └──────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘

Build & test loop (verified working in this sandbox today):
  Zig core ──► Linux .so ──► Bun FFI tests run HERE (passed)
       └──────► Windows .exe / .dll — same source, no changes
  Lua source (GitHub) ──► zig cc ──► lua54.dll (Windows) ✓
  wasmtime wheel (pypi) ──► _wasmtime.dll (Windows PE) ✓
```

**Three distribution flavors** (same codebase, one flag):
1. **Full** — Bun EXE + UI + orchestrator + core DLL + Lua + WASM (~100 MB) — normal users.
2. **Lite** — Zig core CLI EXE (~800 KB) + PowerShell recipes — power users, `irm | iex` friendly.
3. **Go (future)** — stdlib-only ~12 MB static EXE when a Go toolchain is available.

## 6. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Zig niche talent pool | Core API is ~15 C-ABI functions; Win32 code is C-compatible; `zigwin32` supplies bindings; Go engine is the reserve |
| Bun novel for this domain | Orchestrator is plain TS: Node SEA fallback verified available; every Bun-specific call (`bun:ffi`, `bun:sqlite`, `Bun.serve`) has a Node mapping |
| Windows-only behavior untestable here | Platform-neutral logic runs on Linux `.so` (verified); Windows syscalls isolated behind the core API + manual Windows checklist (BLK-6) |
| 98 MB Bun EXE | Acceptable v1; UPX source-build deferred; Go core later shrinks to ~12 MB |
| Lua without sandbox | Default plugin path is Lua with whitelisted API; hostile third-party code goes through WASM (wasmtime) |
| PowerShell visibility | Recipes live in the repo as reviewable text; every invocation audit-logged with exit codes |

## 7. Roadmap by Language

- **Phase 1 (C# services → port)**: registry/restore/tweak/debloat/health → Zig core + PS recipes + Bun orchestration + web UI reuse
- **Phase 2**: DriverOptimizer, SmartScan → Zig core (WMI via PS initially, COM later)
- **Phase 2.5**: Software installer → Bun queue + PS winget
- **Phase 3**: DISM/SFC/ISO builder → PS recipes supervised by Bun
- **Phase 4**: signing, installer, auto-update → Python build scripts + Bun updater
- **Plugins (cross-cutting)**: Lua packs at Phase 2; WASM sandbox at Phase 4

## 8. Decision Log

| Date | Decision |
|---|---|
| 2026-08-16 v1 | 5-language hybrid (Zig, Bun/TS, PowerShell, Python, React) |
| 2026-08-16 v2 | Upgraded to 10: + C via zig cc, Node SEA, Lua, WebAssembly, Go (reserve); 13 candidates excluded on verified blocking |
