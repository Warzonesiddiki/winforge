# WinForge — WASM Plugin Sandbox (Phase 4 design + verified spike)

- **Status:** Design accepted; spike executed and verified 2026-08-16.
  Not yet a product feature — this document is the Phase 4 blueprint.
  **Re-scoped 2026-08-16:** see
  [`WASM_REALSCOPE_2026-08-16.md`](./WASM_REALSCOPE_2026-08-16.md) for the
  evidence-based decision not to ship an unverified Windows-only binding in
  the W1 session (Lua is the first scriptable tier); the target is unchanged.
- **Relationship to Lua packs:** Lua (`native/build-lua.sh`, `lua54.dll`) is
  the *convenient* plugin tier for trusted community packs. WASM is the
  *strong-isolation* tier for third-party packs that must not be able to touch
  anything except the capabilities WinForge explicitly hands them.

## Why WASM for hostile plugins

A Lua state runs in-process with a whitelisted API, but a malicious C module
or an interpreter exploit escapes it. A WASM guest under wasmtime cannot:

- see the filesystem, network, environment, or clock unless a host function
  provides them (deny-by-default linear memory sandbox);
- call anything except the imports the host chose to link;
- exceed fuel/epoch limits the host configures (CPU bounding).

That is exactly the trust model WinForge needs for "install a pack from the
internet": the pack can *propose* tweaks through a narrow API; the engine
still validates and executes them through the same orchestrator paths (audit,
undo, protected services, elevation boundary) as everything else.

## Verified facts (all executed in the sandbox, 2026-08-16)

1. **Runtime availability, Linux (dev/test):** `pip install wasmtime` works
   (pypi reachable). A `.wat` module importing a host function
   (`winforge.health_score`) and exporting `classify()` was compiled,
   instantiated, and executed:
   `classify(health=87) == 2`, `classify(health=42) == 0`. The spike proves
   host-capability injection: the same module linked against two different
   host closures returns different results.
2. **Runtime availability, Windows (ship):** the
   `wasmtime-47.0.1-py3-none-win_amd64` wheel on pypi ships
   `wasmtime/win32-x86_64/_wasmtime.dll` — a valid PE (`MZ` header verified).
   The C API DLL can be loaded from Go via the same `winapi.SystemDLL`-style
   pattern used elsewhere (with a WinForge-controlled absolute path, not the
   DLL search path).
3. **Guest toolchains:** guests are compiled *elsewhere* (TinyGo, Rust
   `wasm32-unknown-unknown`, or hand-written WAT). The sandbox cannot build
   Rust/TinyGo (blocked hosts), which is fine: WinForge only needs to *run*
   `.wasm`, never to build it.

Spike source (reproduce anytime):

```wat
(module
  (import "winforge" "health_score" (func $health_score (result i32)))
  (func (export "classify") (result i32)
    (if (result i32) (i32.ge_s (call $health_score) (i32.const 80))
      (then (i32.const 2))
      (else
        (if (result i32) (i32.ge_s (call $health_score) (i32.const 50))
          (then (i32.const 1))
          (else (i32.const 0)))))))
```

```bash
python3 -m venv /tmp/w && /tmp/w/bin/pip install wasmtime
/tmp/w/bin/python - <<'PY'
from wasmtime import Store, Module, Instance, Func, FuncType, ValType, Engine
engine = Engine(); store = Store(engine)
module = Module(engine, open('plugin.wat').read())
health = Func(store, FuncType([], [ValType.i32()]), lambda: 87)
print(Instance(store, module, [health]).exports(store)["classify"](store))  # 2
PY
```

## Host API design (capability-limited, mirrors the Lua plan)

The guest gets ONLY these imports; every one funnels into the same validated
Go APIs the orchestrator uses (never raw registry/SCM access):

| Import | Signature (wasm) | Backing Go call |
|---|---|---|
| `winforge.health_score` | `() -> i32` | `app.Health(...).Score` |
| `winforge.tweak_is_applied` | `(id_ptr, id_len) -> i32` | `orchestrator.IsApplied` |
| `winforge.propose_registry_set` | `(json_ptr, json_len) -> i32` | build a `config.Operation`, run through `validateOperation` + orchestrator apply with audit/undo |
| `winforge.log` | `(ptr, len)` | bounded audit/log write |

Rules:

- All strings crossing the boundary are length-prefixed, bounded by the same
  `limits.go` caps as catalog JSON, and validated before use.
- Proposals from a guest NEVER bypass: protected services, the elevated
  command allowlist, hive/path validation, or reversibility requirements.
- Elevated engine processes refuse WASM plugins entirely, exactly like the
  existing plugin-directory rule (UAC boundary).
- Fuel metering on: a plugin that burns its fuel is terminated, not trusted.

## Integration path (when Phase 4 starts)

1. Vendor the wasmtime **C API** DLL/so next to the plugin runtime (Windows:
   extract from the pypi wheel or the GitHub release once reachable; Linux
   dev: the pypi wheel).
2. Add `internal/plugin/wasm.go` behind a build tag: cgo-free binding via
   `syscall.NewLazyDLL` on Windows (the C API is a flat C ABI) — keeping the
   stdlib-only rule for the default build; the WASM tier is an optional
   add-on discovered at runtime (`wasmtime.dll` present → tier enabled).
3. Manifest extension: `manifest.json` gains `"type": "wasm"` +
   `"module": "pack.wasm"`; `MergeTweaks` stays unchanged (proposals surface
   as ordinary tweaks tagged with their plugin origin).
4. Test matrix (Linux, in-sandbox): happy path, out-of-fuel guest, guest that
   requests an unknown import (must fail to link), guest that passes an
   over-long string (must be rejected by bounds), guest proposing a protected
   service change (must be refused by the same guard tests as the catalog).
