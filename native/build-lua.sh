#!/usr/bin/env bash
# Builds Lua 5.4.7 as a shared library for the WinForge Lua plugin runtime:
#   out/liblua54.so   — Linux, used by the engine's Lua tests in the sandbox
#   out/lua54.dll     — Windows, shipped next to winforge.exe for Lua packs
#
# Source: github.com/lua/lua tag v5.4.7 (codeload tarball — reachable from the
# sandbox). Compiler: zig cc (install: python3 -m venv /tmp/zig &&
# /tmp/zig/bin/pip install ziglang; binary = /tmp/zig/bin/python-zig).
#
# Both builds were verified in-sandbox on 2026-08-16:
#   liblua54.so: ELF 64-bit LSB shared object, x86-64
#   lua54.dll:   PE32+ DLL, x86-64
set -euo pipefail

cd "$(dirname "$0")"

ZIG="${ZIG:-python-zig}"
LUA_TAG="${LUA_TAG:-v5.4.7}"
SRC="${LUA_SRC:-/tmp/lua-build/lua}"
OUT="out"
mkdir -p "$OUT"

if [ ! -d "$SRC" ]; then
  mkdir -p "$(dirname "$SRC")"
  curl -sSL "https://codeload.github.com/lua/lua/tar.gz/refs/tags/${LUA_TAG}" \
    -o /tmp/lua-build/lua.tar.gz
  tar xzf /tmp/lua-build/lua.tar.gz -C /tmp/lua-build
  mv "/tmp/lua-build/lua-${LUA_TAG#v}" "$SRC"
fi

# Core + libraries, excluding the standalone interpreter (lua.c) and the
# combined-build helper (onelua.c). This matches the upstream makefile's
# CORE_O/LIB_O object list.
mapfile -t SOURCES < <(ls "$SRC"/*.c | grep -v -e '/lua\.c$' -e '/onelua\.c$')

echo "==> Linux shared library (${#SOURCES[@]} translation units)"
"$ZIG" cc -shared -O2 -fPIC -DLUA_USE_LINUX \
  -o "$OUT/liblua54.so" "${SOURCES[@]}" -lm -ldl

echo "==> Windows DLL"
"$ZIG" cc -shared -O2 -DLUA_BUILD_AS_DLL \
  -target x86_64-windows-gnu \
  -o "$OUT/lua54.dll" "${SOURCES[@]}"

echo "==> Artifacts:"
file "$OUT/liblua54.so" "$OUT/lua54.dll" 2>/dev/null || ls -la "$OUT"
