# WinForge — W2 (WASM plugin tier) re-scope decision (2026-08-16)

- **Status:** Re-scoped with evidence, not silently deferred. The Phase 4
  design in `docs/WASM_PLUGIN_SANDBOX.md` remains the target; this note
  records why it was not implemented in the 2026-08-16 session that landed
  W1 (Lua) and what would unblock it.

## What was re-evaluated

The Lua binding (W1) is small and cgo-free on Windows: ~25 C functions
(`luaL_newstate`, `lua_pcallk`, stack push/get, `lua_sethook`, …), each a
trivial ABI call, with one class of callback (`lua_CFunction`). It was
landed with the platform-independent logic unit-tested on Linux and the thin
Windows binding verified by `GOOS=windows go vet` + `go test -c` (runtime
behavior is BLK-6).

The wasmtime **C** API is a different order of problem. Measured against the
`wasmtime 47.0.1` wheel already used for the spike:

- The shared library is **~28 MB** (`_libwasmtime.so`; the Windows
  `_wasmtime.dll` is comparable), vs. lua54.dll at ~456 KB.
- The wheel exposes **980 callable C symbols**. Even the *minimal* subset
  needed for the documented host API (engine + fuel config + store + module +
  linker + `define_func` + instantiate + export lookup + func call + memory
  data + trap/error message + byte/val/name vectors) is **~30–35 functions**,
  plus opaque handle-lifetime rules (`*_delete` ownership,
  `wasm_val_vec_t`, `wasm_byte_vec_t`, `wasm_trap_t`, `wasmtime_error_t`) and
  at least two callback signatures (`wasm_func_callback_t`, the store fuel
  deadline is not needed).
- Host functions require a **C→Go callback**. On Windows that is
  `syscall.NewCallback` (same mechanism W1 uses). On Linux there is no
  stdlib `dlopen` and no stdlib C→Go callback mechanism; both require cgo,
  which the engine's `CGO_ENABLED=0` / stdlib-only guarantee rules out. So,
  exactly like Lua, the WASM host would be **Windows-only at runtime**, with
  platform-independent logic (proposal building, validation, bounds) the
  only part testable on Linux.

## Why it was not shipped this session

1. **Verification mandate.** WinForge's operating protocol is "verify by
   execution; no fabrication." A 30+ function C binding with manual handle
   ownership and memory reads across the wasmtime linear memory cannot be
   made trustworthy from `GOOS=windows go vet` alone — the Lua binding's
   ABI surface is simple enough that compile + code-reading is a reasonable
   stand-in for BLK-6, but the WASM binding has enough lifetime and
   struct-layout detail that an unexercised path would be a latent crash or
   memory-safety bug in a *security-boundary* component (the whole point of
   WASM is strong isolation). Shipping it without a Windows execution would
   violate the project's own standard.
2. **No in-sandbox Windows runtime (BLK-6).** The Lua runtime at least has
   the prior session's `6*7=42` execution of `liblua54.so` proving the C
   ABI surface works. There is no equivalent cgo-free path to *execute* the
   WASM binding in this sandbox, and Linux cgo is off the table.
3. **Scope/risk vs. the rest of the roadmap.** W1 (now merged) already
   provides a scriptable community-pack tier with the same "propose →
   validate → orchestrate" model. WASM's incremental value is stronger
   isolation for *untrusted* packs; the engine already enforces the proposal
   whitelist, op-type restrictions, and the UAC boundary for Lua. The next
   highest-value, fully sandbox-verifiable work is W3 (UI↔engine bridge
   phase 2 with an auth design).

## What is required to ship W2 (unchanged target)

The design in `docs/WASM_PLUGIN_SANDBOX.md` stands. To implement it without
compromising stdlib-only / `CGO_ENABLED=0`:

1. A Windows-only `internal/plugin/wasm_windows.go` using
   `syscall.LoadDLL` (absolute path next to the exe / data dir, like the Lua
   host) binding the ~35-function subset, plus `wasm_other.go` returning
   `ErrWasmUnavailable`.
2. Manifest `"type":"wasm"` + `"module":"pack.wasm"`; runtime discovered
   when the DLL is present, skipped best-effort otherwise (same UX as Lua).
3. Host imports `winforge.health_score`, `winforge.tweak_is_applied`,
   `winforge.propose_registry_set`, `winforge.log` — each funneling through
   the SAME `config.ValidateOperationForPlugin` + plugin op-type whitelist +
   orchestrator paths used by Lua. Linear-memory strings bounded by
   `limits.go`.
4. Fuel metering on (`wasmtime_config_consume_fuel_set` +
   `wasmtime_context_set_fuel`); out-of-fuel terminates the guest.
5. Platform-independent proposal/validation logic unit-tested on Linux with
   a fake `WasmHost`; the Windows binding verified by `GOOS=windows go vet`
   + `go test -c`, AND **executed on a real Windows box** (extend
   `docs/WINDOWS_SMOKE_CHECKLIST.md`) before merge — that is the gate this
   session could not satisfy.

## Decision

W2 is **re-scoped, not abandoned**: no unverified WASM binding lands. The
next session should either (a) implement it on a Windows-capable runner /
real hardware where the binding can be executed and crash/fuel/memory cases
proven, or (b) explicitly decide to accept Lua as the only scriptable tier
and close W2. The W3 bridge work proceeds regardless.
## Update 2026-08-16 (arena/01a00ac0-winforge) — platform-independent tier LANDED

The decision above is **unchanged** (no unverified Windows C binding ships).
What changed: the platform-independent half of item 5 in "What is required"
has been landed as a sandbox-verifiable commit:

- `internal/plugin/wasm.go` (whitelisted proposal API + strict validation +
  op whitelist + `validateWasmModule` magic/version/size checks),
  `wasm_other.go` (non-Windows `ErrWasmUnavailable`), `wasm_windows.go`
  (DLL-search stub for `wasmtime.dll` by absolute path, deliberately returns
  `ErrWasmUnavailable` until verified — honest about the missing binding),
  `wasm_test.go` (24 wasmAPI hostile-case tests + 6 discovery tests with a
  fake `WasmHost`), and `examples/plugins/example-wasm-pack/` (8-byte minimal
  wasm + `pack.wat.example` spike). Windows `go vet` + `go test -c` + 6.5 MB PE
  clean; 18/18 packages `go test -race` green.

The remaining gate is exactly as described: the ~30–35-function Windows C
binding with fuel + linear-memory + callback handling, which still requires
a Windows runner and a §13 smoke execution before it can merge. The checklist
section was added in this session as §13 (WASM DLL-search stub verification).

No fabrication, no unverified security-boundary code ships; the strong-isolation
tier now has a tested, bounded proposal surface ready for the future host.
