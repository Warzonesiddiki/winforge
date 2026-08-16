# WinForge — Lua Plugin Integration (verified artifacts + honest plan)

- **Status:** runtime artifacts built and executed (2026-08-16); the in-engine
  Go binding is designed but deliberately not yet landed — see "The cgo
  tension" below for the reason.

## Verified today (all by execution, in-sandbox)

1. **`native/build-lua.sh`** builds Lua 5.4.7 from the github.com/lua/lua
   `v5.4.7` tag with `zig cc`:
   - `native/out/liblua54.so` — Linux ELF shared object (1.3 MB)
   - `native/out/lua54.dll` — Windows PE32+ DLL (456 KB, `MZ` verified,
     built with `-DLUA_BUILD_AS_DLL -target x86_64-windows-gnu`)
   Artifacts are gitignored (`native/out/`); the script is the committed,
   reproducible artifact.
2. **The `.so` actually runs Lua**: loaded via ctypes,
   `luaL_newstate → luaL_openlibs → luaL_loadstring("return 6*7") →
   lua_pcallk → lua_tonumberx` returned **42**. The C ABI surface WinForge
   needs (state, load, pcall, tonumber/tostring, push*) is confirmed present
   and working in our build.

## The cgo tension (why the Go binding is not landed yet)

The engine ships as a **stdlib-only, CGO_ENABLED=0** static binary — that is
a core architectural guarantee (BLK-2 resolution depends on it; the toolchain
bootstrap and CI assume it).

- **Windows:** binding `lua54.dll` needs **no cgo** — `syscall.NewLazyDLL` +
  `NewProc` call C ABI functions directly (the same pattern as
  `internal/registry` and `internal/winapi`). This is the shipping path.
- **Linux:** there is no stdlib `dlopen`; loading `liblua54.so` from Go
  requires cgo (or a third-party purego dependency, which violates
  stdlib-only). So the handover's "tests run on Linux against the .so"
  cannot be satisfied *inside* `go test` without breaking either
  CGO_ENABLED=0 shipping or the no-third-party-modules rule.

**Decision:** do not compromise the stdlib-only guarantee.

## Landing plan (next session; Windows-first, sandbox-verifiable pieces first)

1. `internal/plugin/lua_windows.go` (build tag `windows`): LazyDLL binding to
   `lua54.dll` looked up ONLY next to the executable or in the WinForge data
   dir (never the DLL search path). Sandbox verification: `GOOS=windows go
   vet` + `go test -c` compile it; behavioral verification goes on the
   Windows smoke checklist (docs/WINDOWS_SMOKE_CHECKLIST.md).
2. Whitelisted API surface exposed to scripts (identical to the WASM tier's
   philosophy — scripts propose, the engine validates and executes):
   - `winforge.registry.set(hive, path, name, kind, value)`
   - `winforge.registry.delete(hive, path, name)`
   - `winforge.service.set_start_mode(name, mode)`
   - `winforge.log(message)`
   Every call builds a `config.Operation` and passes through
   `validateOperation` + the orchestrator (audit, undo, protected services,
   elevation rules). Lua NEVER reaches the elevated command executor.
3. Manifest extension: `manifest.json` `"type": "lua"`, `"script":
   "pack.lua"`. Elevated processes ignore Lua plugins (same UAC rule as
   JSON plugins today).
4. Linux-side logic tests without cgo: the *API surface* (operation
   construction, validation, bounds, hostile-input rejection) is plain Go and
   fully testable with a fake script host; only the thin LazyDLL layer is
   Windows-only, mirroring how `internal/registry` is tested today.
5. Hostile-script tests (Windows checklist + compiled tests): bad hive,
   oversized strings, protected service, disallowed API name, infinite loop
   (instruction-count hook `lua_sethook(LUA_MASKCOUNT)` → abort).
