#!/usr/bin/env bash
# Builds the WinForge Zig core for local testing (Linux .so) and Windows shipping (.exe + .dll).
#
# Requirements:
#   - Zig ≥ 0.14 (sandbox: `python3 -m venv /tmp/zig && /tmp/zig/bin/pip install ziglang`
#     → binary available as `python-zig`; set ZIG=/path/to/python-zig if it is not on PATH)
set -euo pipefail

cd "$(dirname "$0")"

ZIG="${ZIG:-python-zig}"
OUT="out"
mkdir -p "$OUT"

echo "==> Linux shared library (local FFI testing)"
"$ZIG" build-lib core.zig -dynamic -O ReleaseSafe -femit-bin="$OUT/libwinforge_core.so"

echo "==> Windows DLL (embedded in the Bun EXE)"
"$ZIG" build-lib core.zig -dynamic -target x86_64-windows-gnu -O ReleaseSafe -femit-bin="$OUT/winforge_core.dll"

echo "==> Windows EXE (standalone core CLI)"
"$ZIG" build-exe core.zig -target x86_64-windows-gnu -O ReleaseSafe -femit-bin="$OUT/winforge-core.exe"

echo "==> Artifacts:"
ls -la "$OUT"
