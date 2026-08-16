# Archive

This directory holds code that is no longer part of the active product but is
kept for historical reference. Nothing here is built, tested, or shipped.

| Path | What it was | Why archived |
|------|-------------|--------------|
| [`WinForge.Elite/`](WinForge.Elite/) | WPF/.NET 8 Phase 1 desktop scaffold (~5,400 lines C#/XAML) | The product went Go-primary (see `docs/LANGUAGE_SELECTION.md`). The .NET toolchain was unreachable from the build sandbox (BLK-1). The scaffold is preserved as a reference for a future native UI. |
| [`runtime/`](runtime/) | Bun FFI bridge scaffold + `test_core.ts` | Experiment to call a Zig core from Bun. Never integrated; the Go engine is stdlib-only. |
| [`native/core.zig`](native/core.zig), [`native/build.sh`](native/build.sh) | Zig "core" companion library + build script | Same abandoned Bun/Zig experiment. **`native/build-lua.sh` is NOT archived** — it is actively used to build `lua54.dll` for the Lua plugin tier and remains in `native/`. |
| [`ci/`](ci/) | Staging copy of `github-actions-ci.yml` plus a README | The live workflow now lives at `.github/workflows/ci.yml`; the improved drop-in is `ci.yml.fixed` at the repo root. This copy was identical to the live one and is no longer needed. |

To revive anything here, `git mv` it back to its original location and update
the references in `AGENTS.md`, `README.md`, and `docs/`.
