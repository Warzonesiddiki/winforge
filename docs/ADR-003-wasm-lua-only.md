# ADR-003: WASM Plugin Tier — Ship Validation Only, Defer Windows C Host

- **Status:** Accepted
- **Date:** 2026-08-16
- **Deciders:** WinForge maintainers

## Context

The plugin system has two tiers:

1. **Lua W1** — shipped. A cgo-free `syscall.LoadDLL` host binds
   `lua54.dll` next to the executable, with a 10M-instruction runaway
   budget, an absolute-path DLL lookup, and a closed operation
   whitelist. 33 tests cover happy paths, hostile scripts, and runaway
   detection.
2. **WASM W2** — platform-independent proposal/validation is implemented
   and tested (`wasm.go` + 30 tests with a fake host), but the Windows
   production host that loads `_wasmtime.dll` and executes real modules
   is a stub that returns `ErrWasmUnavailable`.

Shipping the WASM C host would require binding approximately 30–35
wasmtime C API functions (`wasmtime_config_new`, `wasmtime_store_new`,
`wasmtime_linker_define_func`, `wasmtime_func_call`, trap inspection,
value-type conversions, etc.) plus four `syscall.NewCallback` host
imports. This code lives in a security boundary: a bug in the host
could let a malicious WASM module escape the sandbox or corrupt the
engine process.

The sandbox is Linux-only for CI (no Windows runners with real
wasmtime), and there is no Windows runtime tester on the project
(BLK-6). Shipping an unverified ~35-function FFI binding in a security
boundary would contradict the project's zero-compromise stance.

## Decision

The WASM tier ships as **validation only** in current releases:

- `wasm.go` (proposal/validation) is compiled and tested on every
  platform.
- `wasm_windows.go` returns `ErrWasmUnavailable` with a clear message.
- Lua is the only scripting tier that executes third-party packs.
- The example WASM pack in `examples/plugins/example-wasm-pack/` remains
  for future contributors and for the fake-host tests.

## Consequences

- **Positive:** No unverified code in a security boundary ships to
  users. The validation layer prevents malformed WASM packs from being
  loaded, so future activation is a drop-in.
- **Negative:** Authors cannot write plugins in WASM yet. The
  "WebAssembly sandbox" feature is partial.
- **Path to activation:** A Windows-native contributor implements
  `wasmtime_windows.go` against a pinned wasmtime release, adds
  integration tests that execute the example pack on Windows, runs the
  §13 smoke checklist, and updates this ADR to "Implemented".

## References

- `docs/WASM_PLUGIN_SANDBOX.md` — design
- `docs/WINDOWS_SMOKE_CHECKLIST.md` §13 — WASM runtime tests
- `internal/plugin/wasm.go`, `internal/plugin/wasm_windows.go`
- `examples/plugins/example-wasm-pack/`
