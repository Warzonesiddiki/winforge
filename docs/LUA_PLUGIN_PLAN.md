# WinForge — Lua Plugin Integration (binding LANDED 2026-08-16)

- **Status:** The in-engine Go binding is implemented and sandbox-verified
  (Linux unit tests + GOOS=windows `go vet`/`go test -c` + valid 6.74 MB
  winforge.exe). Runtime behavior on Windows remains pending on BLK-6 (see
  `docs/WINDOWS_SMOKE_CHECKLIST.md` §11). The pre-built artifacts from
  `native/build-lua.sh` (liblua54.so executed 6*7=42; lua54.dll is a valid
  PE) remain the prerequisite for the Windows runtime.

## What shipped

### Files

- `internal/plugin/lua.go` — platform-independent core: the whitelisted
  proposal API (`luaAPI`), strict operation construction through
  `config.ValidateOperationForPlugin`, a closed plugin op-type whitelist
  (registry dword/string/qword set, registry value delete, service
  start-mode), and the `ScriptHost` interface.
- `internal/plugin/lua_windows.go` — `//go:build windows` host: cgo-free
  binding to `lua54.dll` via `syscall.LoadDLL` + `syscall.NewCallback`,
  absolute-path DLL lookup (exe dir then data dir), the `winforge.*`
  function table, removal of `os`/`io`/`debug`/`package`/`loadfile`/
  `load`/`loadstring`/`require`/`dofile`, `print` redirected to the bounded
  log, and an instruction-count hook (`lua_sethook LUA_MASKCOUNT`, budget
  10,000,000) that aborts runaway scripts.
- `internal/plugin/lua_other.go` — non-Windows stub returning
  `ErrLuaUnavailable` (the platform-independent logic is still tested on
  Linux via a fake `ScriptHost`).
- `internal/plugin/plugin.go` — manifest gains `"type":"lua"` +
  `"script":"pack.lua"` (default `pack.lua`, must be a bare file name);
  `DiscoverWithOptions` carries `LuaDLLDirs`; Lua plugins route through
  `loadLuaPlugin` → host.Run → strict `TweakConfig.Validate()`.
- `internal/app/app.go` — passes the exe directory and data directory as the
  two Lua DLL search locations; elevated processes still ignore the whole
  plugins dir (UAC boundary).
- `internal/config/models.go` — exports
  `ValidateOperationForPlugin` (the strict-loader per-op validator).
- `examples/plugins/example-lua-pack/` — documented sample pack
  (`manifest.json` + `pack.lua`).
- Tests: `internal/plugin/lua_test.go` (proposal builders + every hostile
  case) and `internal/plugin/plugin_test.go` (manifest type routing,
  unavailable/hostile/fake-host discovery).

### The Lua API

```lua
local t = winforge.tweak{
  id          = "my-tweak",
  name        = "My Tweak",
  category    = "Privacy",
  description = "...",
  risk        = "low",          -- low|medium|high
  reversible  = true,
}
local set = winforge.registry.set("HKCU", "Software\\MyKey", "Value",
                                  "dword", 1)   -- kind: dword|qword|string
local del = winforge.registry.delete("HKCU", "Software\\MyKey", "Value")
local svc = winforge.service.set_start_mode("Fax", "disabled")
winforge.revert(del)            -- add an op handle to the tweak's revert list
winforge.log("proposed")        -- bounded; print() is aliased to this
t:commit()
```

Every proposed operation is built into a `config.Operation` and run through
the strict loader's `validateOperation` — bad hives, shallow
`registry_delete_key` paths, oversized strings, DWORD/QWORD range, unknown
service modes, and unknown op types are rejected at proposal time. A
separate plugin op-type whitelist forbids `command`, `appx_remove`,
`task_*`, `power_*`, `netbios`, and `registry_delete_key`: scripts can only
propose the safe subset. Protected-service and malformed-name enforcement
happens at apply time through the same engine guard used by catalog tweaks.

## The cgo tension (respected, not relitigated)

The engine ships **stdlib-only, CGO_ENABLED=0**. On Windows,
`syscall.NewLazyDLL`/`syscall.LoadDLL` + `syscall.NewCallback` call the C
ABI directly — no cgo. On Linux there is no stdlib `dlopen`, so loading
`liblua54.so` would require cgo (or a third-party purego dependency), which
violates the stdlib-only guarantee. Therefore:

- The **platform-independent logic** (API surface, operation construction,
  validation, bounds, hostile-input rejection, manifest routing) is plain Go
  and fully unit-tested on Linux with a fake `ScriptHost`.
- The **thin Windows binding** is verified by `GOOS=windows go vet` +
  `go test -c`; behavioral items are in `docs/WINDOWS_SMOKE_CHECKLIST.md`
  §11.

## Longjmp note (Windows)

`lua_error` performs a `longjmp` to the protected-call frame inside
lua54.dll. When invoked from a Go `syscall.NewCallback`, the Go frame is
unwound without running its defers. This is safe here because the callbacks
hold no Go locks or heap resources across the raise (`stateMu` is released
before returning into Lua), and `Run` locks the goroutine to one OS thread
so any abandoned stack stays within that thread; after `pcall` returns,
normal Go control flow resumes and `lua_close` runs. This is explicitly
exercised on the Windows smoke checklist (item 11.8), including a
post-abort stability check.

## Verified in-sandbox (2026-08-16)

- `gofmt -l .` clean · `go vet ./...` clean (linux + windows) · all 18
  test packages green incl. `-race` (24 plugin tests).
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build` → `winforge.exe`
  6,738,944 bytes, valid `MZ` PE.
- `GOOS=windows GOARCH=amd64 go test -c ./internal/plugin` compiles.
- The example pack's Lua is validated by the same `TweakConfig.Validate()`
  path used for JSON plugins (covered by the fake-host discovery test).
