# WinForge — Blocked Items Register

> Purpose: single source of truth for every blocker affecting WinForge development.
> Status verified: 2026-08-16. Update dates as items change.

## Summary

| ID | Blocked item | Severity | Status | Resolved by |
|---|---|---|---|---|
| BLK-1 | Microsoft/.NET toolchain hosts unreachable | High | Blocked | WPF archived; superseded by Go-primary hybrid |
| BLK-2 | Go toolchain + module proxy unreachable | Medium | ✅ **RESOLVED** | Toolchain bootstrapped from source in-sandbox (see below) |
| BLK-3 | GitHub App lacks `workflows` permission | High | Blocked | Repo owner grants permission |
| BLK-4 | Electron runtime binaries unreachable | Low | Blocked | Electron dropped from options |
| BLK-5 | nodejs.org blocked (official Node download) | Low | Worked around | npm `node-win-x64` + `postject` |
| BLK-6 | No Windows runtime for behavioral tests | Medium | Blocked | Manual Windows checklist + Linux-tested core |
| BLK-7 | WPF Phase 1 code cannot be compiled | High | Dormant | Archived; superseded by hybrid |
| BLK-8 | 9 alternative toolchains blocked (Rust, Deno, Flutter, Java, FPC, Nim, Tcl, + see §BLK-8) | Info | Evaluated | Excluded with evidence; see section below |

## BLK-1 — Microsoft / .NET toolchain hosts unreachable

**What fails**: `dot.net`, `api.nuget.org`, `dotnetcli.azureedge.net`, `builds.dotnet.microsoft.com` (all TCP/SSL fail). Also `deb.debian.org` (apt), so no `dotnet-sdk-8.0` package either.

**Impact**: The `WinForge.Elite` WPF project cannot be built, packaged, or type-checked in this environment — only static analysis (C#/XAML parsing, SQL/binding checks, done in prior sessions) is possible.

**Attempted workarounds**: dotnet-install script via GitHub raw/API (script obtained, but it downloads from blocked Microsoft hosts); CI push (blocked by BLK-3).

**Resolution**: Hybrid architecture (see [LANGUAGE_SELECTION.md](./LANGUAGE_SELECTION.md)) — native core in Zig, which cross-compiles Windows artifacts locally. WPF code remains in `WinForge.Elite/` as a dormant alternate UI.

## BLK-2 — Go toolchain + module proxy unreachable → ✅ RESOLVED (2026-08-16)

**What failed**: `go.dev`, `dl.google.com`, `proxy.golang.org` unreachable; no system Go.

**Resolution**: Bootstrapped a full Go 1.22.12 toolchain **from source inside the sandbox** —
`codeload.github.com` (the golang/go mirror) is reachable, so the chain was:
Go 1.4.3 (C-based, compiled with gcc via a `-fcommon -no-pie` wrapper) → Go 1.17.13 → Go 1.20.14 → Go 1.22.12 (cgo enabled). ~7 minutes total. Module proxy is irrelevant: the engine is **stdlib-only** (`GOPROXY=off`).

**Verification after resolution**: `go build ./...` ✅ · `go vet ./...` ✅ · 18/18 test packages ✅ incl. `-race` ✅ · `GOOS=windows GOARCH=amd64 go build` → **`winforge.exe` (6.24 MB, valid PE)** ✅ · windows `go vet ./...` ✅. One test-harness fix required for Linux parity (`isolate()` used `t.TempDir()` with 300-char subtest names → NAME_MAX; now uses a short fixed-prefix dir).

Procedure documented in [GO_TOOLCHAIN_BOOTSTRAP.md](./GO_TOOLCHAIN_BOOTSTRAP.md).

## BLK-3 — GitHub App lacks `workflows` permission

**What fails**: `git push` of any change to `.github/workflows/*` is rejected: *"refusing to allow a GitHub App to create or update workflow `.github/workflows/ci.yml` without `workflows` permission"* (re-verified 2026-08-16).

**Impact**: CI cannot be modernized; the current `ci.yml` in the repo is a stale Go-based workflow that fails every run (expects `go.mod` and `web/` which no longer exist). No `dotnet build` / test / artifact pipeline possible.

**Resolution steps (repo owner)**: GitHub App settings → Permissions → Repository permissions → **Workflows: Read and write** → re-request installation for `Warzonesiddiki/winforge`.

**Drop-in fix ready**: `ci.yml.fixed` at the repo root — copy to `.github/workflows/ci.yml` once permission exists (it builds the WPF project on windows-latest and runs web checks). Will be replaced by a hybrid CI (Zig + Bun builds) as part of the migration.

## BLK-4 — Electron runtime binaries unreachable

**What fails**: Electron download hosts (`github.com/*/releases/download/*` binary redirects → `objects.githubusercontent.com`, and `npmmirror.com/mirrors/electron`) unreachable.

**Impact**: Electron cannot be installed or packaged here. Combined with node-gyp being blocked (native modules), Electron is removed from consideration (heaviest option anyway).

## BLK-5 — nodejs.org blocked (official Node downloads)

**What fails**: `nodejs.org/dist` unreachable.

**Workaround (verified)**: npm registry works, and `node-win-x64` + `postject` npm packages exist — Node SEA (single-executable) packaging is available if needed. Bun (also via npm) is the primary runtime and already cross-compiles Windows EXEs (verified).

## BLK-6 — No Windows runtime for behavioral tests

**What fails**: Sandbox is Linux-only; cannot execute Windows binaries, UAC elevation, real registry writes, Appx removal, DISM, winget.

**Mitigations**:
- Zig core compiles the *same source* to a Linux `.so`; platform-neutral logic (operation scheduling, JSON snapshots, undo stack, health scoring, audit writes) is exercised locally through the Bun FFI bridge (verified working).
- Windows-only paths are isolated behind the core API and covered by a manual checklist to run on a real Windows machine:
  1. UAC/elevation prompt on launch
  2. `SRSetRestorePointW` creates restore point before mutation
  3. Tweak apply writes HKLM/HKCU value and verify passes; undo reverts it
  4. Protected paths (SAM/SECURITY/SafeBoot) refused
  5. Appx remove/readd for one package; winget install/uninstall for one app
  6. Health score matches manual count of applied tweaks

## BLK-7 — WPF Phase 1 code cannot be compiled

**What exists**: 25 C# + 8 XAML files (registry/restore/PowerShell/tweak/privacy/debloat/software/preset/health services, 6 MVVM views) — statically verified only (parse, SQL, bindings).

**Why blocked**: BLK-1 (no SDK locally) + BLK-3 (no CI).

**Status**: **Archived, not deleted.** Retained as reference implementation for the hybrid port (every service maps 1:1 to a Zig core module or a PS recipe) and as an alternate UI. `BUILD_STATUS.md` continues to document its exact verified state.

---

## BLK-8 — Alternative language toolchains evaluated and excluded (2026-08-16)

During the v2 language-selection research (see [LANGUAGE_SELECTION.md](./LANGUAGE_SELECTION.md)) the following toolchains were probed and found **unreachable from this sandbox** (TCP/SSL failure on their distribution hosts). Each is excluded on evidence, not preference:

| Language | Host(s) blocked | Notes |
|---|---|---|
| Rust | static.rust-lang.org, crates.io | No rustup, no crates |
| Deno | dl.deno.land | — |
| Flutter / Dart | storage.googleapis.com | No SDK download |
| Java / Kotlin | api.adoptium.net, repo.maven.apache.org | No JDK, no Maven |
| Free Pascal / Lazarus | downloads.freepascal.org | Would otherwise be an excellent fit (native, tiny EXEs, Win32-rich) |
| Nim | nim-lang.org | — |
| Tcl | tcl.tk | — |
| Electron | github release CDN, npmmirror | see BLK-4 |
| .NET | dot.net, nuget.org, azureedge | see BLK-1 |

**Positive discoveries from the same sweep** (verified alternatives that unblock these roles):

| Need | Blocked path | Verified alternative |
|---|---|---|
| Native core compiler | Rust / FPC / Nim | **Zig 0.16 via pypi `ziglang`** — cross-compiles Windows EXE/DLL + Linux .so from this sandbox |
| Plugin scripting | lua.org (blocked) | **Lua source on github.com/lua/lua** — compiled to a 483 KB Windows DLL with `zig cc`, in-sandbox |
| Plugin sandbox | — | **wasmtime 47.0.1 via pypi wheel** — extracted a valid `_wasmtime.dll` Windows PE |
| C/C++ build system | apt (blocked) | **cmake 4.4 + ninja 1.13 via pypi** |
| Packaging fallback | nodejs.org (blocked) | **npm `node-win-x64` + `postject`** (Node SEA) |
| Small-EXE compression | pypi `upx` (absent) | Deferred — UPX source-build possible via cmake+ninja+zig c++ if needed |

## Reachability reference (probed 2026-08-16)

| Host | Status | Used for |
|---|---|---|
| registry.npmjs.org | ✅ 200 | Node/Bun tooling |
| pypi.org / files.pythonhosted.org | ✅ 200 | Python tooling, **Zig via `ziglang` wheel**, **wasmtime wheels**, cmake, ninja |
| github.com (API, codeload) | ✅ | git, gh, source tarballs (Lua, wasm3, zigwin32, Go engine branch) |
| dot.net, nuget, azureedge, builds.dotnet.microsoft.com | ❌ | .NET |
| go.dev, dl.google.com, proxy.golang.org | ❌ | Go |
| nodejs.org | ❌ | Node official |
| deb.debian.org | ❌ | apt |
| electron mirrors (objects.github, npmmirror) | ❌ | Electron |
| static.rust-lang.org, crates.io | ❌ | Rust |
| dl.deno.land | ❌ | Deno |
| storage.googleapis.com | ❌ | Flutter/Dart |
| api.adoptium.net, repo.maven.apache.org | ❌ | Java/Kotlin |
| downloads.freepascal.org | ❌ | Free Pascal/Lazarus |
| nim-lang.org | ❌ | Nim |
| tcl.tk, lua.org | ❌ | Tcl / Lua official FTP (Lua GitHub mirror works instead) |
