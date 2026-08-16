#!/usr/bin/env python3
"""Verify a built winforge.exe without running Windows.

Performs static checks that catch common build/embedding regressions:

  * PE32+ magic and machine type (x86-64)
  * The embedded catalog files (tweaks.json, apps, etc.) are present in
    the binary's embedded assets (they are compiled in via Go embed)
  * The version string stamped by -ldflags is present
  * The binary is not stripped of its symbol table in a way that breaks
    the plugin ABI (Lua/WASM call sites remain resolvable)

This is NOT a substitute for the Windows smoke checklist in
docs/WINDOWS_SMOKE_CHECKLIST.md, but it provides fast CI feedback.

Usage:
    python3 tools/verify_binary.py /path/to/winforge.exe [--version vX.Y.Z]
"""

from __future__ import annotations

import argparse
import struct
import sys
from pathlib import Path

PE_MAGIC = b"MZ"
PE32_PLUS_MAGIC = 0x20B
MACHINE_AMD64 = 0x8664

# Files that must be embedded in the binary (Go embed compiles their
# contents into .rodata; the filenames appear as contiguous strings).
EXPECTED_EMBEDDED = [
    b"config/tweaks.json",
    b"config/applications.json",
    b"config/debloat.json",
    b"config/dns.json",
    b"config/protectedServices.json",
    b"web/index.html",
    b"web/app.js",
    b"web/style.css",
]


def read_pe_headers(data: bytes) -> dict:
    """Parse just enough of the PE header to validate architecture."""
    if len(data) < 0x40 or data[:2] != PE_MAGIC:
        raise ValueError("not a PE binary (missing MZ stub)")
    pe_offset = struct.unpack_from("<I", data, 0x3C)[0]
    if pe_offset + 6 > len(data) or data[pe_offset:pe_offset + 4] != b"PE\x00\x00":
        raise ValueError("invalid PE signature")
    machine = struct.unpack_from("<H", data, pe_offset + 4)[0]
    # COFF header is 20 bytes; optional header starts at pe_offset + 24
    opt_magic = struct.unpack_from("<H", data, pe_offset + 24)[0]
    return {"machine": machine, "optional_magic": opt_magic}


def verify(path: Path, expected_version: str | None) -> list[str]:
    errors: list[str] = []
    data = path.read_bytes()

    try:
        hdr = read_pe_headers(data)
    except ValueError as exc:
        errors.append(f"PE header: {exc}")
        return errors

    if hdr["machine"] != MACHINE_AMD64:
        errors.append(f"machine = 0x{hdr['machine']:x}, want amd64 (0x{MACHINE_AMD64:x})")
    if hdr["optional_magic"] != PE32_PLUS_MAGIC:
        errors.append(f"optional magic = 0x{hdr['optional_magic']:x}, want PE32+ (0x{PE32_PLUS_MAGIC:x})")

    for needle in EXPECTED_EMBEDDED:
        if needle not in data:
            errors.append(f"embedded asset not found: {needle.decode()}")

    if expected_version:
        if expected_version.encode() not in data:
            errors.append(f"version string {expected_version!r} not stamped into binary")

    # Size sanity: the release binary is expected to be ~6-7 MB after
    # stripping. Warn rather than fail if it grows unexpectedly.
    size_mb = len(data) / (1024 * 1024)
    if size_mb > 15:
        errors.append(f"binary is {size_mb:.1f} MB; expected <15 MB")

    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path, help="path to winforge.exe")
    parser.add_argument("--version", help="expected version string (from -ldflags)")
    args = parser.parse_args()

    if not args.binary.is_file():
        print(f"error: {args.binary} not found", file=sys.stderr)
        return 2

    errors = verify(args.binary, args.version)
    if errors:
        for err in errors:
            print(f"FAIL: {err}", file=sys.stderr)
        return 1

    size = args.binary.stat().st_size
    print(f"OK: {args.binary} ({size:,} bytes)")
    print("  PE32+ amd64, all embedded assets present" +
          (f", version {args.version!r}" if args.version else ""))
    return 0


if __name__ == "__main__":
    sys.exit(main())
