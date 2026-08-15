import { dlopen, FFIType, ptr } from "bun:ffi";
import { existsSync } from "node:fs";

// FFI bridge test: loads the Zig core (.so on Linux / .dll on Windows) and
// asserts the exported C-ABI functions behave per the product algorithm.
// Run:  bun runtime/test_core.ts
// The core library is produced by native/build.sh.

const libName = process.platform === "win32" ? "winforge_core.dll" : "libwinforge_core.so";
const libPath = new URL(`../native/out/${libName}`, import.meta.url).pathname;

if (!existsSync(libPath)) {
  console.error(`Core library not found at ${libPath} — run native/build.sh first.`);
  process.exit(1);
}

const lib = dlopen(libPath, {
  winforge_core_version: { args: [FFIType.ptr, FFIType.usize], returns: FFIType.usize },
  winforge_health_score: {
    args: [FFIType.i32, FFIType.i32, FFIType.f64, FFIType.bool],
    returns: FFIType.i32,
  },
});

const buf = Buffer.alloc(128);
const len = Number(lib.symbols.winforge_core_version(ptr(buf), buf.length));
const version = buf.toString("utf8", 0, len);
console.log("core version:", version);

const cases: Array<[number, number, number, boolean, number]> = [
  // applied, removed, privacy%, telemetry, expected
  [10, 5, 50, false, 83], // 50 + 20 + 5 + round(7.5)=8 → 83
  [10, 5, 50, true, 78], // 83 - 5 telemetry penalty
  [0, 0, 0, true, 45], // 50 - 5 → 45
  [100, 100, 100, false, 100], // clamped at 100
];

let failures = 0;
for (const [applied, removed, privacy, telemetry, expected] of cases) {
  const actual = Number(lib.symbols.winforge_health_score(applied, removed, privacy, telemetry));
  const ok = actual === expected;
  if (!ok) failures++;
  console.log(
    `health_score(applied=${applied}, removed=${removed}, privacy=${privacy}, telemetry=${telemetry}) = ${actual} ${ok ? "✓" : `✗ expected ${expected}`}`,
  );
}

if (failures > 0) {
  console.error(`${failures} assertion(s) failed`);
  process.exit(1);
}
console.log("core bridge: all assertions passed");
